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

package varint

import (
	"math/bits"
	"runtime"
	"simd/archsimd"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/state"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

var (
	signBits128 = archsimd.BroadcastUint8x16(0x7f)

	// And truncMasks[i] with a vector to zero all lanes i or greater, and all
	// sign bits.
	truncMasks = func() [17]archsimd.Uint8x16 {
		var mask archsimd.Uint8x16
		var masks [17]archsimd.Uint8x16
		for i := range masks {
			if i < 16 {
				mask = mask.SetElem(uint8(i), 0xff)
			}
			masks[i] = mask.And(signBits128)
		}

		return masks
	}()

	mulShift = makeU16x8(func(i uint16) uint16 { return 1 << (8 - i) })

	shuffles = [...]archsimd.Int8x16{
		makeI8x16(func(i int8) int8 {
			if i < 7 {
				return i*2 + 1
			}
			return -1
		}),
		makeI8x16(func(i int8) int8 {
			if i < 7 {
				return i*2 + 2
			}
			return -1
		}),
		makeI8x16(func(i int8) int8 {
			if i == 7 {
				return 1
			}
			return -1
		}),
		makeI8x16(func(i int8) int8 {
			if i == 7 {
				return 2
			}
			return -1
		}),
	}

	rotate8Shuffle = makeI8x16(func(u int8) int8 { return (u + 8) % 16 })
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

//hyperpb:stencil AVX32 AVX[uint32]
//hyperpb:stencil AVX64 AVX[uint64]

// AVX is an AVX-accelerated varint parsing function.
//
//go:nosplit
func AVX[T uint32 | uint64](p1 state.P1, p2 state.P2) (state.P1, state.P2, uint64) {
	long := layout.Size[T]() == 8

	start := p1.PtrAddr
	var x uint64

	{
		// Callers often have an inlined fast-path.
		if p1.Len() < 16 {
			return ScalarSplit(p1, p2)
		}

		data := archsimd.LoadUint8x16(xunsafe.Cast[[16]uint8](p1.Ptr()))
		p1.Log(p2, "varint-avx", "data: %b", data)

		// Find all of the continuation bytes. ToBits produces pmovmskb which is
		// what we want, although we have to do a redundant comparison here...
		signs := archsimd.Mask8x16(data.AsInt8x16()).ToBits()

		n := bits.TrailingZeros16(^signs)
		len := bits.TrailingZeros16(^signs) + 1

		// Discard extra bytes and the sign bits.
		data = data.And(truncMasks[n])
		p1.Log(p2, "varint-avx", "len: %d, trunc: %b", len, data)

		var lo, hi archsimd.Uint16x8
		if long {
			lo, hi = split8to16(data)
		} else {
			lo = data.ExtendLo8ToUint16()
		}
		p1.Log(p2, "varint-avx", "lo: %b, hi: %b", lo, hi)

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
		// 0aaaaaaa 00bbbbbb 000ccccc 0000dddd fffff000 gggggg00 hhhhhhh0 00000000
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
		//
		// b0000000 cc000000 ddd00000 eeee0000 00000eee 000000ff 0000000g 00000000
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
		//
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 0iiiiiii
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
		//
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 j0000000
		// 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000
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
		// The shuffles are straightforward from this, remembering that the above
		// is big-endian.

		lo = lo.Mul(mulShift)
		if long {
			hi = hi.Mul(mulShift)
		}
		p1.Log(p2, "varint-avx", "lo: %b, hi: %b", lo, hi)

		// TODO: we can replace two of these the below vmovdqus with a
		// a vpbroadcastb of 1 and vpsubb of the resulting vector, reducing
		// memory traffic.

		shuf0 := lo.AsUint8x16().PermuteOrZero(shuffles[0])
		shuf1 := lo.AsUint8x16().PermuteOrZero(shuffles[1])
		p1.Log(p2, "varint-avx", "0: %b, 1: %b", shuf0, shuf1)
		or := shuf0.Or(shuf1)

		if long {
			shuf2 := hi.AsUint8x16().PermuteOrZero(shuffles[2])
			shuf3 := hi.AsUint8x16().PermuteOrZero(shuffles[3])
			p1.Log(p2, "varint-avx", "2: %b, 3: %b", shuf2, shuf3)
			or = or.Or(shuf2).Or(shuf3)
		}

		p1.Log(p2, "varint-avx", "or: %b", or)
		x = or.AsUint64x2().GetElem(0)
		p1.Log(p2, "varint-avx", "out: %d, %b", x, x)

		// Now, some cleanup. We have overflow conditions in the following cases:
		//
		// 1. len == 10 and data[9] is greater than 1.
		// 2. len > 10.
		if len >= 10 && (len > 10 || data.GetElem(9) > 1) {
			goto fail
		}

		p1.PtrAddr = p1.PtrAddr.Add(len)
	}

	if debug.Enabled {
		len := int(p1.PtrAddr - start) // For debug only.
		p1.Log(p2, "varint-avx", "%d:%#x (%d bytes)", x, x, len)
		runtime.GC() // This checks for the above crash bug.
	}

	return p1, p2, x

fail:
	p1.Fail(p2, tdp.ErrorTruncated)
	for {
	}
}
