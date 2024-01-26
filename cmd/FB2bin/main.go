package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	adConverter "github.com/ysh86/CMTtools/adc"
)

func bitToByte(length int, bits []byte) (uint16, error) {
	if length*9 != len(bits) {
		return 0, errors.New("invalid length")
	}

	var ret uint16
	for l := 0; l < length; l++ {
		// start bit
		if bits[0] != 1 {
			return 0, errors.New("invalid bits")
		}
		bits = bits[1:]

		// from MSB & little-endian
		for i, b := range bits[0:8] {
			ret |= (uint16(b) << (7 - i)) << (l * 8)
		}

		bits = bits[8:]
	}

	return ret, nil
}

func bitToBytes16(bits []byte) ([]byte, error) {
	if 16*9 != len(bits) {
		return nil, errors.New("invalid length")
	}

	var ret [16]byte
	for i := range ret {
		b, err := bitToByte(1, bits[i*9:i*9+9])
		if err != nil {
			return nil, err
		}
		ret[i] = byte(b & 0xff)
		if ret[i] == 0x00 {
			// null-terminated
			return ret[0:i], nil
		}
	}

	return ret[:], nil
}

func dumpData(attrib uint16, bits []byte) {
	cur := 0
	if attrib == 0x02 {
		// BASIC code
		for cur < len(bits) {
			lineLen, _ := bitToByte(1, bits[cur:cur+9])
			if lineLen == 0 {
				// end mark: 0x00
				cur = cur + 9
				fmt.Printf("end of data: %d\n", cur)
				break
			}
			//fmt.Printf("%3d: ", lineLen)
			lineLen -= 1
			cur = cur + 9

			lineNum, _ := bitToByte(2, bits[cur:cur+9*2])
			lineLen -= 2
			cur = cur + 9*2
			fmt.Printf("%4d %3d,", lineNum, lineLen)

			for l := 0; l < int(lineLen); l++ {
				b, _ := bitToByte(1, bits[cur:cur+9])
				fmt.Printf(" %02x", b)
				cur += 9
			}
			fmt.Println("")
		}
	} else {
		// BG2 dump
		pos := 0
		for cur < len(bits) {
			b, err := bitToByte(1, bits[cur:cur+9])
			if err != nil {
				panic(err)
			}
			cur = cur + 9
			pos++
			fmt.Printf(" %02x", b)
			if pos&0xf == 0 {
				fmt.Printf("\n")
			}
		}
		if pos&0xf != 0 {
			fmt.Printf("\n")
		}
	}

	if cur != len(bits) {
		panic(fmt.Errorf("invalid data: cur=%d, len=%d", cur, len(bits)))
	}
}

func main() {
	inFile := flag.String("infile", "", "wav/trace file to decode")
	flag.Parse()
	if len(os.Args) == 2 {
		inFile = &os.Args[1]
	}

	// in
	var err error
	var f *os.File
	if *inFile == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(*inFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
	}

	// step1: wav/trace log to bits
	rbits, wbits := io.Pipe()
	defer rbits.Close()
	if strings.HasSuffix(*inFile, ".wav") {
		adConverter.FBWav2bits(wbits, f)
	} else {
		adConverter.FBPort2bits(wbits, f)
	}

	// step2: bits to Tape blocks
	errc := make(chan interface{})
	go func() {
		defer close(errc)

		var bits [1024 * 9]byte
		var dataLen uint16
		var attrib uint16
		for {
			// skip start code
			countZeros := 0
			for {
				_, err := io.ReadFull(rbits, bits[0:1])
				if err == io.EOF {
					fmt.Printf("---- EOF ----\n")
					return
				}
				if err != nil {
					panic(err)
				}
				if bits[0] != 0 {
					break
				}
				countZeros++
			}
			fmt.Printf("---- block start ----\n")
			fmt.Printf("start zeros: %d\n", countZeros)

			// tape mark
			_, err := io.ReadFull(rbits, bits[1:20])
			if err != nil {
				panic(err)
			}
			for i, b := range bits[0:20] {
				if b != 1 {
					panic(fmt.Errorf("invalid mark bits: %d, %d", i, b))
				}
			}
			// info or data
			isInfo := false
			_, err = io.ReadFull(rbits, bits[0:20])
			if err != nil {
				panic(err)
			}
			if bits[0] == 1 {
				// info block
				for i, b := range bits[0:20] {
					if b != 1 {
						panic(fmt.Errorf("invalid info mark bits: %d, %d", i, b))
					}
				}
				_, err = io.ReadFull(rbits, bits[0:40])
				if err != nil {
					panic(err)
				}
				for i, b := range bits[0:40] {
					if b != 0 {
						panic(fmt.Errorf("invalid info mark bits: %d, %d", i, b))
					}
				}
				isInfo = true
			} else {
				// data block
				for i, b := range bits[0:20] {
					if b != 0 {
						panic(fmt.Errorf("invalid data mark bits: %d, %d", i, b))
					}
				}
			}

			if isInfo {
				length := 1 + 128*9 + 2*9 + 1
				_, err := io.ReadFull(rbits, bits[0:length])
				if err != nil {
					panic(err)
				}
				fmt.Printf("info block: %d bits\n", length)

				// validation
				if bits[0] != 1 {
					panic(fmt.Errorf("invalid start bit: %d", bits[0]))
				}
				attrib, _ = bitToByte(1, bits[1:1+9])
				name, _ := bitToBytes16(bits[10 : 10+9*16])
				reserved, _ := bitToByte(1, bits[154:154+9])
				dataLen, _ = bitToByte(2, bits[163:163+9*2])
				loadAddr, _ := bitToByte(2, bits[181:181+9*2])
				callAddr, _ := bitToByte(2, bits[199:199+9*2])
				// emp: 104*9 [bits]
				checksum, _ := bitToByte(2, bits[length-1-9*2:length-1])
				if bits[length-1] != 1 {
					panic(fmt.Errorf("invalid end bit: %d", bits[length-1]))
				}
				fmt.Printf("attrib:   %02x\n", attrib)
				fmt.Printf("name:     %s\n", string(name))
				fmt.Printf("reserved: %02x\n", reserved)
				fmt.Printf("dataLen:  %04x\n", dataLen)
				fmt.Printf("loadAddr: %04x\n", loadAddr)
				fmt.Printf("callAddr: %04x\n", callAddr)
				fmt.Printf("checksum: %04x\n", checksum)
			} else {
				length := 1 + dataLen*9 + 9*2 + 1
				fmt.Printf("data block: %d bits\n", length)

				// validation
				_, err := io.ReadFull(rbits, bits[0:1])
				if err != nil {
					panic(err)
				}
				if bits[0] != 1 {
					panic(fmt.Errorf("invalid start bit: %d", bits[0]))
				}

				// data
				data := make([]byte, dataLen*9)
				_, err = io.ReadFull(rbits, data)
				if err != nil {
					panic(err)
				}
				dumpData(attrib, data)

				// checksum
				_, err = io.ReadFull(rbits, bits[0:18])
				if err != nil {
					panic(err)
				}
				checksum, _ := bitToByte(2, bits[0:18])
				fmt.Printf("checksum: %04x\n", checksum)

				// validation
				_, err = io.ReadFull(rbits, bits[0:1])
				if err != nil {
					panic(err)
				}
				if bits[0] != 1 {
					panic(fmt.Errorf("invalid end bit: %d", bits[0]))
				}
			}
		}
	}()

	<-errc
}
