/*
go-paulstretch are Go bindings to libpaulstretch, tiny & portable implementation of the Paulstretch extreme audio stretching algorithm.

Audio format

go-paulstretch uses streams of mono uncompressed 32-bit float samples, in native endianness.

Usage

To stretch a sound, create a Paulstretch instance with NewPaulstretch. Paulstretch supports the Reader, Writer and Closer interfaces to provide a pipe-like interface with a stream of audio samples as input and a stream of stretched audio samples as output.

Concurrency

All functions of this package are completely safe for concurrent use.
*/
package paulstretch

// #cgo pkg-config: paulstretch
// #include <paulstretch.h>
import "C"
import (
	"io"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

// Paulstretch is an initialized Paulstretch instance, used to stretch audio.
//
// To create a Paulstretch, use NewPaulstetch.
//
// Paulstretch supports the Reader, Writer and Closer interfaces, used to write a stream of
// audio samples and get back a stream of stretched audio samples.
type Paulstretch struct {
	ps          C.paulstretch
	writeBuf    []byte
	writeOff    int
	readBuf     []byte
	readOff     int
	closed      bool
	rwCond      sync.Cond
	writePermit chan struct{}
}

// NewPaulstretch returns a Paulstretch initialized with a stretch factor and stretching window size.
//
// stretchFactor is the stretching factor for the audio.
// A stretch factor of 10 on 1 second of audio would produce approximately 10 seconds of audio.
// stretchFactor must be greater than or equal to 1.0.
//
// windowSize is the size (in samples) of the window used for stretching the audio.
// In internally corresponds to the size of the FFT run on parts of the song.
// A window size corresponding to 0.25 seconds works best for most music.
// Larger values can also be used to "smear" a sound into a texture.
// windowSize should be greater than or equal to 128.
func NewPaulstretch(stretchFactor float64, windowSize int) *Paulstretch {
	ps := C.paulstretch_create(C.double(stretchFactor), C.size_t(windowSize))
	p := Paulstretch{
		ps:          ps,
		writeBuf:    make([]byte, windowSize*4),
		writeOff:    0,
		readBuf:     make([]byte, windowSize*4),
		readOff:     windowSize * 4,
		rwCond:      sync.Cond{L: &sync.Mutex{}},
		writePermit: make(chan struct{}, 1),
	}
	p.writePermit <- struct{}{}
	runtime.SetFinalizer(&p, func(p *Paulstretch) {
		C.paulstretch_destroy(p.ps)
	})
	return &p
}

// Close signals Paulstretch that no other data will be written to it, and that Read
// should return EOF instead of waiting for more stretch audio data.
func (p *Paulstretch) Close() error {
	p.rwCond.L.Lock()
	if !p.closed {
		p.closed = true
		close(p.writePermit)
		p.rwCond.Signal()
	}
	p.rwCond.L.Unlock()
	return nil
}

// Write writes bytes of an audio sample stream (native-endian floats) to Paulstretch.
//
// Write may block until Read is called enough times, because Paulstretch does not buffer
// stretch output samples and needs them to be read before processing new samples.
func (p *Paulstretch) Write(data []byte) (int, error) {
	if p.closed {
		return 0, io.EOF
	}
	n := 0
	for p.writeOff+len(data) >= len(p.writeBuf) {
		var buf []byte
		c := len(p.writeBuf) - p.writeOff
		if p.writeOff == 0 {
			buf = data
			data = data[len(p.writeBuf):]
		} else {
			buf = p.writeBuf
			copy(buf[p.writeOff:], data)
			data = data[len(p.writeBuf)-p.writeOff:]
			p.writeOff = 0
		}
		sh := reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(&buf[0])),
			Len:  len(buf) / 4,
			Cap:  len(buf) / 4,
		}
		samples := *(*[]C.float)(unsafe.Pointer(&sh))
		<-p.writePermit
		p.rwCond.L.Lock()
		if p.closed {
			p.rwCond.L.Unlock()
			return n, io.EOF
		}
		C.paulstretch_write(p.ps, &samples[0])
		p.rwCond.Signal()
		p.rwCond.L.Unlock()
		n += c
	}
	if len(data) > 0 {
		copy(p.writeBuf[p.writeOff:], data)
		p.writeOff += len(data)
		n += len(data)
	}
	return n, nil
}

// WriteSamples is a utility function that eventually calls Write with this sample array.
//
// WriteSamples returns the number of samples written to Paulstretch and any underlying error
// encountered during Write.
func (p *Paulstretch) WriteSamples(samples []float32) (int, error) {
	sh := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&samples[0])),
		Len:  len(samples) * 4,
		Cap:  len(samples) * 4,
	}
	b := *(*[]byte)(unsafe.Pointer(&sh))
	n, err := p.Write(b)
	return n / 4, err
}

// Read reads bytes of the stretched audio sample stream (native-endian floats) from Paulstretch.
//
// Read may block until Write is called enough times, as a pipe-like behviour, since Paulstretch
// uses the written audio samples to generate the stretched ones.
func (p *Paulstretch) Read(data []byte) (int, error) {
	if p.readOff < len(p.readBuf) {
		n := copy(data, p.readBuf[p.readOff:])
		p.readOff += n
		return n, nil
	}
	if len(data) == 0 {
		return 0, nil
	}
	p.rwCond.L.Lock()
	var outSamples *C.float
	available := C.paulstretch_read(p.ps, &outSamples)
	for !available {
		if p.closed {
			p.rwCond.L.Unlock()
			return 0, io.EOF
		}
		select {
		// add a write permit if none is currently pending
		case p.writePermit <- struct{}{}:
		default:
		}
		p.rwCond.Wait()
		available = C.paulstretch_read(p.ps, &outSamples)
	}
	p.rwCond.L.Unlock()
	sh := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(outSamples)),
		Len:  len(p.readBuf),
		Cap:  len(p.readBuf),
	}
	out := *(*[]byte)(unsafe.Pointer(&sh))
	n := copy(data, out)
	if n < len(p.readBuf) {
		copy(p.readBuf[n:], out[n:])
		p.readOff = n
	}
	return n, nil
}

// ReadSamples is a utility function that eventually calls Read with this sample array.
//
// ReadSamples returns the number of samples read from Paulstretch and any underlying error
// encountered during Read.
func (p *Paulstretch) ReadSamples(samples []float32) (int, error) {
	sh := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&samples[0])),
		Len:  len(samples) * 4,
		Cap:  len(samples) * 4,
	}
	b := *(*[]byte)(unsafe.Pointer(&sh))
	n, err := p.Read(b)
	return n / 4, err
}

// OptimalBufferSize returns the optimal size, in samples, of the buffers to be passed to WriteSamples and Readsamples.
//
// Paulstretch internally uses buffers of this size to process data, and using buffers of this size helps avoid some copying.
func (p *Paulstretch) OptimalBufferSize() int {
	return len(p.readBuf) / 4
}
