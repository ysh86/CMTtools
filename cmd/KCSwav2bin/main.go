package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/youpy/go-wav"
)

func bitToByte(bits []byte) ([]byte, error) {
	if len(bits) != 11 {
		return nil, errors.New("invalid length")
	}

	// check next file
	isEOF := true
	for _, b := range bits {
		if b != 1 {
			isEOF = false
			break
		}
	}
	if isEOF {
		return nil, io.EOF
	}

	// start bit
	if bits[0] != 0 {
		return nil, errors.New("invalid start bit")
	}

	// from LSB
	var ret byte
	for i := 0; i < 8; i++ {
		ret |= (bits[1+i] << i)
	}

	// stop bits
	if bits[9] != 1 || bits[10] != 1 {
		return nil, errors.New("invalid stop bits")
	}

	data := [1]byte{ret}
	return data[:], nil
}

func main() {
	inFile := flag.String("infile", "", "wav file to read")
	flag.Parse()

	if len(os.Args) == 2 {
		inFile = &os.Args[1]
	}

	// out
	outFile := *inFile + ".bin"
	fw, err := os.Create(outFile)
	if err != nil {
		panic(err)
	}
	defer fw.Close()

	// in
	f, err := os.Open(*inFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	reader := wav.NewReader(f)

	// input parameters
	duration, err := reader.Duration()
	if err != nil {
		panic(err)
	}
	format, err := reader.Format()
	if err != nil {
		panic(err)
	}
	fmt.Printf("duration:    %v\n", duration)
	fmt.Printf("format:      %v\n", format.AudioFormat)
	fmt.Printf("bits/sample: %v\n", format.BitsPerSample)
	fmt.Printf("block align: %v\n", format.BlockAlign)
	fmt.Printf("byte rate:   %v\n", format.ByteRate)
	fmt.Printf("ch:          %v\n", format.NumChannels)
	fmt.Printf("sample rate: %v\n", format.SampleRate)
	if format.AudioFormat != wav.AudioFormatPCM {
		panic(errors.New("format.AudioFormat"))
	}
	if format.BitsPerSample != 8 && format.BitsPerSample != 16 {
		panic(errors.New("format.BitsPerSample"))
	}

	// decode parameters
	//
	// KCS 2400 baud:
	//  Zero: 2400Hz => 10x2 samples @ 48kHz
	//  One:  4800Hz =>  5x2 samples @ 48kHz
	//  margin: 2
	//
	preAmp := 8
	margin := uint32(2)
	thLo := 0
	thHi := 0
	if format.BitsPerSample == 8 {
		th := 1 << (format.BitsPerSample - 1)
		thLo = th * 3 / 5
		thHi = th * 7 / 5
	} else {
		th := 1 << (format.BitsPerSample - 1)
		thLo = th*3/5 - th
		thHi = th*7/5 - th
	}
	fmt.Printf("threshold L: %d\n", thLo)
	fmt.Printf("threshold H: %d\n", thHi)
	countForZero := format.SampleRate/2400/2 - margin
	countForOne := format.SampleRate/4800/2 - margin
	fmt.Printf("threshold 0: %v\n", countForZero)
	fmt.Printf("threshold 1: %v\n", countForOne)

	// step1: wave to bits
	rbits, wbits := io.Pipe()
	defer rbits.Close()
	go func() {
		defer wbits.Close()

		counter255 := 0
		counterOnes := 0
		var bit [1]byte
		for {
			samples, err := reader.ReadSamples(2048)
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}

			for _, sample := range samples {
				// fix level
				value := reader.IntValue(sample, 0) * preAmp // L only
				if value < thLo {
					value = 0
				} else if value > thHi {
					value = 255
				} else {
					value = 128
				}
				// count
				if value == 255 {
					counter255++
				} else if counter255 != 0 {
					//fmt.Printf("count: %d\n", counter255)
					if counter255 >= int(countForZero) {
						//fmt.Printf("0: %d\n", counter255)
						bit[0] = 0
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
						counterOnes = 0
					} else if counter255 >= int(countForOne) {
						//fmt.Printf("1: %d\n", counter255)
						if counterOnes == 0 {
							// 1st
							counterOnes = 1
						} else {
							// 2nd
							bit[0] = 1
							_, err := wbits.Write(bit[:])
							if err != nil {
								panic(err)
							}
							counterOnes = 0
						}
					}
					counter255 = 0
				}
			}
		}
	}()

	// step2: bits to bytes
	errc := make(chan interface{})
	go func() {
		defer close(errc)

		var bits [11]byte
		countOnes := 0

	LOOP:
		// skip start code
		for {
			_, err := io.ReadFull(rbits, bits[0:1])
			if err != nil {
				panic(err)
			}
			if bits[0] != 1 {
				break
			}
			countOnes++
		}
		fmt.Printf("---- start ----\n")
		fmt.Printf("start ones: %d\n", countOnes)

		_, err := io.ReadFull(rbits, bits[1:])
		if err != nil {
			panic(err)
		}

		pos := 0
		for {
			data, err := bitToByte(bits[:])
			if err == io.EOF {
				countOnes = 11
				fmt.Printf("EOF pos: %04x\n", pos)
				goto LOOP
			}
			if err != nil {
				panic(err)
			}
			// output
			fw.Write(data)
			//fmt.Printf("%04x: %02x\n", pos, data[0])
			pos++

			// next
			_, err = io.ReadFull(rbits, bits[:])
			if err == io.EOF {
				fmt.Printf("EOF pos: %04x\n", pos)
				fmt.Printf("---- EOF ----\n")
				break
			}
			if err != nil {
				panic(err)
			}
		}
	}()

	<-errc
}
