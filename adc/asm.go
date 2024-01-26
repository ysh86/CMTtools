package adc

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// parse a trace(disasm) log of FB V2.1A CMT save.
// You can use the trace log files instead of the real WAV files.
//
// sample: watching for the port $4016 by Mesen emu
//
//	B597 $A9 $04     LDA #$04
//	B599 $8D $16 $40 STA $4016 = $00
//	B59C $C6 $1C     DEC $001C = $34
//	B59E $D0 $FC     BNE $B59C = $C6
//	...
func FBPort2bits(wbits io.WriteCloser, f *os.File) {
	go func() {
		defer wbits.Close()

		scanner := bufio.NewScanner(f)
		zeroOrOne := -1
		count := -1
		var bit [1]byte
		for scanner.Scan() {
			l := scanner.Text()
			if strings.Contains(l, "LDA") {
				if zeroOrOne == 1 {
					if count == 52 {
						bit[0] = 0
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
					} else if count == 106 {
						bit[0] = 1
						_, err := wbits.Write(bit[:])
						if err != nil {
							panic(err)
						}
					} else {
						panic(errors.New("invalid trace log"))
					}
				}
				if strings.Contains(l, "#$04") {
					zeroOrOne = 0
					count = 0
				}
				if strings.Contains(l, "#$FF") {
					zeroOrOne = 1
					count = 0
				}
			}
			if strings.Contains(l, "DEC") {
				count += 1
			}
		}
		if zeroOrOne == 1 {
			if count == 52 {
				bit[0] = 0
				_, err := wbits.Write(bit[:])
				if err != nil {
					panic(err)
				}
			} else if count == 106 {
				bit[0] = 1
				_, err := wbits.Write(bit[:])
				if err != nil {
					panic(err)
				}
			} else {
				panic(errors.New("invalid trace log"))
			}
		}
	}()
}

// parse a trace(disasm) log of FB V2.1A CMT save.
// You can use the trace log files instead of the real WAV files.
//
// sample: watching for the executions $B591 and $B587 by Mesen emu
//
//	B587 LDA #$34
//	B587 LDA #$34
//	B587 LDA #$34
//	B591 LDA #$6A
//	B591 LDA #$6A
//	B591 LDA #$6A
//	B591 LDA #$6A
//	...
func FBAsm2bits(wbits io.WriteCloser, f *os.File) {
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
