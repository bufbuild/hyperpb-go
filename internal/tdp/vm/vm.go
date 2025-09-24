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
	"unsafe"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/xsync"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

const notAGroup = ^tdp.Tag(0)

var (
	stackPool = xsync.Pool[[]frame]{}
	p3Pool    = xsync.Pool[p3]{
		Reset: func(pp *p3) { *pp = p3{} },
	}
)

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

	shared   xunsafe.Addr[dynamic.Shared]
	endGroup tdp.Tag // End-of-group tag.
}

// P2 is the other half of the state for the TDP parser. See [P1].
type P2 struct {
	messageAddr xunsafe.Addr[dynamic.Message]
	fieldAddr   xunsafe.Addr[tdp.FieldParser]
	p3Addr      xunsafe.Addr[p3]

	// A scratch register that is preserved across *most* calls. Thunks
	// do not preserve the scratch register, and some functions in this file
	// do not either.
	scratch uint64
}

// p3 is parser state that is passed behind a pointer.
type p3 struct {
	_ xunsafe.NoCopy

	err   ParseError
	stack struct {
		ptr         xunsafe.Addr[frame]
		top, bottom xunsafe.Addr[frame]
	}

	t_ xunsafe.Addr[tdp.TypeParser]
	Options
}

// frame is a recursion frame for the parser.
type frame struct {
	end     xunsafe.Addr[byte]
	g       tdp.Tag
	message xunsafe.Addr[dynamic.Message]
	ty      xunsafe.Addr[tdp.TypeParser]
	field   xunsafe.Addr[tdp.FieldParser]
}

func (p1 P1) Shared() *dynamic.Shared {
	return p1.shared.AssertValid()
}

func (p1 P1) Arena() *arena.Arena {
	return p1.shared.AssertValid().Arena()
}

func (p1 P1) Src() *byte {
	return p1.shared.AssertValid().Src
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
	return p2.messageAddr.AssertValid()
}

func (p2 P2) Type() *tdp.TypeParser {
	return p2.p3().t_.AssertValid()
}

func (p2 P2) Field() *tdp.FieldParser {
	return p2.fieldAddr.AssertValid()
}

func (p2 P2) p3() *p3 { //nolint:funcorder
	return p2.p3Addr.AssertValid()
}

func (p2 P2) Scratch() uint64 {
	return p2.scratch
}

func (p1 P1) SetScratch(p2 P2, v uint64) (P1, P2) {
	p1.Log(p2, "scratch", "%d:%#x", v, v)
	p2.scratch = v
	return p1, p2
}

func (p1 P1) Len() int {
	return int(p1.EndAddr - p1.PtrAddr)
}

// Fail causes a parse failure by panicking with the given error code.
func (p1 P1) Fail(p2 P2, err ErrorCode) {
	p2.p3().err = ParseError{
		code:   err,
		offset: p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src())),
	}

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
	height := p2.p3().stack.bottom.Sub(p2.p3().stack.ptr)
	var b byte
	if p1.PtrAddr < p1.EndAddr {
		b = *p1.Ptr()
	}
	debug.Log(
		[]any{
			"%p:%p:%d %v [%d:%d] = 0x%02x",
			p1.Shared(), p2.Message(), height, p1.endGroup, start, end, b,
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

	p1.Fail(p2, ErrorTruncated)
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

// Varint parses a 64-bit varint.
//
//go:nosplit
func (p1 P1) Varint(p2 P2) (P1, P2, uint64) {
	if debug.Enabled {
		// Force this function to behave as if it is not nosplit in debug mode,
		// so that we don't overflow the nosplit stack when we turn on
		// debugging.
		return parseVarintNoinline(p1, p2)
	}

	return parseVarint(p1, p2)
}

// Fixed32 parses a 32-bit fixed-width integer.
func (p1 P1) Fixed32(p2 P2) (P1, P2, uint32) {
	p1, p2 = p1.AtLeast(p2, 4)
	x := xunsafe.ByteLoad[uint32](p1.Ptr(), 0)
	p1 = p1.Advance(4)

	p1.Log(p2, "fixed32", "%d:%#x (%d bytes)", x, x, 4)
	return p1, p2, x
}

// Fixed64 parses a 64-bit fixed-width integer.
func (p1 P1) Fixed64(p2 P2) (P1, P2, uint64) {
	p1, p2 = p1.AtLeast(p2, 8)
	x := xunsafe.ByteLoad[uint64](p1.Ptr(), 0)
	p1 = p1.Advance(8)

	p1.Log(p2, "fixed64", "%d:%#x (%d bytes)", x, x, 8)
	return p1, p2, x
}

// LengthPrefix parses a varint up to the current length.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (p1 P1) LengthPrefix(p2 P2) (P1, P2, int) {
	if p1.Len() == 0 {
		p1.Fail(p2, ErrorTruncated)
	}

	var n uint64
	p1, p2, n = p1.Varint(p2)

	// Explicit inlining of atLeast(). len() is guaranteed to fit in a
	// uint32.
	if n > uint64(p1.Len()) {
		p1.Fail(p2, ErrorTruncated)
	}
	return p1, p2, int(n)
}

// Bytes parses a length-delimited byte buffer.
func (p1 P1) Bytes(p2 P2) (P1, P2, zc.Range) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)

	r := zc.NewRaw(p1.PtrAddr.Sub(xunsafe.AddrOf(p1.Src())), n)
	p1 = p1.Advance(n)

	if debug.Enabled {
		text := r.Bytes(p1.Src())
		p1.Log(p2, "bytes", "%#v, %q", r, text)
	}
	return p1, p2, r
}

