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

import "unsafe"

// ByteAdd adds the given offset to p, without scaling.
//
// It also throws in a cast for free.
//
// This is used in the VM to decode varints, but the VM unconditionally loads 8
// bytes for performance reasons. In practice, this is not a problem - if we
// end up loading data from another object, that data would be masked off, or
// handled by length checks later on.
//
// checkptr does not like us doing this. It could be fixed with a bounds check
// here, but that would add an unnecessary branch in the hottest part of the VM
// without changing observable behavior.
//
//go:nocheckptr
func ByteAdd[T any, P ~*E, E any, I Int](p P, n I) *T {
	return (*T)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(n)))
}

// Sub computes the difference between two pointers, without scaling.
func ByteSub[P1 ~*E1, P2 ~*E2, E1, E2 any](p1 P1, p2 P2) int {
	return int(uintptr(unsafe.Pointer(p1)) - uintptr(unsafe.Pointer(p2)))
}

// ByteLoad loads a value of the given type at the given byte offset.
func ByteLoad[T any, P ~*E, E any, I Int](p P, n I) T {
	return *ByteAdd[T](p, n)
}

// ByteLoad stores a value of the given type at the given byte offset.
func ByteStore[T any, P ~*E, E any, I Int](p P, n I, v T) {
	*ByteAdd[T](p, n) = v
}
