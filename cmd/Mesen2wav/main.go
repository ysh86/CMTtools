package main

import (
	"bufio"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/youpy/go-wav"
)

func trace2bits(wbits io.WriteCloser, f *os.File) {
	go func() {
		defer wbits.Close()

		scanner := bufio.NewScanner(f)
		var bit [1]byte
		for scanner.Scan() {
			l := scanner.Text()
			// Zero
			if strings.Contains(l, "#$34") {
				bit[0] = 0
				_, err := wbits.Write(bit[:])
				if err != nil {
					panic(err)
				}
			}
			// One
			if strings.Contains(l, "#$6A") {
				bit[0] = 1
				_, err := wbits.Write(bit[:])
				if err != nil {
					panic(err)
				}
			}
		}
	}()
}

func main() {
	inFile := flag.String("infile", "", "trace log to convert")
	flag.Parse()
	if len(os.Args) == 2 {
		inFile = &os.Args[1]
	}

	// in
	var err error
	var f *os.File
	outFile := *inFile + ".wav"
	if *inFile == "-" {
		f = os.Stdin
		outFile = "stdin.wav"
	} else {
		f, err = os.Open(*inFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
	}

	// out
	fwav, err := os.Create(outFile)
	if err != nil {
		panic(err)
	}
	defer fwav.Close()

	// step1: trace log to bits
	rbits, wbits := io.Pipe()
	defer rbits.Close()
	trace2bits(wbits, f)
	bits, err := io.ReadAll(rbits) // all on mem :)
	if err != nil {
		panic(err)
	}

	// support 1ch, 48kHz, 8bit only
	//
	// wav parameters
	//  Zero: short 12.5 samples x2 @ 48kHz = 1920 Hz
	//  One:  long  25.0 samples x2 @ 48kHz = 960 Hz
	//
	nZERO := uint32(25)
	ZERO := [25]wav.Sample{
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
	}
	nONE := uint32(50)
	ONE := [50]wav.Sample{
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}}, {[2]int{255, 255}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
		{[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}}, {[2]int{0, 0}},
	}

	// step2: count LPCM samples
	numSamples := uint32(0)
	for _, b := range bits {
		if b == 0 {
			numSamples += nZERO
		} else {
			numSamples += nONE
		}
	}
	writer := wav.NewWriter(fwav, numSamples, 1, 48000, 8)

	// step3: bits to wav
	for _, b := range bits {
		if b == 0 {
			writer.WriteSamples(ZERO[:])
		} else {
			writer.WriteSamples(ONE[:])
		}
	}
}
