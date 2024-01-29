package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	adConverter "github.com/ysh86/CMTtools/adc"
)

func main() {
	inFile := flag.String("infile", "-", "wav file to read")
	flag.Parse()
	if len(os.Args) == 2 {
		inFile = &os.Args[1]
	}

	// in
	var err error
	var f *os.File
	outFile := *inFile + ".bin"
	if *inFile == "-" {
		f = os.Stdin
		outFile = "stdin.bin"
	} else {
		f, err = os.Open(*inFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
	}

	// out
	fw, err := os.Create(outFile)
	if err != nil {
		panic(err)
	}
	defer fw.Close()

	// step1: wav to bits
	rbits, wbits := io.Pipe()
	defer rbits.Close()
	adConverter.KCSWav2bits(wbits, f)

	// step2: bits to bytes
	done := make(chan interface{})
	go func() {
		defer close(done)

		countOnes := 0
		var bits [11]byte
		globalPos := 0
		pos := 0

	LOOP:
		// skip start code
		for {
			_, err := io.ReadFull(rbits, bits[0:1])
			if err != nil {
				break
			}
			// skip the noisy part & search the first Zero
			if countOnes > 100 && bits[0] == 0 {
				break
			}
			countOnes++
		}
		fmt.Fprintf(os.Stderr, "---- start ----\n")
		fmt.Fprintf(os.Stderr, "start ones: %d\n", countOnes)

		_, err := io.ReadFull(rbits, bits[1:])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			panic(err)
		}

		globalPos += pos
		pos = 0
		for {
			data, err := bitToByte(bits[:])
			if err == io.EOF {
				countOnes = 11
				fmt.Fprintf(os.Stderr, "EOF: %04x, %04x\n", globalPos+pos, pos)
				goto LOOP
			}
			if err != nil {
				panic(err)
			}
			// output
			//fmt.Fprintf(os.Stderr, "%04x: %02x\n", pos, data[0])
			fw.Write(data)
			pos++

			// next
			_, err = io.ReadFull(rbits, bits[:])
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				fmt.Fprintf(os.Stderr, "EOF: %04x, %04x\n", globalPos+pos, pos)
				fmt.Fprintf(os.Stderr, "---- EOF ----\n")
				break
			}
			if err != nil {
				panic(err)
			}
		}
	}()

	// wait to finish
	<-done
}

func bitToByte(bits []byte) ([]byte, error) {
	if len(bits) != 11 {
		return nil, fmt.Errorf("invalid length: %d", len(bits))
	}

	// check next file
	isEOF := true
	for _, b := range bits {
		if b == 0 {
			isEOF = false
			break
		}
	}
	if isEOF {
		return nil, io.EOF
	}

	// start bit
	if bits[0] != 0 {
		e := fmt.Errorf("invalid start bits: %d, LSB %+v MSB, %d, %d", bits[0], bits[1:9], bits[9], bits[10])
		fmt.Fprintln(os.Stderr, e)
		return nil, io.EOF
		//return nil, e
	}

	// from LSB
	var ret byte
	for i := 0; i < 8; i++ {
		ret |= (bits[1+i] << i)
	}

	// stop bits
	if bits[9] != 1 || bits[10] != 1 {
		e := fmt.Errorf("invalid stop bits: %d, %08b(%02x), %d, %d", bits[0], ret, ret, bits[9], bits[10])
		fmt.Fprintln(os.Stderr, e)
		return nil, io.EOF
		//return nil, e
	}

	data := [1]byte{ret}
	return data[:], nil
}
