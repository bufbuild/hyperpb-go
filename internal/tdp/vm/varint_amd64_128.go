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

// Package varintsimd provides a SIMD-accelerated protobuf varint decoder
// using Go 1.26's simd/archsimd package (GOEXPERIMENT=simd).
//
// This is a port of the approach used in Google's protobuf C++ implementation
// (varint_shuffle.h), which uses SSSE3 PSHUFB to strip continuation bits from
// varint-encoded bytes and pack the 7-bit payloads into a 64-bit integer.
//
// The algorithm:
//  1. Load 16 bytes from the input buffer into an XMM register.
//  2. Detect the varint boundary by finding the first byte without the MSB set.
//  3. Use a precomputed PSHUFB shuffle mask (indexed by the continuation-bit pattern)
//     to strip each byte's MSB and pack the 7-bit groups into a contiguous 64-bit value.
//  4. Return the decoded uint64 and the number of bytes consumed.

//go:build amd64

package vm

import (
	"math/bits"
	"simd/archsimd"

	"buf.build/go/hyperpb/internal/xbits"
	"buf.build/go/hyperpb/internal/xunsafe"
)

var (
	signBits128 = archsimd.BroadcastUint8x16(0x7f)

	// And truncMasks[i] with a vector to zero all lanes i or greater, and all
	// sign bits.
	truncMasks = func() [16]archsimd.Uint8x16 {
		var mask archsimd.Uint8x16
		var masks [16]archsimd.Uint8x16
		for i := range masks {
			mask = mask.SetElem(uint8(i), 0xff)
			masks[i] = mask.And(signBits128)
		}

		return masks
	}()

	mulShift = makeU16x8(func(i uint16) uint16 { return 1 << i })

	shuffles = [...]archsimd.Int8x16{
		makeI8x16(func(i int8) int8 {
			if i%2 == 1 {
				return -1
			}
			return i
		}).AsInt16x8().TruncateToInt8(),
		makeI8x16(func(i int8) int8 {
			if i%2 == 1 {
				return -1
			}
			return i + 1
		}).AsInt16x8().TruncateToInt8(),
		makeI8x16(func(i int8) int8 {
			switch i {
			case 13:
				return 0
			case 14:
				return 2
			default:
				return -1
			}
		}).AsInt16x8().TruncateToInt8(),
		makeI8x16(func(i int8) int8 {
			switch i {
			case 13:
				return 3
			default:
				return -1
			}
		}).AsInt16x8().TruncateToInt8(),
	}

	rotate8Shuffle = makeI8x16(func(u int8) int8 { return (u - 8) % 8 })
)

func makeI8x16(f func(int8) int8) archsimd.Int8x16 {
	var v archsimd.Int8x16
	for i := range int8(16) {
		v = v.SetElem(uint8(i), f(i))
	}
	return v
}

func makeU16x8(f func(uint16) uint16) archsimd.Uint16x8 {
	var v archsimd.Uint16x8
	for i := range uint16(8) {
		v = v.SetElem(uint8(i), f(i))
	}
	return v
}

func split8to16(v archsimd.Uint8x16) (lo, hi archsimd.Uint16x8) {
	lo = v.ExtendLo8ToUint16()
	hi = v.PermuteOrZero(rotate8Shuffle).ExtendLo8ToUint16()
	return lo, hi
}

func init() {
	if archsimd.X86.AVX() {
		parseVarint = parseVarintAVX
	}
}

