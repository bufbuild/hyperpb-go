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
	"unsafe"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/xunsafe"
)

//go:generate go run ../../tools/hyperstencil

// This file contains inlined or modified versions of functions from
// package [dynamic], since Go cannot seem to inline them, resulting in
// spills of p1/p2 across hot calls.

// MutableCold is like [message.MutableCold], but with a parser-friendly ABI.
func MutableCold(p1 P1, p2 P2) (P1, P2, *dynamic.Cold) {
	if p2.Message().ColdIndex < 0 {
		size := int(p2.Message().Type().ColdSize)
		cold := xunsafe.Cast[dynamic.Cold](p1.Arena().Alloc(size))
		p2.Message().ColdIndex = int32(len(p1.Shared().Cold))
		p1.Shared().Cold = append(p1.Shared().Cold, cold)
		return p1, p2, cold
	}
	return p1, p2, xunsafe.LoadSlice(p1.Shared().Cold, p2.Message().ColdIndex)
}

// GetMutableField returns the field data for a given message. Uses p2.m and p2.f for
// the message and offset.
//
// If this field is in the cold region, it allocates one.
func GetMutableField[T any](p1 P1, p2 P2) (P1, P2, *T) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	return p1, p2, (*T)(p)
}

// StoreField loads a field pointer using [GetMutableField] and stores v to it.
//

func StoreField[T any](p1 P1, p2 P2, v T) (P1, P2) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	*(*T)(p) = v
	return p1, p2
}

//hyperpb:stencil StoreFromScratch32 StoreFromScratch[uint32]
//hyperpb:stencil StoreFromScratch64 StoreFromScratch[uint64]

// StoreFromScratch is like [StoreField], but it uses p2.scratch as the value to
// store. This is useful for avoiding spills, by writing the temporary parsed
// value into the call-preserved scratch register.
func StoreFromScratch[T tdp.Int](p1 P1, p2 P2) (P1, P2) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	*(*T)(p) = T(p2.Scratch())
	return p1, p2
}

// SetBit is like [dynamic.Message.SetBit], but with a different ABI that keeps parser
// state in registers.
//
// It can only be used to set bits to true, and it draws m and n from
// p1 and p2.
func SetBit(p1 P1, p2 P2) (P1, P2) {
	n := int(p2.Field().Offset.Bit)
	word := xunsafe.Add(xunsafe.Cast[uint32](xunsafe.Add(p2.Message(), 1)), n/32)
	mask := uint32(1) << (n % 32)
	*word |= mask
	return p1, p2
}

// AllocMessage allocates a message value, using the type in p2.Field().
//
//go:noinline
func AllocMessage(p1 P1, p2 P2) (P1, P2, *dynamic.Message) {
	ty := p1.Shared().Library().AtOffset(p2.Field().Message.TypeOffset)
	size := int(ty.Size)

	// Open-coded copy of arena.Alloc, which otherwise would not inline.
	a := p1.Arena()
	// Messages are always pointer-aligned, so we can skip this part.
	// size += arena.Align - 1
	// size &^= arena.Align - 1

	var n xunsafe.Addr[byte]
	n, a.Next = a.Next, a.Next.Add(size)
	if a.Next <= a.End {
		p := n.AssertValid()
		a.Log("alloc", "%v:%v, %d:%d", p, a.Next, size, arena.Align)

		// Go seems unwilling to inline AllocInPlace() here.
		m := xunsafe.Cast[dynamic.Message](p)
		xunsafe.StoreNoWB(&m.Shared, p1.Shared())
		m.TypeOffset = p2.Field().Message.TypeOffset
		m.ColdIndex = -1
		return p1, p2, m
	}

	a.Next = n
	a.Grow(size)

	// This call is guaranteed to not infinite recurse.
	// Doing a goto to the top of the function seems to confuse Go's register
	// allocator, causing it to spill p1 and p2 in the prologue.
	return AllocMessage(p1, p2)
}

// AllocInPlace is like [AllocMessage], but only performs initialization using
// the given data value.
//
//go:nosplit
func AllocInPlace(p1 P1, p2 P2, data *byte) (P1, P2, *dynamic.Message) {
	m := xunsafe.Cast[dynamic.Message](data)
	xunsafe.StoreNoWB(&m.Shared, p1.Shared())
	m.TypeOffset = p2.Field().Message.TypeOffset
	m.ColdIndex = -1
	return p1, p2, m
}

//go:nosplit
func getUntypedMutableField(p1 P1, p2 P2) (P1, P2, unsafe.Pointer) {
	offset := p2.Field().Offset.Data
	if offset >= 0 {
		return p1, p2, unsafe.Add(unsafe.Pointer(p2.Message()), offset)
	}
	var cold *dynamic.Cold
	p1, p2, cold = MutableCold(p1, p2)
	return p1, p2, unsafe.Add(unsafe.Pointer(cold), ^offset)
}
