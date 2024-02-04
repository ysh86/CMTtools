package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	START_ADDR = uint16(0x8000)
)

func main() {
	enc, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	defer r.Close()

	// dec & checksum
	go func() {
		defer w.Close()

		rpos := 0
		sum := uint(0)
		for _, d := range enc {
			dec := [1]byte{^d}

			if rpos == 0x100 {
				checksum := byte(sum & 0xff)
				if dec[0] != checksum {
					panic(fmt.Errorf("checksum: %02x != %02x", dec[0], checksum))
				}
				rpos = 0
				sum = 0
				continue
			}

			rpos += 1
			sum += uint(d)
			w.Write(dec[:])
		}
		fmt.Fprintf(os.Stderr, "done dec: %02x\n", (^sum)&0xff)
	}()

	// dump intermediate only
	if true {
		isFirst := true
		buf := make([]byte, 1)
		for {
			_, err = io.ReadFull(r, buf[:])
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			if isFirst {
				// BASIC txt start code?
				buf[0] = 0xff
				isFirst = false
			}
			_, err = os.Stdout.Write(buf[:])
			if err != nil {
				panic(err)
			}
		}
		os.Exit(0)
	}

	// parse
	pos := uint16(0)
	buf1 := make([]byte, 1)
	_, err = io.ReadFull(r, buf1[:])
	if err != nil {
		panic(err)
	}
	pos += 1

	// BASIC txt start code?
	if buf1[0] != 0x00 {
		panic(fmt.Errorf("not start code: %02x", buf1[0]))
	}

	fmt.Fprintf(os.Stderr, "start addr: %04x\n", START_ADDR)

	// lines
	pointer := uint16(1)
	for {
		if pos < pointer {
			// skip
			_, err = io.ReadFull(r, buf1[:])
			if err != nil {
				panic(err)
			}
			pos += 1
			fmt.Fprintf(os.Stderr, "skip: %02x\n", buf1[0])
			continue
		}
		if pos != pointer {
			panic(errors.New("invalid pointer"))
		}

		err = binary.Read(r, binary.LittleEndian, &pointer)
		if err != nil {
			break
		}
		pos += 2
		if pointer == 0x0000 {
			// EOT
			break
		}
		pointer -= START_ADDR

		var line uint16
		err = binary.Read(r, binary.LittleEndian, &line)
		if err != nil {
			panic(err)
		}
		pos += 2

		txt := make([]byte, 0, 4096)
		for {
			_, err = io.ReadFull(r, buf1[:])
			if err != nil {
				panic(err)
			}
			pos += 1
			if buf1[0] == 0x00 {
				// EOL
				break
			}
			txt = append(txt, buf1[0])
		}

		fmt.Printf("%5d %+v\n", line, txt)
		fmt.Fprintf(os.Stderr, "%5d pos: %04x, next: %04x\n", line, pos, pointer)
	}

	fmt.Fprintf(os.Stderr, "done\n")
}
