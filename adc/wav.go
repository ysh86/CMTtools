package adc

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/youpy/go-wav"
)

func FBWav2bits(wbits io.WriteCloser, f *os.File) {
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

	// wav parameters
	//  Zero: short 10 - 13 -> 11.5 samples x2 @ 44.1kHz = 1917 Hz
	//  One:  long  22 - 24 -> 23.0 samples x2 @ 44.1kHz = 958 Hz
	//  margin: 2
	//
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
	countForZero := format.SampleRate/1917/2 - 2
	countForOne := format.SampleRate/958/2 - 2
	fmt.Printf("threshold 0: %v\n", countForZero)
	fmt.Printf("threshold 1: %v\n", countForOne)

	go func() {
		defer wbits.Close()

		counter255 := 0
		var bit [1]byte
		for {
			samples, err := reader.ReadSamples(1024)
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}

			for _, sample := range samples {
				value := reader.IntValue(sample, 0) // L only
				// fix level
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
					if counter255 >= int(countForOne) {
						//fmt.Printf("1: %d\n", counter255)
						bit[0] = 1
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
					} else if counter255 >= int(countForZero) {
						//fmt.Printf("0: %d\n", counter255)
						bit[0] = 0
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
					}
					counter255 = 0
				}
			}
		}
	}()
}

func KCSWav2bits(wbits io.WriteCloser, f *os.File) {
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
}