//go:nosplit
func parseVarintAVX(p1 P1, p2 P2) (P1, P2, uint64) {
	if p1.Len() < 16 {
		return parseVarintScalarNoinline(p1, p2)
	}

	data := archsimd.LoadUint8x16(xunsafe.Cast[[16]uint8](p1.Ptr()))

	// Find all of the continuation bytes. ToBits produces pmovmskb which is
	// what we want, although we have to do a redundant comparison here...
	var zero archsimd.Int8x16
	signs := data.AsInt8x16().Less(zero).ToBits()

	len := bits.TrailingZeros16(^signs) + 1

	// Discard extra bytes and the sign bits.
	data = data.And(truncMasks[len&0xf])

	lo, hi := split8to16(data)

	// lo and hi are now of the following form (highest bytes irrelevant, big
	// endian u16s).
	//
	// 00000000 0aaaaaaa 00000000 0bbbbbbb 00000000 0ccccccc 00000000 0ddddddd
	// 00000000 0eeeeeee 00000000 0fffffff 00000000 0ggggggg 00000000 0hhhhhhh
	//
	// 00000000 0iiiiiii 00000000 0jjjjjjj xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	// xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	//
	// We want to get them into this form, which we can then shuffle into
	// something we can or together.
	//
	// 00000000 0aaaaaaa b0000000 00bbbbbb cc000000 000ccccc ddd00000 0000dddd
	// eeee0000 00000eee fffff000 000000ff gggggg00 0000000g hhhhhhh0 00000000
	//
	// 00000000 0iiiiiii j0000000 00jjjjjj xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	// xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	//
	// To convert 00000000 0bbbbbbb -> b0000000 00bbbbbb, we can left shift by
	// 7 and then swap the bytes, then 6 bits, and so on. This can be realized
	// as a multiply plus a shuffle.
	//
	// However, we want to overlay alternating bytes, like so:
	//
	// 0aaaaaaa 00000000 00bbbbbb 00000000 000ccccc 00000000 0000dddd 00000000
	// fffff000 00000000 gggggg00 000000ff hhhhhhh0 0000000g hhhhhhh0 00000000
	//
	// b0000000 00000000 cc000000 00000000 00000000 00000000 00000000 00000000
	// 00000eee 00000000 000000ff 00000000 0000000g 00000000 00000000 00000000
	//
	// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
	// 00000000 00000000 00000000 00000000 00000000 0iiiiiii 00jjjjjj 00000000
	//
	// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
	// 00000000 00000000 00000000 00000000 00000000 j0000000 00000000 00000000
	//
	// If we or all four vectors together, we get our desired result.
	//
	// The shuffle masks need to incorporate the byte shift, so this
	// is what we have after the multiplies:
	//
	// 0aaaaaaa 00000000 00bbbbbb b0000000 000ccccc cc000000 0000dddd ddd00000
	// 00000eee eeee0000 000000ff fffff000 0000000g gggggg00 00000000 hhhhhhh0
	//
	// 0iiiiiii 00000000 00jjjjjj j0000000 xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	// xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx
	//
	// The first two masks are simple: [0, z, 2, z, 4, z, ...] and
	// [1, z, 3, z, 5, z, ...]. The latter two need to incorporate the
	// fact that we want to shift upwards, so those need to be
	// [z x 13, 0, 2, z] and [z x 13, 3, z, z].
	//
	// To make this even simpler, we can move each index down to skip the
	// truncation step: [0, 2, 4, ...], [1, 3, 5, ...], etc

	lo = lo.Mul(mulShift)
	hi = hi.Mul(mulShift)

	shuf0 := lo.AsUint8x16().PermuteOrZero(shuffles[0])
	shuf1 := lo.AsUint8x16().PermuteOrZero(shuffles[1])
	shuf2 := hi.AsUint8x16().PermuteOrZero(shuffles[2])
	shuf3 := hi.AsUint8x16().PermuteOrZero(shuffles[3])

	out := shuf0.Or(shuf1).Or(shuf2).Or(shuf3).AsUint64x2().GetElem(0)

	// Now, some cleanup. We have overflow conditions in the following cases:
	//
	// 1. len == 10 and data[9] is greater than 1.
	// 2. len > 10.
	truncated := (xbits.Bit(len == 10) & xbits.Bit(data.GetElem(9) > 1)) | xbits.Bit(len > 10)
	if truncated != 0 {
		p1.Fail(p2, ErrorTruncated)
	}

	return p1, p2, out
}
