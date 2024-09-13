package adc

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
)

const (
	num = (1 + 8 + 2) * 2

	thLong  = 0x30
	thShort = 0x16
)

// support Version 0 only
// https://web.archive.org/web/20231126092033/http://retropc.net/ryu/xm7/t77form.html
func T772bits(wbits io.WriteCloser, f *os.File, reverse bool) {
	go func() {
		defer wbits.Close()

		// file header
		expected := []byte("XM7 TAPE IMAGE 0")
		header := make([]byte, len(expected))
		_, err := io.ReadFull(f, header)
		if err != nil {
			panic(err)
		}
		if !slices.Equal(header, expected) {
			panic("no header")
		}

		// marker
		expected = []byte{0, 0}
		marker := make([]byte, len(expected))
		_, err = io.ReadFull(f, marker)
		if err != nil {
			panic(err)
		}
		if !slices.Equal(marker, expected) {
			panic("no marker")
		}

		// data
		fillNum := num
		datas := make([]uint16, num)
	SEARCH:
		for {
			// fill
			for i := num - fillNum; i < num; i++ {
				err = binary.Read(f, binary.BigEndian, datas[i:i+1])
				if err != nil {
					break SEARCH
				}
			}

			/*
				fmt.Fprintf(os.Stderr, "%+v\n", datas)
				pos, _ := f.Seek(0, io.SeekCurrent)
				fmt.Fprintf(os.Stderr, "---- input pos: %08x,%08x", pos-num*2, pos)
			*/

			var bits []byte
			if !reverse {
				if datas[0] <= 0x8000 || datas[1] >= 0x8000 || // start bit
					datas[2] <= 0x8000 || datas[3] >= 0x8000 || // byte
					datas[4] <= 0x8000 || datas[5] >= 0x8000 ||
					datas[6] <= 0x8000 || datas[7] >= 0x8000 ||
					datas[8] <= 0x8000 || datas[9] >= 0x8000 ||
					datas[10] <= 0x8000 || datas[11] >= 0x8000 ||
					datas[12] <= 0x8000 || datas[13] >= 0x8000 ||
					datas[14] <= 0x8000 || datas[15] >= 0x8000 ||
					datas[16] <= 0x8000 || datas[17] >= 0x8000 ||
					datas[18] <= 0x8000 || datas[19] >= 0x8000 || // end bits
					datas[20] <= 0x8000 || datas[21] >= 0x8000 {
					// skip
					//fmt.Fprintf(os.Stderr, ": skip half bit\n")
					datas = append(datas[1:22], 0)
					fillNum = 1
					continue
				}

				// decode byte
				bits, err = decode(datas)
				if err != nil || bits[0] != 0 || bits[9] != 1 || bits[10] != 1 {
					// skip
					//fmt.Fprintf(os.Stderr, ": dec: skip bit\n")
					datas = append(datas[2:22], 0, 0)
					fillNum = 2
					continue
				}
			} else {
				if datas[0] >= 0x8000 || datas[1] <= 0x8000 || // start bit
					datas[2] >= 0x8000 || datas[3] <= 0x8000 || // byte
					datas[4] >= 0x8000 || datas[5] <= 0x8000 ||
					datas[6] >= 0x8000 || datas[7] <= 0x8000 ||
					datas[8] >= 0x8000 || datas[9] <= 0x8000 ||
					datas[10] >= 0x8000 || datas[11] <= 0x8000 ||
					datas[12] >= 0x8000 || datas[13] <= 0x8000 ||
					datas[14] >= 0x8000 || datas[15] <= 0x8000 ||
					datas[16] >= 0x8000 || datas[17] <= 0x8000 ||
					datas[18] >= 0x8000 || datas[19] <= 0x8000 || // end bits
					datas[20] >= 0x8000 || datas[21] <= 0x8000 {
					// skip
					//fmt.Fprintf(os.Stderr, ": skip half bit\n")
					datas = append(datas[1:22], 0)
					fillNum = 1
					continue
				}

				// decode byte
				bits, err = decodeR(datas)
				if err != nil || bits[0] != 0 || bits[9] != 1 || bits[10] != 1 {
					// skip
					//fmt.Fprintf(os.Stderr, ": dec: skip bit\n")
					datas = append(datas[2:22], 0, 0)
					fillNum = 2
					continue
				}
			}

			// output
			//fmt.Fprintf(os.Stderr, ": OK: %+v\n", bits)
			_, err = wbits.Write(bits)
			if err != nil {
				panic(err)
			}
			fillNum = num
		}
	}()
}

func decode(datas []uint16) ([]byte, error) {
	ret := make([]byte, 0, num/2)

	for i := 0; i < num; i += 2 {
		if 0x8000+thLong-12 < datas[i] && datas[i] < 0x8000+thLong+18 { /*&&
			(thLong-12 < datas[i+1] && datas[i+1] < thLong+18) {*/
			// 0x24 < x < 0x42
			ret = append(ret, 1)
		} else if /*(0x8000+thShort-4 < datas[i] && datas[i] < 0x8000+thShort+16) &&*/
		thShort-4 < datas[i+1] && datas[i+1] < thShort+16 {
			// 0x12 < x < 0x26
			ret = append(ret, 0)
		} else {
			return nil, errors.New("unknown signal")
		}
	}

	return ret, nil
}

func decodeR(datas []uint16) ([]byte, error) {
	ret := make([]byte, 0, num/2)

	for i := 0; i < num; i += 2 {
		if /*0x8000+thLong-12 < datas[i+1] && datas[i+1] < 0x8000+thLong+18 &&*/
		thLong-12 < datas[i] && datas[i] < thLong+18 {
			// 0x24 < x < 0x42
			ret = append(ret, 1)
		} else if 0x8000+thShort-4 < datas[i+1] && datas[i+1] < 0x8000+thShort+16 { /*&&
			thShort-4 < datas[i] && datas[i] < thShort+16 {*/
			// 0x12 < x < 0x26
			ret = append(ret, 0)
		} else {
			return nil, errors.New("unknown signal")
		}
	}

	return ret, nil
}
