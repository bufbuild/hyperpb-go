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

package vm

import (
	"math/bits"
	"unicode/utf8"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
)

// runeBytesForBits maps bits.Len32 of a rune to the number of bytes needed to
// encode it as UTF-8. Index 0 (the rune 0) maps to 1; out-of-range runes map
// to 0, so an overlong encoding never matches its byte count.
var runeBytesForBits = func() (table [34]byte) {
	table[0] = 1
	for i := range 33 {
		n := utf8.RuneLen(rune(1) << i)
		if n < 0 {
			break
		}
		table[i+1] = byte(n)
	}
	return table
}()

// verifyUTF8 validates that the next n bytes after p1.Ptr() are valid UTF-8.
//
// Fails the parse if validation fails.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func verifyUTF8(p1 P1, p2 P2, n int) (P1, P2, zc.Range) {
	if n == 0 {
		return p1, p2, 0
	}

	p := p1.PtrAddr
	e := p1.PtrAddr.Add(n)
	e8 := p.Add(layout.RoundDown(int(e-p), 8))

	// This first part is an extremely optimized, vectorized ASCII checker.
	// It is split into a chunked part that does eight byte chunks at once,
	// and a remainder part that only does 0 to 7 bytes.
	if e8 > p {
	again:
		bytes := *xunsafe.Cast[uint64](p.AssertValid())
		p = p.Add(8)
		if bytes&tdp.SignBits != 0 {
			p = p.Add(-8) // Back up, need to take the slow path.
			goto unicode
		}
		if p < e8 {
			goto again
		}
	}
	if e > p {
		// Fast path for if the last few bytes are also ASCII.
		left := int(e - p)
		bytes := *xunsafe.Cast[uint64](p.AssertValid())
		p = p.Add(left)
		if bytes&(tdp.SignBits>>uint((8-left)*8)) != 0 {
			p = p.Add(-left)
			goto unicode
		}
	}
	{
		r := zc.NewRaw(p.Sub(xunsafe.AddrOf(p1.Src()))-n, n)
		p1.PtrAddr = p

		if debug.Enabled {
			text := r.Bytes(p1.Src())
			p1.Log(p2, "utf8", "%#v, %q", r, text)
		}
		return p1, p2, r
	}

	// All non-spatial errors are accumulated so we only have to do one branch
	// at the end.
unicode:
	ok := true
	for p < e {
		n := min(8, int(e-p))
		// Fast path for ASCII: simply check that all of the bytes don't have
		// their sign bits set.
		bytes := *xunsafe.Cast[uint64](p.AssertValid())
		mask := uint64(tdp.SignBits) >> uint((8-n)*8)
		ascii := bits.TrailingZeros64(bytes&mask) / 8
		p1.Log(p2, "ascii bytes", "%016x, %d bytes", bytes, ascii)
		p = p.Add(ascii)

		if ascii == 8 {
			continue
		}

		// Need to parse a multi-byte rune.
		// The possible encodings are like this:
		//
		// 110xxxxx 10xxxxxx
		// 1110xxxx 10xxxxxx 10xxxxxx
		// 11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
		//
		// We can use LeadingZeros8 to find which of these cases we're in.

		first := *p.AssertValid()
		count := uint(bits.LeadingZeros8(^first)) // Total # of bytes.
		p1.Log(p2, "wide rune", "%#08b, %d bytes", first, count)
		if count-2 > 2 || uint(e-p) < count {
			// In the above, counts of 0, 1, 2, 3, 4, 5, 6, 7, and 8 map
			// to -2, -1, 0, 1, 2, 3, 4, 5, 6. All of these except 0, 1, and
			// 2 compare as >2, since count is unsigned.
			goto fail
		}

		// Bounds check is complete here. We are free to load four bytes
		// and mask off what we don't need. We can't re-use bytes here
		// because the rune might straddle a boundary.
		raw := *xunsafe.Cast[uint32](p.AssertValid())
		p1.Log(p2, "wide rune bits", "%08b, %d bytes", xunsafe.Bytes(&raw), count)

		// This puts the contents of the first byte into r.
		r := rune(raw & ((1 << (8 - count)) - 1))

		i := count - 1
	decode:
		r <<= 6
		raw >>= 8
		r |= rune(raw & 0b111111)

		if raw&0b11_000000 != 0b10_000000 {
			ok = false
		}

		i--
		if i > 0 {
			goto decode
		}

		p1.Log(p2, "decoded rune", "U+%04x (%q)", r, r)

		// Next we check that the length is correct. The number of bytes
		// needed to encode a rune is a step function of its magnitude, with
		// boundaries at the powers of two U+0080, U+0800, and U+10000. Because
		// each boundary is a power of two, the length is constant across a
		// single bits.Len32 bucket, so we can look it up directly. An overlong
		// encoding decodes to a rune whose natural length is smaller than
		// count, so the mismatch below rejects it.
		//
		// Previously, we used a bit of clever arithmetic involving division by
		// 4, which failed for astral plane runes.
		wantCount := int(runeBytesForBits[bits.Len32(uint32(r))])

		p1.Log(p2, "rune size", "want: %d, got: %d", wantCount, count)
		if wantCount != int(count) {
			ok = false
		}

		// Finally, we can check that this rune is in the valid range.
		if r&^0x7ff == 0xd800 { // This checks for the surrogate range.
			ok = false
		}
		if r > 0x10ffff {
			ok = false
		}

		p = p.Add(int(count))
	}

	if ok {
		r := zc.NewRaw(e.Sub(xunsafe.AddrOf(p1.Src()))-n, n)
		p1.PtrAddr = e

		if debug.Enabled {
			text := r.Bytes(p1.Src())
			p1.Log(p2, "utf8", "%#v, %q", r, text)
		}
		return p1, p2, r
	}

fail:
	p1.Fail(p2, ErrorUTF8)
	return p1, p2, 0
}
