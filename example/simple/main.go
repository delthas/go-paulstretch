package main

import (
	"flag"
	"io"
	"os"

	"github.com/delthas/go-paulstretch"
)

func main() {
	inputFile := flag.String("input", "", "the input file to stretch")
	outputFile := flag.String("output", "", "the output file to store the stretched audio into")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	// stretch the audio ten times
	stretchFactor := 10.0

	// a good default buffer_duration for most music is 0.25 seconds
	bufferDuration := 0.25
	// multiply by the sample rate to get the buffer size in samples
	windowSize := int(bufferDuration * 44100)

	ps := paulstretch.NewPaulstretch(stretchFactor, windowSize)

	// for this example we use raw files because using audio codecs is not relevant
	// to generate the input file: `ffmpeg -i input.file -f f32le -c:a pcm_f32le input.raw`
	// ^ (actually this strangely messes up the volume, i used audacity instead (export as raw)
	// to generate the output file: `ffmpeg -i output.raw -c:a <codec> output.file`

	in, err := os.Open(*inputFile)
	if err != nil {
		panic(err)
	}
	defer in.Close()

	out, err := os.Create(*outputFile)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	go func() {
		_, err := io.Copy(ps, in)
		ps.Close()
		if err != nil {
			panic(err)
		}
	}()

	_, err = io.Copy(out, ps)
	if err != nil {
		panic(err)
	}
}