// UTF8 parses a length-delimited byte buffer, and validates it for UTF8.
func (p1 P1) UTF8(p2 P2) (P1, P2, zc.Range) {
	if p2.p3().AllowInvalidUTF8 {
		return p1.Bytes(p2)
	}

	return verifyUTF8(p1.LengthPrefix(p2))
}

// ParseMapEntry is a shim over [PushMessage] used for map entries.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (p1 P1) ParseMapEntry(p2 P2) (P1, P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)
	p1, p2 = p1.SetScratch(p2, uint64(n))

	// This should *not* call PushMapEntry; this goes inside of the message that
	// gets pushed by PushMapEntry itself.
	return p1.PushMessage(p2, p2.Message())
}

// PushMessage pushes a new message to be parsed onto the parser stack.
//
// The length of the message should be in p2.Scratch.
//
//go:nosplit
func (p1 P1) PushMessage(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch())
	if len == 0 {
		return p1, p2
	}

	p1.Log(p2, "n", "%d", len)

	if p1.endGroup != notAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.endGroup = notAGroup
	p2.messageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.p3().t_ = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.fieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushMapEntry pushes a new map entry to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushMapEntry(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch())
	if len == 0 {
		return p1, p2
	}

	if p1.endGroup != notAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.endGroup = notAGroup
	p2.messageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser.MapEntry
	p2.p3().t_ = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.fieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushGroup pushes a new group to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushGroup(p2 P2, m *dynamic.Message) (P1, P2) {
	start := tdp.Tag(p2.Scratch())

	// Indeed, we can just +1, because we need to replace the low three
	// 0b011 bits with 0b100. Much simpler than clearing and overwriting those
	// bits!
	end := start + 1

	p1, p2 = p1.push(p2, p1.EndAddr)

	p1.endGroup = end
	p2.messageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.p3().t_ = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.fieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logMessage(p1 P1, p2 P2) (P1, P2) {
	p1.Log(
		p2, "new", "%#x, %v",
		p2.messageAddr,
		p2.Message().Type(),
	)
	return p1, p2
}

func (p3 *p3) stackSlice() []frame {
	n := p3.stack.bottom.Sub(p3.stack.ptr)
	return unsafe.Slice(p3.stack.ptr.AssertValid(), n)
}

// push pushes a parser frame.
//
//go:nosplit
func (p1 P1) push(p2 P2, end xunsafe.Addr[byte]) (P1, P2) {
	if debug.Enabled {
		p1, p2 = logPush(p1, p2)
	}

	if p2.p3().stack.ptr == p2.p3().stack.top {
		p1.Fail(p2, ErrorRecursionDepth)
	}

	p2.p3().stack.ptr = p2.p3().stack.ptr.Add(-1)

	// Note: a single frame is just too large to hit Go's SROA pass (same bug
	// that results in p1/p2 being two structs). Thus, we write each field
	// separately to avoid wasteful stack traffic.
	frame := p2.p3().stack.ptr.AssertValid()
	frame.end = p1.EndAddr
	frame.g = p1.endGroup
	frame.message = p2.messageAddr
	frame.ty = p2.p3().t_
	frame.field = p2.fieldAddr

	p1.EndAddr = end
	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logPush(p1 P1, p2 P2) (P1, P2) {
	p1.Log(p2, "push", "%v/%v/%v", p2.p3().stack.top, p2.p3().stack.ptr, p2.p3().stack.bottom)
	return p1, p2
}

// pop pops a parser frame.
//
// Returns whether the last frame was popped.
//
//go:nosplit
func (p1 P1) pop(p2 P2) (P1, P2, bool) {
	if debug.Enabled {
		p1.Log(
			p2, "finish", "%v, ty: %p:%s %v",
			p2.messageAddr,
			p2.Message().Type(),
			p2.Message().Type().Descriptor.FullName(),
			p2.Message().Type(),
		)

		s := &p2.p3().stack
		p1.Log(p2, "pop", "%v/%v/%v\n%s", s.top, s.ptr, s.bottom,
			p2.Message().Dump())
	}

	last := p2.p3().stack.ptr.AssertValid()
	p1.EndAddr = last.end
	p1.endGroup = last.g
	p2.messageAddr = last.message
	p2.p3().t_ = last.ty
	p2.fieldAddr = last.field
	p2.p3().stack.ptr = p2.p3().stack.ptr.Add(1)

	return p1, p2, p2.p3().stack.ptr == p2.p3().stack.bottom
}

func (p1 P1) byTag(p2 P2, tag2 uint64) (P1, P2, uint64) {
	t := p2.Type()
	p := swiss.LookupI32xU32(t.Tags, int32(tag2))
	if p == nil {
		p2.fieldAddr = 0
		return p1, p2, tag2
	}
	p2.fieldAddr = xunsafe.AddrOf(t.Fields().Get(int(*p)))
	return p1, p2, tag2
}
