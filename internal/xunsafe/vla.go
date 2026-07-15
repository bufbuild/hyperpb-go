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
	"unsafe"

	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// VLA is a mechanism for accessing a variable-length array that follows
// some struct.
type VLA[T any] [0]T

// Beyond obtains the VLA past the end of p.
//
// Address calculation assumes that p is well-aligned.
func Beyond[T, Header any](p *Header) *VLA[T] {
	// The below code performs the following address calculation without
	// triggering a load (Go likes to perform loads of the result of pointer
	// arithmetic like the following).
	//
	//  &Cast[struct {
	//    _   Header
	//    VLA VLA[T]
	//  }](p).VLA

	align := layout.Align[T]()
	return Addr[VLA[T]](
		AddrOf(p).Add(1).RoundUpTo(align),
	).AssertValid()
}

// Get returns a pointer to the nth element of this array.
func (a *VLA[T]) Get(n int) *T {
	return Add(Cast[T](a), n)
}

// ByteGet returns a pointer to the element of this array at the given byte offset.
func (a *VLA[T]) ByteGet(n int) *T {
	return ByteAdd[T](a, n)
}

// Slice converts this VLA into a slice of the given length.
func (a *VLA[T]) Slice(n int) []T {
	return unsafe.Slice(a.Get(0), n)
}
