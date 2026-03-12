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

// Package state contains the definition of the VM's state.
//
// This package exists so that vm/internal packages can access these types
// without being part of the vm package.
package state

import (
	"unsafe"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
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
// Generic parser functions are homed under P1, with a parser2 argument,
// such that these functions have the following signature:
//
//	func(P1, parser2) (P1, parser2)
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

	SharedPtr xunsafe.Addr[dynamic.Shared]
	EndGroup  tdp.Tag // End-of-group tag.
}

// P2 is the other half of the state for the TDP parser. See [P1].
type P2 struct {
	MessageAddr xunsafe.Addr[dynamic.Message]
	FieldAddr   xunsafe.Addr[tdp.FieldParser]
	P3Addr      xunsafe.Addr[P3]

	// A Scratch register that is preserved across *most* calls. Thunks
	// do not preserve the Scratch register, and some functions in this file
	// do not either.
	Scratch uint64
}

func (p1 P1) Shared() *dynamic.Shared {
	return p1.SharedPtr.AssertValid()
}

func (p1 P1) Arena() *arena.Arena {
	return p1.SharedPtr.AssertValid().Arena()
}

func (p1 P1) Src() *byte {
	return p1.SharedPtr.AssertValid().Src
}

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

func (p2 P2) Message() *dynamic.Message {
	return p2.MessageAddr.AssertValid()
}

func (p2 P2) Type() *tdp.TypeParser {
	return p2.P3().Type.AssertValid()
}

func (p2 P2) Field() *tdp.FieldParser {
	return p2.FieldAddr.AssertValid()
}

func (p2 P2) P3() *P3 { //nolint:funcorder
	return p2.P3Addr.AssertValid()
}

func (p1 P1) SetScratch(p2 P2, v uint64) (P1, P2) {
	p1.Log(p2, "scratch", "%d:%#x", v, v)
	p2.Scratch = v
	return p1, p2
}

func (p1 P1) Len() int {
	return int(p1.EndAddr - p1.PtrAddr)
}

// Fail causes a parse failure by panicking with the given error code.
func (p1 P1) Fail(p2 P2, err tdp.ErrorCode) {
	p2.P3().Err = err.ErrorAt(p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src())))

	_ = *(*byte)(nil) // Trigger a panic without calling runtime.gopanic. Linters hate this!
	for {             //nolint:staticcheck // This code is unreachable.
	}
}

// Log logs debugging information during a parse.
func (p1 P1) Log(p2 P2, op, format string, args ...any) {
	if !debug.Enabled {
		return
	}

	start := p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src()))
	end := p1.EndAddr.Sub(xunsafe.AddrOf(p1.Src()))
	height := p2.P3().Stack.Bottom.Sub(p2.P3().Stack.Ptr)
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

// Buf returns the data left to parse.
func (p1 P1) Buf() []byte {
	if p1.Len() == 0 {
		return nil
	}
	return unsafe.Slice(p1.Ptr(), p1.Len())
}

func (p1 P1) Advance(n int) P1 {
	debug.Assert(p1.Len() >= n, "parser overflow")

	p1.PtrAddr = p1.PtrAddr.Add(n)
	return p1
}

func (p1 P1) ByTag(p2 P2, tag2 uint64) (P1, P2, uint64) {
	t := p2.Type()
	p := swiss.LookupI32xU32(t.Tags, int32(tag2))
	if p == nil {
		p2.FieldAddr = 0
		return p1, p2, tag2
	}
	p2.FieldAddr = xunsafe.AddrOf(t.Fields().Get(int(*p)))
	return p1, p2, tag2
}

// PushMessage pushes a new message to be parsed onto the parser stack.
//
// The length of the message should be in p2.Scratch.
//
//go:nosplit
func (p1 P1) PushMessage(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch)
	if len == 0 {
		return p1, p2
	}

	p1.Log(p2, "n", "%d", len)

	if p1.EndGroup != NotAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.EndGroup = NotAGroup
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.P3().Type = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushMapEntry pushes a new map entry to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushMapEntry(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch)
	if len == 0 {
		return p1, p2
	}

	if p1.EndGroup != NotAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.EndGroup = NotAGroup
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser.MapEntry
	p2.P3().Type = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushGroup pushes a new group to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushGroup(p2 P2, m *dynamic.Message) (P1, P2) {
	start := tdp.Tag(p2.Scratch)

	// Indeed, we can just +1, because we need to replace the low three
	// 0b011 bits with 0b100. Much simpler than clearing and overwriting those
	// bits!
	end := start + 1

	p1, p2 = p1.push(p2, p1.EndAddr)

	p1.EndGroup = end
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.P3().Type = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logMessage(p1 P1, p2 P2) (P1, P2) {
	p1.Log(
		p2, "new", "%#x, %v",
		p2.MessageAddr,
		p2.Message().Type(),
	)
	return p1, p2
}

func (p3 *P3) stackSlice() []Frame {
	n := p3.Stack.Bottom.Sub(p3.Stack.Ptr)
	return unsafe.Slice(p3.Stack.Ptr.AssertValid(), n)
}

// push pushes a parser frame.
//
//go:nosplit
func (p1 P1) push(p2 P2, end xunsafe.Addr[byte]) (P1, P2) {
	if debug.Enabled {
		p1, p2 = logPush(p1, p2)
	}

	if p2.P3().Stack.Ptr == p2.P3().Stack.Top {
		p1.Fail(p2, tdp.ErrorRecursionDepth)
	}

	p2.P3().Stack.Ptr = p2.P3().Stack.Ptr.Add(-1)

	// Note: a single frame is just too large to hit Go's SROA pass (same bug
	// that results in p1/p2 being two structs). Thus, we write each field
	// separately to avoid wasteful stack traffic.
	frame := p2.P3().Stack.Ptr.AssertValid()
	frame.End = p1.EndAddr
	frame.Group = p1.EndGroup
	frame.Message = p2.MessageAddr
	frame.Type = p2.P3().Type
	frame.Field = p2.FieldAddr

	p1.EndAddr = end
	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logPush(p1 P1, p2 P2) (P1, P2) {
	p1.Log(p2, "push", "%v/%v/%v", p2.P3().Stack.Top, p2.P3().Stack.Ptr, p2.P3().Stack.Bottom)
	return p1, p2
}

// Pop pops a parser frame.
//
// Returns whether the last frame was popped.
//
//go:nosplit
func (p1 P1) Pop(p2 P2) (P1, P2, bool) {
	if debug.Enabled {
		p1.Log(
			p2, "finish", "%v, ty: %p:%s %v",
			p2.MessageAddr,
			p2.Message().Type(),
			p2.Message().Type().Descriptor.FullName(),
			p2.Message().Type(),
		)

		s := &p2.P3().Stack
		p1.Log(p2, "pop", "%v/%v/%v\n%s", s.Top, s.Ptr, s.Bottom,
			p2.Message().Dump())
	}

	last := p2.P3().Stack.Ptr.AssertValid()
	p1.EndAddr = last.End
	p1.EndGroup = last.Group
	p2.MessageAddr = last.Message
	p2.P3().Type = last.Type
	p2.FieldAddr = last.Field
	p2.P3().Stack.Ptr = p2.P3().Stack.Ptr.Add(1)

	return p1, p2, p2.P3().Stack.Ptr == p2.P3().Stack.Bottom
}
