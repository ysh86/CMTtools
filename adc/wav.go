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
	fmt.Fprintf(os.Stderr, "duration:    %v\n", duration)
	fmt.Fprintf(os.Stderr, "format:      %v\n", format.AudioFormat)
	fmt.Fprintf(os.Stderr, "bits/sample: %v\n", format.BitsPerSample)
	fmt.Fprintf(os.Stderr, "block align: %v\n", format.BlockAlign)
	fmt.Fprintf(os.Stderr, "byte rate:   %v\n", format.ByteRate)
	fmt.Fprintf(os.Stderr, "ch:          %v\n", format.NumChannels)
	fmt.Fprintf(os.Stderr, "sample rate: %v\n", format.SampleRate)
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
	fmt.Fprintf(os.Stderr, "threshold L: %d\n", thLo)
	fmt.Fprintf(os.Stderr, "threshold H: %d\n", thHi)
	countForZero := format.SampleRate/1917/2 - 2
	countForOne := format.SampleRate/958/2 - 2
	fmt.Fprintf(os.Stderr, "threshold 0: %v\n", countForZero)
	fmt.Fprintf(os.Stderr, "threshold 1: %v\n", countForOne)

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
						//fmt.Fprintf(os.Stderr, "1: %d\n", counter255)
						bit[0] = 1
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
					} else if counter255 >= int(countForZero) {
						//fmt.Fprintf(os.Stderr, "0: %d\n", counter255)
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
	fmt.Fprintf(os.Stderr, "duration:    %v\n", duration)
	fmt.Fprintf(os.Stderr, "format:      %v\n", format.AudioFormat)
	fmt.Fprintf(os.Stderr, "bits/sample: %v\n", format.BitsPerSample)
	fmt.Fprintf(os.Stderr, "block align: %v\n", format.BlockAlign)
	fmt.Fprintf(os.Stderr, "byte rate:   %v\n", format.ByteRate)
	fmt.Fprintf(os.Stderr, "ch:          %v\n", format.NumChannels)
	fmt.Fprintf(os.Stderr, "sample rate: %v\n", format.SampleRate)
	if format.AudioFormat != wav.AudioFormatPCM {
		panic(errors.New("format.AudioFormat"))
	}
	if format.BitsPerSample != 8 && format.BitsPerSample != 16 {
		panic(errors.New("format.BitsPerSample"))
	}

	preAmp := 1
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
	fmt.Fprintf(os.Stderr, "pre amp: x%d\n", preAmp)
	fmt.Fprintf(os.Stderr, "threshold L: %d\n", thLo)
	fmt.Fprintf(os.Stderr, "threshold H: %d\n", thHi)

	// decode parameters
	//
	// KCS 2400 baud:
	//  Zero: 2400Hz
	//  One:  4800Hz
	//
	// MSX 1200 baud:
	//  Zero: 1200Hz
	//  One:  2400Hz
	//
	/*
		minIntervalForZero := int(float32(format.SampleRate/2400/2) * 0.8)
		maxIntervalForZero := int(float32(format.SampleRate/2400/2) * 1.2)
		minIntervalForOne := int(float32(format.SampleRate/4800/2) * 0.8)
		maxIntervalForOne := int(float32(format.SampleRate/4800/2) * 1.2)
	*/
	minIntervalForZero := int(float32(format.SampleRate/1200/2) * 0.8)
	maxIntervalForZero := int(float32(format.SampleRate/1200/2) * 1.2)
	minIntervalForOne := int(float32(format.SampleRate/2400/2) * 0.8)
	maxIntervalForOne := int(float32(format.SampleRate/2400/2) * 1.2)

	fmt.Fprintf(os.Stderr, "Zero: %2d <= samples <= %2d\n", minIntervalForZero, maxIntervalForZero)
	fmt.Fprintf(os.Stderr, "One:  %2d <= samples <= %2d\n", minIntervalForOne, maxIntervalForOne)

	go func() {
		defer wbits.Close()

		interval := -1
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
				value := reader.IntValue(sample, 0) * 7 / 5 // * preAmp // L only
				if value < thLo {
					value = 0
				} else if value > thHi {
					value = 255
				} else {
					value = 128
				}

				// count samples in the half cycle
				switch value {
				case 0:
					if interval == -1 {
						// start count
						interval = 0
					} else {
						interval += 1
					}
				case 128:
					if interval >= 0 {
						interval += 1
					}
				case 255:
					if interval >= 0 {
						interval += 1

						if minIntervalForZero <= interval && interval <= maxIntervalForZero {
							if counterOnes == 1 {
								bit[0] = 2 // error?
								_, err := wbits.Write(bit[:])
								if err != nil {
									panic(err)
								}
								counterOnes = 0
							}
							bit[0] = 0
							_, err := wbits.Write(bit[:])
							if err != nil {
								panic(err)
							}
						} else if minIntervalForOne <= interval && interval <= maxIntervalForOne {
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

						// reset count
						interval = -1
					}
				}
			}
		}
	}()
}
