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

// Package impl contains the definition of the VM's state, as well as core
// VM operations. The actual VM interpreter lives in the vm package.
//
// This package exists so that vm/internal packages can access these types
// without being part of the vm package.
package impl

import (
	"unsafe"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/options"
	"buf.build/go/hyperpb/internal/xunsafe"
)

const NotAGroup = ^tdp.Tag(0)

// P1 is half of the state for the TDP parser.
//
// This struct must no more than four fields, and all four fields must be
// word-sized or smaller, so that it fits in registers AND does not trigger
// go.dev/issue/72897.
//
// For this reason, the parser state is split into two structs that will fit
// in registers and will not be spilled. This means that functions with the
// [parseFunc] signature will keep all of the parser data in registers with
// minimal spillage. Ideally this would all be in a single struct, but see the
// above bug.
//
// Moreover, these structs should contain no pointers; pointers have instead
// been replaced with addresses, all of which are rooted at the call to
// startParse. This avoids unnecessary spilling for GC stack scanning, since
// those pointers are already findable elsewhere.
//
// Generic parser functions are homed under P1, with a P2 argument,
// such that these functions have the following signature:
//
//	func(P1, P2) (P1, P2)
//
// Some functions do not have the signature because they are guaranteed inline
// candidates.
//
// Note that returning no values is slower than returning the parser state: this
// is because it will force the caller to spill the parser state across the
// call.
//
// The Go register ABI means P1 and P2 occupy the following registers:
//
//	x86:     rax, rbx, rcx, rdi, rsi,  r8,  r9, r10
//	aarch64:  r0,  r1,  r2,  r3,  r4,  r5,  r6,  r7
type P1 struct {
	PtrAddr xunsafe.Addr[byte]
	EndAddr xunsafe.Addr[byte] // One past the end of the stream.

	shared   xunsafe.Addr[dynamic.Shared]
	EndGroup tdp.Tag // End-of-group tag.
}

// P2 is the other half of the state for the TDP parser. See [P1].
type P2 struct {
	MessageAddr xunsafe.Addr[dynamic.Message]
	FieldAddr   xunsafe.Addr[tdp.FieldParser]
	p3Addr      xunsafe.Addr[P3]

	// A scratch register that is preserved across *most* calls. Thunks
	// do not preserve the Scratch register, and some functions in this file
	// do not either.
	scratch uint64
}

// P3 is parser state that is passed behind a pointer.
type P3 struct {
	_ xunsafe.NoCopy

	err   tdp.Error
	stack stack

	ty xunsafe.Addr[tdp.TypeParser]
	options.Options

	frames *[]frame
}

func (p1 P1) Shared() *dynamic.Shared { return p1.shared.AssertValid() }
func (p1 P1) Arena() *arena.Arena     { return p1.shared.AssertValid().Arena() }
func (p1 P1) Src() *byte              { return p1.shared.AssertValid().Src }

func (p2 P2) Message() *dynamic.Message { return p2.MessageAddr.AssertValid() }
func (p2 P2) Field() *tdp.FieldParser   { return p2.FieldAddr.AssertValid() }
func (p2 P2) P3() *P3                   { return p2.p3Addr.AssertValid() }
func (p2 P2) Type() *tdp.TypeParser     { return p2.P3().ty.AssertValid() }

// Ptr returns the current cursor within the message.
//
// This is preferred to PtrAddr.AssertValid() because it checks for a specific
// GC-panicking bug.
func (p1 P1) Ptr() *byte {
	// There is an exciting bug that can occur where we dereference p1.b_
	// while it points to the end of the input slice. Being able to do have
	// p1.b_ equal the one-past-the-end spot is nice, but if we dereference it,
	// Go may scan through this pointer, and mark the allocation it points to.
	// If it happens to point to freed memory, the GC panics, because this is
	// an unrecoverable constraint violation.
	//
	// This assert makes sure that none of our large test suite accidentally
	// performs this illegal maneuver.
	//
	// Annoyingly this means we also need to be careful in parser1.buf(),
	// because we cannot form a zero-sized slice to the end of an allocation.
	debug.Assert(p1.PtrAddr < p1.EndAddr,
		"p1.PtrAddr cannot point one past the end: need %v < %v", p1.PtrAddr, p1.EndAddr)
	return p1.PtrAddr.AssertValid()
}

// Len returns the length of [P1.Buf].
func (p1 P1) Len() int {
	return int(p1.EndAddr - p1.PtrAddr)
}

// Buf returns the data left to parse.
func (p1 P1) Buf() []byte {
	if p1.Len() == 0 {
		return nil
	}
	return unsafe.Slice(p1.Ptr(), p1.Len())
}

// Scratch returns the scratch register.
func (p2 P2) Scratch() uint64 {
	return p2.scratch
}

// Scratch sets the scratch register.
//
// The caller is responsible for spilling this value if necessary.
func (p1 P1) SetScratch(p2 P2, v uint64) (P1, P2) {
	p1.Log(p2, "scratch", "%d:%#x", v, v)
	p2.scratch = v
	return p1, p2
}

// Fail causes a parse failure by panicking with the given error code.
func (p1 P1) Fail(p2 P2, err tdp.ErrorCode) {
	p2.P3().err = err.ErrorAt(p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src())))

	// Trigger a panic without calling runtime.gopanic.
	// The NoEscape is to silence clueless linters.
	_ = *xunsafe.NoEscape[*byte](nil)

	for { //nolint:staticcheck // This code is unreachable.
	}
}

// Log logs debugging information during a parse.
func (p1 P1) Log(p2 P2, op, format string, args ...any) {
	if !debug.Enabled {
		return
	}

	start := p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src()))
	end := p1.EndAddr.Sub(xunsafe.AddrOf(p1.Src()))
	height := p2.P3().stack.bottom.Sub(p2.P3().stack.ptr)
	var b byte
	if p1.PtrAddr < p1.EndAddr {
		b = *p1.Ptr()
	}
	debug.Log(
		[]any{
			"%p:%p:%d %v [%d:%d] = 0x%02x",
			p1.Shared(), p2.Message(), height, p1.EndGroup, start, end, b,
		},
		op, format, args...,
	)
}

// AtLeast fails the parse if there aren't at least n bytes left to parse.
//
//go:nosplit
func (p1 P1) AtLeast(p2 P2, n uint64) (P1, P2) {
	if n <= uint64(p1.Len()) {
		return p1, p2
	}

	p1.Fail(p2, tdp.ErrorTruncated)
	return p1, p2
}

// Advance advances the cursor by n bytes.
func (p1 P1) Advance(n int) P1 {
	debug.Assert(p1.Len() >= n, "parser overflow")

	p1.PtrAddr = p1.PtrAddr.Add(n)
	return p1
}

// ByTag sets the current field to the field with the given tag.
//
// Sets the field to nil if no field with such a tag exists.
func (p1 P1) ByTag(p2 P2, tag uint64) (P1, P2, uint64) {
	t := p2.Type()
	p := swiss.LookupI32xU32(t.Tags, int32(tag))
	if p == nil {
		p2.FieldAddr = 0
		return p1, p2, tag
	}
	p2.FieldAddr = xunsafe.AddrOf(t.Fields().Get(int(*p)))
	return p1, p2, tag
}
