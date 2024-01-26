package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	adConverter "github.com/ysh86/CMTtools/adc"
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

	// in
	f, err := os.Open(*inFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// out
	outFile := *inFile + ".bin"
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
