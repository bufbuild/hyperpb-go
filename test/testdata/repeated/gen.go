// Copyright 2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// gen is a script for generating blocks of repeated Protoscope fields.
package main

import (
	"flag"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

var (
	lo  = flag.Uint64("lo", 0, "lower bound (inclusive)")
	hi  = flag.Uint64("hi", 0, "upper bound (exclusive)")
	n   = flag.Int("n", 0, "the number of elements to generate")
	row = flag.Int("row", 16, "the number of elements to a row")

	format = flag.String("f", "", "the format to print each value in")

	ascii = flag.Bool("ascii", false, "generate ASCII strings instead")
	uni   = flag.Bool("unicode", false, "generate Unicode strings instead")
	bytes = flag.Bool("bytes", false, "generate byte strings instead")
	zipf  = flag.Bool("zipf", false, "use a Zipf distribution rather than a uniform one")
)

func makeString[T byte | rune](char func() T) string {
	var n int
	if *zipf {
		n = int(rand.Uint64N(*hi-*lo) + *lo)
	} else {
		hi := uint64(1) << *hi
		lo := uint64(1) << *lo
		n = bits.Len64(rand.Uint64N(hi-lo) + lo)
	}

	buf := new(strings.Builder)
	for range n {
		buf.WriteString(string(char()))
	}
	return buf.String()
}

func main() {
	flag.Parse()

	var cells [][]string
	var widths [][]int
	for i := range *n {
		if i%*row == 0 {
			cells = append(cells, nil)
			widths = append(widths, nil)
		}

		var value any
		switch {
		case *ascii:
			value = makeString(func() rune {
				for {
					r := rand.Int32N(0x7f)
					if unicode.IsGraphic(r) {
						return r
					}
				}
			})
		case *uni:
			value = makeString(func() rune {
				for {
					r := rand.Int32N(unicode.MaxRune + 1)
					// Uniformly distribute encoded lengths.
					switch rand.IntN(4) {
					case 0:
						r &= 0x7f
					case 1:
						r &= 0x7ff
					case 2:
						r &= 0xffff
					}
					if unicode.IsGraphic(r) && !unicode.IsMark(r) && !unicode.IsSpace(r) {
						return r
					}
				}
			})
		case *bytes:
			value = makeString(func() byte { return byte(rand.IntN(0xff)) })
		default:
			v := rand.Uint64N(*hi-*lo) + *lo
			if *zipf {
				// We don't bother with rand.Zipf. Instead, we pick a random bit
				// length between 0 and the bit length of hi and truncate v to that.
				k := bits.Len64(*hi)
				k = rand.IntN(k) + 1
				v &= (uint64(1) << k) - 1
			}
			value = v
		}

		cell := fmt.Sprintf(*format, value)
		cells[len(cells)-1] = append(cells[len(cells)-1], cell)
		widths[len(widths)-1] = append(widths[len(widths)-1], uniseg.StringWidth(cell))
	}

	// Discover the widest cell in each column.
	var maxima []int
	for _, row := range widths {
		for col, width := range row {
			if len(maxima) <= col {
				maxima = append(maxima, 0)
			}

			maxima[col] = max(maxima[col], width)
		}
	}

	// Snap each maximum to an even number.
	for i, n := range maxima {
		maxima[i] = (n + 2) &^ 1
	}

	maxima[len(maxima)-1] = 0 // No need to pad the final cell.

	// Render each row with the appropriate padding between them.
	for i, row := range cells {
		for j, cell := range row {
			fmt.Print(cell)

			if pad := maxima[j] - widths[i][j]; pad > 0 {
				fmt.Print(strings.Repeat(" ", pad))
			}
		}
		fmt.Println()
	}
}
