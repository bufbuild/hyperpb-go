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

package xunsafe

import (
	"fmt"
	"unsafe"

	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// intptr is an integer type with the same layout as a uintptr but signed.
//
// On every platform we support, int and uintptr have the same layout.
type intptr int

// Addr is a typed raw address.
//
// The underlying type is an int64 in order to work around a Go codegen bug.
// The bug is essentially that we want to do an arithmetic shift on the value,
// which requires casting what would normally be a uintptr to int64. For some
// reason, when in a generic context, this confuses Go's inliner *just
// enough* to cause things to fail to inline, resulting in a generic function
// call on the critical path.
type Addr[T any] intptr

// AddrOf gets the address of a pointer.
func AddrOf[P ~*E, E any](p P) Addr[E] {
	return Addr[E](uintptr(unsafe.Pointer(p)))
}

// EndOf calculates the one-past-the-end address of s without creating an
// intermediate one-past-the-end pointer.
func EndOf[S ~[]E, E any](s S) Addr[E] {
	return AddrOf(unsafe.SliceData(s)).Add(len(s))
}

// AssertValid asserts that this address is a valid pointer.
//
//go:nosplit
func (a Addr[T]) AssertValid() *T {
	return (*T)(unsafe.Pointer(uintptr(a))) // Don't worry about it.
}

// Add adds the given offset to this address.
func (a Addr[T]) Add(n int) Addr[T] {
	return a + Addr[T](n*layout.Size[T]())
}

// ByteAdd adds the given unscaled offset to this address.
func (a Addr[T]) ByteAdd(n int) Addr[T] {
	return a + Addr[T](n)
}

// Sub returns the offset between two addresses.
func (a Addr[T]) Sub(b Addr[T]) int {
	return int(a-b) / layout.Size[T]()
}

// Padding returns the number of bytes between this address and the next address
// aligned to the given alignment, which must be a power of two.
func (a Addr[T]) Padding(align int) int {
	return layout.Padding(int(a), align)
}

// RoundUpTo rounds this address upwards to align, which must be a power of two.
func (a Addr[T]) RoundUpTo(align int) Addr[T] {
	return Addr[T](layout.RoundUp(uintptr(a), uintptr(align)))
}

// SignBit returns whether this address has its sign bit set.
//
// Pointers with the high bits set are never used by Go, so we can use this bit
// to store extra information.
func (a Addr[T]) SignBit() bool {
	return a>>(layout.Bits[Addr[T]]()-1) != 0
}

// SignBitMask returns either all zeros or all ones, according to the sign bit
// of a.
func (a Addr[T]) SignBitMask() Addr[T] {
	return a >> (layout.Bits[Addr[T]]() - 1)
}

// ClearSignBit clears the sign bit of this address, flipping all of the other
// bits in the process.
func (a Addr[T]) ClearSignBit() Addr[T] {
	return a ^ a.SignBitMask()
}

// Format implements [fmt.Formatter].
func (a Addr[T]) Format(state fmt.State, verb rune) {
	if verb == 'v' {
		fmt.Fprintf(state, "%#x", uintptr(a))
		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, verb), uintptr(a))
}
