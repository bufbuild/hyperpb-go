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

// Package vm contains the core interpreter VM for the hyperpb parser.
//
// This includes the state structs [P1] and [P2], the entry point [Run], and
// all of the helper functions for manipulating the parser state that the
// various [Thunk]s use (these are implemented in another package).
//
// Almost all operations in this package "pass through" the P1/P2 parser state,
// matching the signature of [Thunk]. This is important because it helps guide
// register allocation for all of these functions, which are extremely hot.
// See https://en.wikipedia.org/wiki/Threaded_code for more information on this
// technique.
package vm

import (
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/impl"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/utf8"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/varint"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

// VM state types.
type (
	P1 = impl.P1
	P2 = impl.P2
)

// Varint32 parses a 64-bit varint, but will perform less work by discarding
// arbitrary high bits beyond bit 31.
//
//go:nosplit
func Varint32(p1 P1, p2 P2) (P1, P2, uint64) {
	return varint.Varint32(p1, p2)
}

// Varint64 parses a 64-bit varint.
//
//go:nosplit
func Varint64(p1 P1, p2 P2) (P1, P2, uint64) {
	return varint.Varint64(p1, p2)
}

// Fixed32 parses a 32-bit fixed-width integer.
func Fixed32(p1 P1, p2 P2) (P1, P2, uint32) {
	p1, p2 = p1.AtLeast(p2, 4)
	x := xunsafe.ByteLoad[uint32](p1.Ptr(), 0)
	p1 = p1.Advance(4)

	p1.Log(p2, "fixed32", "%d:%#x (%d bytes)", x, x, 4)
	return p1, p2, x
}

// Fixed64 parses a 64-bit fixed-width integer.
func Fixed64(p1 P1, p2 P2) (P1, P2, uint64) {
	p1, p2 = p1.AtLeast(p2, 8)
	x := xunsafe.ByteLoad[uint64](p1.Ptr(), 0)
	p1 = p1.Advance(8)

	p1.Log(p2, "fixed64", "%d:%#x (%d bytes)", x, x, 8)
	return p1, p2, x
}

// LengthPrefix parses a varint up to the current length.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func LengthPrefix(p1 P1, p2 P2) (P1, P2, int) {
	if p1.Len() == 0 {
		p1.Fail(p2, tdp.ErrorTruncated)
	}

	var n uint64
	p1, p2, n = Varint64(p1, p2)

	// Explicit inlining of atLeast(). len() is guaranteed to fit in a
	// uint32.
	if n > uint64(p1.Len()) {
		p1.Fail(p2, tdp.ErrorTruncated)
	}
	return p1, p2, int(n)
}

// Bytes parses a length-delimited byte buffer.
func Bytes(p1 P1, p2 P2) (P1, P2, zc.Range) {
	var n int
	p1, p2, n = LengthPrefix(p1, p2)

	r := zc.NewRaw(p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src())), n)
	p1 = p1.Advance(n)

	if debug.Enabled {
		text := r.Bytes(p1.Src())
		p1.Log(p2, "bytes", "%#v, %q", r, text)
	}
	return p1, p2, r
}

// UTF8 parses a length-delimited byte buffer, and validates it for UTF8.
//
// Does not preserve p2.Scratch().
func UTF8(p1 P1, p2 P2) (P1, P2, zc.Range) {
	if p2.P3().AllowInvalidUTF8 {
		return Bytes(p1, p2)
	}

	return utf8.Verify(LengthPrefix(p1, p2))
}

// ParseMapEntry is a shim over [PushMessage] used for map entries.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func ParseMapEntry(p1 P1, p2 P2) (P1, P2) {
	var n int
	p1, p2, n = LengthPrefix(p1, p2)
	p1, p2 = p1.SetScratch(p2, uint64(n))

	// This should *not* call PushMapEntry; this goes inside of the message that
	// gets pushed by PushMapEntry itself.
	return p1.PushMessage(p2, p2.Message())
}
