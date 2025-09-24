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

// Package arena provides a low-level, relatively unsafe arena allocation
// abstraction.
//
// # Design
//
// See <https://mcyoung.xyz/2025/04/21/go-arenas/>.
//
// Arenas are designed to only return pointers to data with pointer-free shape.
// However, we would like to store pointers in this data, so that the arena can
// point to itself (and to no other memory)
//
// This means that to store such data, the pointers must either live for the
// same lifetime as the [Arena] value (such as by storing them alongside it) or
// must point back into the Arena.
//
// We ensure this by making it so that holding a pointer onto any memory
// allocated by an [Arena] will keep all memory reachable from it alive.
// We achieve this by having the shape of each chunk allocated for the arena
// contain a pointer to the arena as a header; each chunk thus must have the
// shape
//
//	type chunk struct {
//	  memory [N]uint64
//	  arena *Arena
//	}
//
// By holding a pointer into chunk.memory anywhere reachable by a GC root (such
// as in a local variable) the GC will mark the allocation for the whole chunk
// as live, and therefore mark the [*Arena] field as live. Tracing through
// chunk.arena.chunks will mark all the other chunks as alive.
//
// Memory not directly allocated by an arena can be tied to it using
// [Arena.KeepAlive]. Using this operation is very slow, since this is the one
// part of the arena that is not re-used when calling [Arena.Free].
package arena

import (
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// Arena is an Arena for holding values of any type which does not contain
// pointers.
//
// A zero Arena is empty and ready to use.
type Arena struct {
	_ xunsafe.NoCopy

	// Exported to allow for open-coding of Alloc() in some hot callsites,
	// because Go won't inline it >_>
	Next, End xunsafe.Addr[byte]
	Cap       int // Always a power of 2.

	// Blocks of memory allocated by this arena. Indexed by their size log 2.
	blocks []*byte

	// Data to keep around for the GC to mark whenever it marks an arena.
	// Holding any pointer to the arena will keep anything here alive, too.
	keep []unsafe.Pointer
}

// Align is the alignment of all objects on the arena.
const Align = int(unsafe.Sizeof(uintptr(0)))

// New allocates a new value of type T on an arena.
func New[T any](a *Arena, value T) *T {
	layout := layout.Of[T]()
	if layout.Align > Align {
		panic("hyperpb: over-aligned object")
	}

	p := xunsafe.Cast[T](a.Alloc(layout.Size))
	*p = value
	return p
}

// KeepAlive ensures that v is not swept by the GC until all pointers into the
// arena go away.
func (a *Arena) KeepAlive(v any) {
	a.keep = append(a.keep, unsafe.Pointer(xunsafe.AnyData(v)))
}

// Alloc allocates memory with the given size.
//
// All memory is pointer-aligned. If zero is true, the memory will be zeroed.
//
//go:nosplit
func (a *Arena) Alloc(size int) *byte {
	// Align size to a pointer boundary.
	size += Align - 1
	size &^= Align - 1

	if a.Next.Add(size) <= a.End {
		// Duplicating this block ensures that Go schedules this branch
		// correctly. This block is the "hot" side of the branch.
		p := a.Next.AssertValid()
		a.Next = a.Next.Add(size)
		a.Log("alloc", "%v:%v, %d:%d", p, a.Next, size, Align)
		return p
	}

	a.Grow(size)
	p := a.Next.AssertValid()
	a.Next = a.Next.Add(size)
	a.Log("alloc", "%v:%v, %d:%d", p, a.Next, size, Align)
	return p
}

// Reserve ensures that at least size bytes can be allocated without calling
// [Arena.Grow].
func (a *Arena) Reserve(size int) {
	if a.Next.Add(size) > a.End {
		a.Grow(size)
	}
}

// Free resets this arena to an "empty" state, allowing all memory allocated by
// it to be re-used.
//
// Although this can be used to amortize trips into Go's allocator, doing so
// trades off safety: any memory allocated by the arena must not be referenced
// after a call to Free.
func (a *Arena) Free() {
	// Discard all but the largest block, which we clear. This means that as
	// an arena is re-used, we will eventually wind up learning the size of the
	// largest block we need to allocate, and use only that one, meaning that
	// "average" calls should never have to call Grow().
	end := len(a.blocks) - 1
	clear(a.blocks[:end])
	xunsafe.Clear(a.blocks[end], 1<<end)

	// Set up next/end/cap to point to the largest block.
	a.Next = xunsafe.AddrOf(a.blocks[end])
	a.End = a.Next.Add(1 << end)
	a.Cap = 1 << end

	// Order doesn't matter here: nothing in a.blocks can point into a.keep,
	// because the only GC-visible pointers in a.blocks are pointers back to
	// a, the arena header.
	//
	// We set this to nil because clearing this will walk us right into an
	// unavoidable bulk write barrier. By writing nil, we only pay for a fast
	// single-pointer write barrier, and make cleaning up the handful of bytes
	// this throws out the GC's problem.
	//
	// In profiling, it turns out that doing clear(a.keep) is several times
	// more expensive than the noscan clear that happens below.
	a.keep = nil
}

// Grow allocates fresh memory onto next of at least the given size.
//
// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (a *Arena) Grow(size int) {
	xunsafe.Escape(a)
	p, n := a.allocChunk(max(size, a.Cap*2))
	// No need to KeepAlive(p) this pointer, since allocChunk sticks it in the
	// dedicated memory block array.

	a.Next = xunsafe.AddrOf(p)
	a.End = a.Next.Add(n)
	a.Cap = n
	a.Log("grow", "%v:%v:%d\n", a.Next, a.End, a.Cap)
}

func (a *Arena) Log(op, format string, args ...any) {
	debug.Log([]any{"%p %v:%v", a, a.Next, a.End}, op, format, args...)
}
