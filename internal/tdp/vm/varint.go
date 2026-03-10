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
	"runtime"

	"buf.build/go/hyperpb/internal/debug"
)

// Overridable at runtime, if good SIMD features are detected.
var parseVarint = parseVarintScalar

// parseVarint is the core varint parsing implementation.
//
//go:nosplit
//go:noinline
func parseVarintScalar(p1 P1, p2 P2) (P1, P2, uint64) {
	// Inlined from protowire.ConsumeVarint to minimize spills and remove
	// bounds checks.
	var b byte
	var x uint64
	var i int

	start := p1.PtrAddr

	// NOTE: Previously, we would load *p and then increment it before checking
	// if b's sign bit was clear. This resulted in the accidental creation of
	// a one-past-the-end pointer, which would result in a rather nasty and
	// rare GC crash, if the GC happened to preempt this function in one of
	// a very small subset of PC values.
	//
	// We also need to perform length checks here, since invalid input can cause
	// the creation of a one-past-the-end pointer. Because the input is
	// untrusted, we have to make the check. The check is carefully written in
	// such a way as to produce the best code and be as
	// branch-predictor-friendly as possible.

	// No bounds check for the first one; we assume parseVarint is not called
	// when the buffer is empty.

	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	if p1.PtrAddr == p1.EndAddr {
		goto fail
	}
	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p1.PtrAddr.AssertValid()
	p1.PtrAddr++
	x |= uint64(b) << (i * 7)
	if b <= 1 {
		goto exit
	}

	p1.Fail(p2, ErrorOverflow)

exit:
	if debug.Enabled {
		len := int(p1.PtrAddr - start) // For debug only.
		p1.Log(p2, "varint", "%d:%#x (%d bytes)", x, x, len)
		runtime.GC() // This checks for the above crash bug.
	}

	return p1, p2, x

fail:
	p1.Fail(p2, ErrorTruncated)
	goto fail
}

//go:noinline
func parseVarintNoinline(p1 P1, p2 P2) (P1, P2, uint64) {
	return parseVarint(p1, p2)
}

//go:noinline
func parseVarintScalarNoinline(p1 P1, p2 P2) (P1, P2, uint64) {
	return parseVarint(p1, p2)
}
