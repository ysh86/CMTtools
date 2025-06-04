package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	adConverter "github.com/ysh86/CMTtools/adc"
)

func main() {
	var inFile string
	var reverse bool

	flag.StringVar(&inFile, "infile", "-", "T77 file to read")
	flag.BoolVar(&reverse, "r", false, "do reverse")
	flag.Parse()
	if len(flag.Args()) == 1 {
		inFile = flag.Arg(0)
	}

	// in
	var err error
	var f *os.File
	outFile := inFile + ".bin"
	if inFile == "-" {
		f = os.Stdin
		outFile = "stdin.bin"
	} else {
		f, err = os.Open(inFile)
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

	// step1: T77 to bits
	rbits, wbits := io.Pipe()
	defer rbits.Close()
	adConverter.T772bits(wbits, f, reverse)

	// step2: bits to bytes
	rbytes, wbytes := io.Pipe()
	defer rbytes.Close()
	go func() {
		defer wbytes.Close()

		var bits [11]byte
		pos := 0
		for {
			// next
			_, err = io.ReadFull(rbits, bits[:])
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				panic(err)
			}

			data, err := bitsToByte(bits[:])
			if err != nil {
				panic(err)
			}

			// output
			//fmt.Fprintf(os.Stderr, "%04x: %02x\n", pos, data[0])
			fw.Write(data)
			wbytes.Write(data)
			pos += 1
		}
		fmt.Fprintf(os.Stderr, "%s: %04x\n", err, pos)
	}()

	// step3: parse bytes
	r := bufio.NewReader(rbytes)
	block := make([]byte, 4096)
	fileNo := 0
	fileSize := 0
	for {
		p, err2 := r.Peek(2)
		if err2 != nil {
			err = err2
			break
		}
		if p[0] != 0x01 || p[1] != 0x3c {
			r.Discard(1)
			continue
		}

		// found a block (0x01,0x3c)
		r.Discard(2)
		blockType, err := r.ReadByte()
		if err != nil {
			panic(err)
		}
		blockSize, err := r.ReadByte()
		if err != nil {
			panic(err)
		}
		_, err = io.ReadFull(r, block[0:blockSize])
		if err != nil {
			panic(err)
		}
		blockCheckSum, err := r.ReadByte()
		if err != nil {
			panic(err)
		}

		// output
		fmt.Fprintf(os.Stderr, "block type:%02x size:%02x checksum:%02x\n", blockType, blockSize, blockCheckSum)
		// check sum
		sum := int(blockType) + int(blockSize)
		for _, b := range block[0:blockSize] {
			sum += int(b)
		}
		if byte(sum&0xff) != blockCheckSum {
			panic(fmt.Errorf("checksum: %04x", sum))
		}
		switch blockType {
		case 0x00:
			// header
			if blockSize != 8+3+9 {
				fmt.Fprintf(os.Stderr, "invalid header size\n")
				break
			}
			fileName := string(block[0:8])
			fileType := (int(block[8]) << 16) | (int(block[9]) << 8) | int(block[10])
			if fileType == 0x020000 {
				// header&footer for machine language
				// 0x00
				// size  2bytes
				// start 2bytes
				// data  Nbytes
				// 0xff,0x00,0x00
				// exec  2bytes
				fileSize = -10
			} else {
				fileSize = 0
			}
			fmt.Fprintf(os.Stderr, "    file:%d name:%s type:%06x\n", fileNo, fileName, fileType)
		case 0x01:
			// data
			fileSize += int(blockSize)
		case 0xff:
			// end
			fmt.Fprintf(os.Stderr, "    file:%d end size:%04x(%d)\n", fileNo, fileSize, fileSize)
			fileNo += 1
		default:
			panic(blockType)
		}
	}

	// finalize
	fmt.Fprintf(os.Stderr, "    %s: files:%d\n", err, fileNo)
}

func bitsToByte(bits []byte) ([]byte, error) {
	if len(bits) != 11 {
		return nil, fmt.Errorf("invalid length: %d", len(bits))
	}

	// start bit
	if bits[0] != 0 {
		e := fmt.Errorf("invalid start bit: %d, LSB %+v MSB, %d, %d", bits[0], bits[1:9], bits[9], bits[10])
		fmt.Fprintln(os.Stderr, e)
		//return nil, io.EOF
		return nil, e
	}

	// from LSB
	var ret byte
	for i := 0; i < 8; i++ {
		ret |= (bits[1+i] << i)
	}

	// stop bits
	if bits[9] != 1 || bits[10] != 1 {
		e := fmt.Errorf("ignore corrupted stop bits: %d, %08b(%02X), %d, %d", bits[0], ret, ret, bits[9], bits[10])
		fmt.Fprintln(os.Stderr, e)
		//return nil, io.EOF
		//return nil, e
	}

	data := [1]byte{ret}
	return data[:], nil
}
