# go-paulstretch [![builds.sr.ht status](https://builds.sr.ht/~delthas/go-paulstretch.svg)](https://builds.sr.ht/~delthas/go-paulstretch) [![GoDoc](https://godoc.org/github.com/delthas/go-paulstretch?status.svg)](https://godoc.org/github.com/delthas/go-paulstretch)

Go bindings for [libpaulstretch](https://github.com/delthas/libpaulstretch), a tiny & portable implementation of the Paulstretch extreme audio stretching algorithm.

## Usage

go-paulstretch depends on [libpaulstretch](https://github.com/delthas/libpaulstretch), which also depends on FFTW3.

The API is well-documented in its [![GoDoc](https://godoc.org/github.com/delthas/go-paulstretch?status.svg)](https://godoc.org/github.com/delthas/go-paulstretch).

There is also a simple example in [`example/simple/`](example/simple).

Using go-paulstretch is as simple as (error-checking code ommited for brevity):
```go
ps := paulstretch.NewPaulstretch(stretchFactor, windowSize)
go func() {
    io.Copy(ps, audio_in)
    ps.Close()
}()
io.Copy(audio_out, ps)
```

## License

MIT
