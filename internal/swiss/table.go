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

// Package swiss provides arena-friendly Swisstable implementations.
//
// The aim of this package is not to provide a better generic map type; Go's
// map type as of 1.24 is itself a high-quality Swisstable, with access to
// compiler intrinsics we do not have. Instead, it is to provide comparable
// performance without requiring the use of Go's maps, which require hitting the
// Go heap instead of using our arenas.
package swiss

import (
	"bytes"
	"fmt"
	"iter"
	"math"
	"math/bits"
	"math/rand/v2"
	"strings"
	"testing"
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/stats"
	"buf.build/go/hyperpb/internal/xunsafe"
)

//go:generate go tool hyperstencil

const (
	minEntires = ctrlSize * 7 / 8
	maxEntries = math.MaxInt32 / 8
)

// Key is one of the allowed keys for [Table].
type Key interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint |
		~uintptr
}

// Table is a swisstable.
type Table[K Key, V any] struct {
	_ [0]ctrl // Ensure the end of the struct is ctrl-aligned.

	// There is a soft and a hard cap. The soft cap is how many elements can be
	// inserted before the table needs to be rehashed, while hard is the actual
	// allocated limit.
	len, soft, hard uint32

	// Instrumentation stats.
	metrics *Metrics

	// Scratch memory for holding a pointer that is needed for decoding keys.
	Scratch *byte

	// We can't use the address of a table as the seed, because the compiler
	// wants to be able to copy tables byte-wise in memory.
	seed hash

	// There is an extra clone of the first control word at the end, which
	// is used to ensure we can always load a full control word at any byte
	// offset within the control word array.
	// ctrl   [hard/ctrlSize + 1]ctrl
	// keys   [hard]K
	// values [hard]V
}

// Layout calculates the layout for a table with the given types.
//
// len is the desired number of elements.
func Layout[K Key, V any](len int) (size, align int) {
	if len > maxEntries {
		panic(fmt.Sprintf("tdp/internal/table: cannot create table of length %d; max is %d", len, maxEntries))
	}

	var t Table[K, V]
	var k K
	var v V

	_, cap := loadFactor(len)

	size = int(unsafe.Sizeof(t))
	size += int(cap) + ctrlSize // Control bytes.
	size += int(unsafe.Sizeof(k)) * int(cap)

	if diff := unsafe.Alignof(v) - unsafe.Alignof(k); diff > 0 {
		size += int(diff)
	}
	size += int(unsafe.Sizeof(v)) * int(cap)

	size = (size + 7) &^ 7 // Round up to a multiple of 8.
	return size, 8
}

// Init initializes data allocated per [Layout]'s requirements, optionally
// copying values out of the given table. This function assumes that from is
// smaller. extract is an optional function for extracting variable-length key
// material from keys in the table.
//
// data is assumed to point zeroed memory.
func (t *Table[K, V]) Init(len int, from *Table[K, V], extract func(K) []byte) *Table[K, V] {
	t.soft, t.hard = loadFactor(len)
	if debug.Enabled {
		t.log("resize", "newLen: %d:%d:%d, from: %s", len, t.soft, t.hard, from.Dump())
		defer func() {
			t.log("resized", "%s", t.Dump())
		}()
	}

	t.seed = hash(rand.Uint64())
	// empty is chosen to be zero so that we do not need to initialize the
	// control bytes.

	if from == nil || from.len == 0 {
		return t
	}

	ctrl1 := from.ctrl()
	keys1 := from.keys()
	vals1 := from.values()

	ctrl2 := xunsafe.Cast[xunsafe.VLA[byte]](t.ctrl())
	keys2 := t.keys()
	vals2 := t.values()

	// The if is hoisted to avoid an extra branch comparison inside of the hot
	// loop.
	if extract == nil {
		for i := 0; ; i++ {
			debug.Assert(i < int(t.hard)/ctrlSize, "infinite loop during copy")

			ctrl := *ctrl1.Get(i)
			for j := range ctrlSize {
				var ok bool
				ctrl, ok = ctrl.next()
				if !ok {
					continue
				}

				n := i*ctrlSize + j
				k := *keys1.Get(n)
				h := t.seed.u64(zext(k))
				idx, occupied := t.search(h, k)
				debug.Assert(!occupied, "fwo keys mapped to one slot")

				mirrored := t.mirrorIndex(idx)
				*ctrl2.Get(idx) = h.h2()
				*ctrl2.Get(mirrored) = h.h2()
				*keys2.Get(idx) = *keys1.Get(n)
				*vals2.Get(idx) = *vals1.Get(n)
				t.len++

				if t.len == from.len {
					return t
				}
			}
		}
	} else {
		for i := 0; ; i++ {
			debug.Assert(i < int(t.hard)/ctrlSize, "infinite loop during copy")

			ctrl := *ctrl1.Get(i)
			for j := range 8 {
				var ok bool
				ctrl, ok = ctrl.next()
				if !ok {
					continue
				}

				n := i*ctrlSize + j
				k := extract(*keys1.Get(n))
				h := t.seed.bytes(k)
				idx, occupied := t.searchFunc(h, k, extract)
				debug.Assert(!occupied, "fwo keys mapped to one slot")

				mirrored := t.mirrorIndex(idx)
				*ctrl2.Get(idx) = h.h2()
				*ctrl2.Get(mirrored) = h.h2()
				*keys2.Get(idx) = *keys1.Get(n)
				*vals2.Get(idx) = *vals1.Get(n)
				t.len++

				if t.len == from.len {
					return t
				}
			}
		}
	}
}

// Len returns this table's length.
func (t *Table[K, V]) Len() int {
	return int(t.len)
}

// Record sets a metrics object to record events to.
//
// This function may not be called concurrently with any table operations.
func (t *Table[K, V]) Record(m *Metrics) {
	t.metrics = m
}

// Lookup looks up the given key and returns it, or nil if no such key.
func (t *Table[K, V]) Lookup(k K) *V {
	h := t.seed.u64(zext(k))
	idx, occupied := t.search(h, k)
	if !occupied {
		return nil
	}
	return t.values().Get(idx)
}

// LookupFunc us like Lookup, but it takes an optional function for extracting
// variable-length key material from keys in the table.
func (t *Table[K, V]) LookupFunc(k []byte, extract func(K) []byte) *V {
	h := t.seed.bytes(k)
	idx, occupied := t.searchFunc(h, k, extract)
	if !occupied {
		return nil
	}
	return t.values().Get(idx)
}

// Insert returns a pointer to the slot for the given key. extract is an
// optional function for extracting variable-length key material from keys
// in the table.
//
// Returns nil if the table would grow too large.
func (t *Table[K, V]) Insert(k K, extract func(K) []byte) *V {
	if t.len == t.soft {
		return nil // Tell the caller to reallocate.
	}

	var idx int
	var occupied bool
	var h hash
	if extract == nil {
		h = t.seed.u64(zext(k))
		idx, occupied = t.search(h, k)
	} else {
		k := extract(k)
		h = t.seed.bytes(k)
		idx, occupied = t.searchFunc(h, k, extract)
	}

	ctrl := xunsafe.Beyond[ctrl](t)
	last := ctrl.Get(int(t.hard) / ctrlSize)
	keys := xunsafe.Beyond[K](last)
	last2 := keys.Get(int(t.hard) - 1)
	values := xunsafe.Beyond[V](last2)

	if !occupied {
		mirrored := t.mirrorIndex(idx)
		*xunsafe.Cast[xunsafe.VLA[byte]](ctrl).Get(idx) = h.h2()
		*xunsafe.Cast[xunsafe.VLA[byte]](ctrl).Get(mirrored) = h.h2()
		*keys.Get(idx) = k
		t.len++
	}
	return values.Get(idx)
}

func (t *Table[K, V]) mirrorIndex(idx int) int {
	mask := int(t.hard - 1)
	cloned := ctrlSize - 1
	return ((idx - cloned) & mask) + (cloned & mask)
}

// All ranges over a table.
func (t *Table[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.len == 0 {
			return
		}

		ctrl := t.ctrl()
		keys := t.keys()
		vals := t.values()

		len := t.len
		for i := range int(t.hard) / ctrlSize {
			ctrl := *ctrl.Get(i)
			for j := range 8 {
				var ok bool
				ctrl, ok = ctrl.next()
				if !ok {
					continue
				}

				k := i*ctrlSize + j
				len--
				if !yield(*keys.Get(k), *vals.Get(k)) || len == 0 {
					return
				}
			}
		}
	}
}

// Metrics contains instrumentation statistics about a table.
type Metrics struct {
	// The average length of a probe sequence.
	Probes stats.Mean
}

// Reset resets all the metrics back to zero.
func (m *Metrics) Reset() {
	*m = Metrics{}
}

// Report reports metrics to a benchmark.
func (m *Metrics) Report(b *testing.B) {
	b.Helper()
	b.ReportMetric(m.Probes.Get(), "probes/seq")
}

// Merge merges each metric in that into m.
func (m *Metrics) Merge(that *Metrics) {
	m.Probes.Merge(&that.Probes)
}

// search searches for a key's bucket: either an occupied slot, or an empty
// slot where it could be inserted at.
//
// Returns the index of the bucket and whether it is already occupied.
func (t *Table[K, V]) search(h hash, k K) (idx int, occupied bool) {
	t.log("search", "h: %v, k: %v", h, k)

	h2 := broadcast(h.h2())
	empty := broadcast(empty)

	p := newProber(xunsafe.Beyond[ctrl](t), int(t.hard), h)
	ctrls := xunsafe.Beyond[ctrl](t)
	last := ctrls.Get(int(t.hard) / ctrlSize)
	keys := xunsafe.Beyond[K](last)
	len := 0
	for {
		// Guaranteed to terminate because there's always going to be an open
		// spot to insert. We include a debug assert for catching when this
		// fails to happen.
		debug.Assert(p.i <= p.mask, "full table: %#v", p)
		len++

		var i int
		var ctrl ctrl
		p, i, ctrl = p.next()

		// First, check for any hits.
		mask := ctrl.matches(h2)
		t.log("matching", "i: %v, ctrl: %v, h2: %v, mask: %v", i, ctrl, h2, mask)
		if mask.nonempty() {
			n := i
			for j := range ctrlSize {
				var eq bool
				mask, eq = mask.next()
				if eq {
					n &= int(t.hard) - 1
					k2 := *keys.Get(n)
					t.log("checking", "%v == %v", k, k2)
					if k == k2 {
						t.log("found occupied", "%v,%v = %v", i, j, n)
						t.recordProbeSeq(len)
						return n, true
					}
				}
				n++
			}
		}

		// Otherwise, check for empties.
		j := ctrl.first(empty)
		if j < ctrlSize {
			n := i + j
			t.log("found vacant", "%v,%v = %v", i, j, n)
			t.recordProbeSeq(len)
			return n & (int(t.hard) - 1), false
		}
	}
}

// searchFunc is like search, but takes a function for extracting a variable-length key.
func (t *Table[K, V]) searchFunc(h hash, k []byte, extract func(K) []byte) (idx int, occupied bool) {
	t.log("search", "h: %v, k: %v", h, k)

	h2 := broadcast(h.h2())
	empty := broadcast(empty)

	p := newProber(xunsafe.Beyond[ctrl](t), int(t.hard), h)
	keys := t.keys()
	len := 0
	for {
		// Guaranteed to terminate because there's always going to be an open
		// spot to insert. We include a debug assert for catching when this
		// fails to happen.
		debug.Assert(p.i <= p.mask, "full table: %#v", p)
		len++

		var i int
		var ctrl ctrl
		p, i, ctrl = p.next()

		// First, check for any hits.
		mask := ctrl.matches(h2)
		t.log("matching", "i: %v, ctrl: %v, h2: %v, mask: %v", i, ctrl, h2, mask)
		if mask.nonempty() {
			n := i
			for j := range ctrlSize {
				var eq bool
				mask, eq = mask.next()
				if eq {
					n &= int(t.hard) - 1
					k2 := extract(*keys.Get(n))
					t.log("checking", "%x == %x", k, k2)
					if bytes.Equal(k, k2) {
						t.log("found occupied", "%v,%v = %v", i, j, n)
						t.recordProbeSeq(len)
						return n, true
					}
				}
				n++
			}
		}

		// Otherwise, check for empties.
		j := ctrl.first(empty)
		if j < ctrlSize {
			n := i + j
			t.log("found vacant", "%v,%v = %v", i, j, n)
			t.recordProbeSeq(len)
			return n & (int(t.hard) - 1), false
		}
	}
}

// XXX: Go bizarrely does not inline the below functions, so they are manually
// inlined in some places above.

func (t *Table[K, V]) ctrl() *xunsafe.VLA[ctrl] {
	return xunsafe.Beyond[ctrl](t)
}

func (t *Table[K, V]) keys() *xunsafe.VLA[K] {
	ctrl := xunsafe.Beyond[ctrl](t)
	last := ctrl.Get(int(t.hard) / ctrlSize)
	return xunsafe.Beyond[K](last)
}

func (t *Table[K, V]) values() *xunsafe.VLA[V] {
	ctrl := xunsafe.Beyond[ctrl](t)
	last := ctrl.Get(int(t.hard) / ctrlSize)
	keys := xunsafe.Beyond[K](last)
	last2 := keys.Get(int(t.hard) - 1)
	return xunsafe.Beyond[V](last2)
}

func (t *Table[K, V]) log(op, format string, args ...any) {
	debug.Log([]any{"%p", t}, op, format, args...)
}

func (t *Table[K, V]) recordProbeSeq(len int) {
	if t.metrics != nil {
		t.metrics.Probes.Record(float64(len))
	}
}

// loadFactor calculates the capacity of a table with n elements, implementing
// a load factor of 7/8.
//
// The returned value is always a power of two divisible by 8.
func loadFactor(len int) (soft, hard uint32) {
	// Go generates better code for unsigned arithmetic here.
	e := uint(max(minEntires, len))
	n := e * 8 / 7
	// Make sure that n is a power of two. Pick the next power of
	// two after n.
	if bits.OnesCount(n) != 1 {
		n = uint(1) << bits.Len(n)
	}
	return uint32(n / 8 * 7), uint32(n)
}

// Format implements [fmt.Formatter].
func (t *Table[K, V]) Format(s fmt.State, verb rune) {
	kv := "%v/%#x: " + fmt.FormatString(s, verb)
	first := true

	fmt.Fprint(s, "[")
	for k, v := range t.All() {
		if !first {
			fmt.Fprint(s, ", ")
		}
		first = false
		fmt.Fprintf(s, kv, k, k, v)
	}
	fmt.Fprint(s, "]")
}

// Dump returns an in-memory dump of t.
func (t *Table[K, V]) Dump() string {
	var k K
	var v V

	if t == nil {
		return fmt.Sprintf("0x0:0x0: Table[%T, %T]\n", k, v)
	}

	buf := new(strings.Builder)
	size, align := Layout[K, V](int(t.len))
	end := xunsafe.Addr[byte](xunsafe.AddrOf(t)).Add(size)
	fmt.Fprintf(buf, "%p:%v: Table[%T, %T]\n", t, end, k, v)
	fmt.Fprintf(buf, "len: %v, cap: %v/%v, load factor: %v\n", t.len, t.soft, t.hard, float64(t.len)/float64(t.hard))
	fmt.Fprintf(buf, "seed: %016x, layout: [%d:%d]\n", uint64(t.seed), size, align)

	fmt.Fprintf(buf, "ctrl:")
	ctrl := xunsafe.Cast[xunsafe.VLA[byte]](t.ctrl())
	for i := range int(t.hard) + ctrlSize {
		if i%ctrlSize == 0 {
			fmt.Fprintf(buf, "\n  %p:%04d", ctrl.Get(i), i)
		}
		fmt.Fprintf(buf, " %02x", *ctrl.Get(i))
	}
	fmt.Fprintln(buf, " (mirrored)")

	fmt.Fprintf(buf, "keys:")
	keys := t.keys()
	perLine := max(1, int(16/unsafe.Sizeof(k)))
	for i := range int(t.hard) {
		if i%perLine == 0 {
			fmt.Fprintf(buf, "\n  %p/%04d:", keys.Get(i), i)
		}
		fmt.Fprintf(buf, " %x", xunsafe.AnyBytes(*keys.Get(i)))
	}
	fmt.Fprintln(buf)

	fmt.Fprintf(buf, "values:")
	values := t.values()
	perLine = max(1, int(16/unsafe.Sizeof(v)))
	for i := range int(t.hard) {
		if i%perLine == 0 {
			fmt.Fprintf(buf, "\n  %p/%04d:", values.Get(i), i)
		}
		fmt.Fprintf(buf, " %x", xunsafe.AnyBytes(*values.Get(i)))
	}
	fmt.Fprintln(buf)

	return buf.String()
}

//hyperpb:stencil InitU8xU8 Table.Init[uint8, uint8] search -> searchU8xU8 searchFunc -> searchFuncU8xU8
//hyperpb:stencil InitU32xU8 Table.Init[uint32, uint8] search -> searchU32xU8 searchFunc -> searchFuncU32xU8
//hyperpb:stencil InitU64xU8 Table.Init[uint64, uint8] search -> searchU64xU8 searchFunc -> searchFuncU64xU8
//hyperpb:stencil InitU8xU32 Table.Init[uint8, uint32] search -> searchU8xU32 searchFunc -> searchFuncU8xU32
//hyperpb:stencil InitU32xU32 Table.Init[uint32, uint32] search -> searchU32xU32 searchFunc -> searchFuncU32xU32
//hyperpb:stencil InitU64xU32 Table.Init[uint64, uint32] search -> searchU64xU32 searchFunc -> searchFuncU64xU32
//hyperpb:stencil InitU8xU64 Table.Init[uint8, uint64] search -> searchU8xU64 searchFunc -> searchFuncU8xU64
//hyperpb:stencil InitU32xU64 Table.Init[uint32, uint64] search -> searchU32xU64 searchFunc -> searchFuncU32xU64
//hyperpb:stencil InitU64xU64 Table.Init[uint64, uint64] search -> searchU64xU64 searchFunc -> searchFuncU64xU64
//hyperpb:stencil InitU8xP Table.Init[uint8, unsafe.Pointer] search -> searchU8xP searchFunc -> searchFuncU8xP
//hyperpb:stencil InitU32xP Table.Init[uint32, unsafe.Pointer] search -> searchU32xP searchFunc -> searchFuncU32xP
//hyperpb:stencil InitU64xP Table.Init[uint64, unsafe.Pointer] search -> searchU64xP searchFunc -> searchFuncU64xP

//hyperpb:stencil InsertU8xU8 Table.Insert[uint8, uint8] search -> searchU8xU8 searchFunc -> searchFuncU8xU8
//hyperpb:stencil InsertU32xU8 Table.Insert[uint32, uint8] search -> searchU32xU8 searchFunc -> searchFuncU32xU8
//hyperpb:stencil InsertU64xU8 Table.Insert[uint64, uint8] search -> searchU64xU8 searchFunc -> searchFuncU64xU8
//hyperpb:stencil InsertU8xU32 Table.Insert[uint8, uint32] search -> searchU8xU32 searchFunc -> searchFuncU8xU32
//hyperpb:stencil InsertU32xU32 Table.Insert[uint32, uint32] search -> searchU32xU32 searchFunc -> searchFuncU32xU32
//hyperpb:stencil InsertU64xU32 Table.Insert[uint64, uint32] search -> searchU64xU32 searchFunc -> searchFuncU64xU32
//hyperpb:stencil InsertU8xU64 Table.Insert[uint8, uint64] search -> searchU8xU64 searchFunc -> searchFuncU8xU64
//hyperpb:stencil InsertU32xU64 Table.Insert[uint32, uint64] search -> searchU32xU64 searchFunc -> searchFuncU32xU64
//hyperpb:stencil InsertU64xU64 Table.Insert[uint64, uint64] search -> searchU64xU64 searchFunc -> searchFuncU64xU64
//hyperpb:stencil InsertU8xP Table.Insert[uint8, unsafe.Pointer] search -> searchU8xP searchFunc -> searchFuncU8xP
//hyperpb:stencil InsertU32xP Table.Insert[uint32, unsafe.Pointer] search -> searchU32xP searchFunc -> searchFuncU32xP
//hyperpb:stencil InsertU64xP Table.Insert[uint64, unsafe.Pointer] search -> searchU64xP searchFunc -> searchFuncU64xP

//hyperpb:stencil searchU8xU8 Table.search[uint8, uint8]
//hyperpb:stencil searchU32xU8 Table.search[uint32, uint8]
//hyperpb:stencil searchU64xU8 Table.search[uint64, uint8]
//hyperpb:stencil searchU8xU32 Table.search[uint8, uint32]
//hyperpb:stencil searchU32xU32 Table.search[uint32, uint32]
//hyperpb:stencil searchU64xU32 Table.search[uint64, uint32]
//hyperpb:stencil searchU8xU64 Table.search[uint8, uint64]
//hyperpb:stencil searchU32xU64 Table.search[uint32, uint64]
//hyperpb:stencil searchU64xU64 Table.search[uint64, uint64]
//hyperpb:stencil searchU8xP Table.search[uint8, unsafe.Pointer]
//hyperpb:stencil searchU32xP Table.search[uint32, unsafe.Pointer]
//hyperpb:stencil searchU64xP Table.search[uint64, unsafe.Pointer]

//hyperpb:stencil searchFuncU8xU8 Table.searchFunc[uint8, uint8]
//hyperpb:stencil searchFuncU32xU8 Table.searchFunc[uint32, uint8]
//hyperpb:stencil searchFuncU64xU8 Table.searchFunc[uint64, uint8]
//hyperpb:stencil searchFuncU8xU32 Table.searchFunc[uint8, uint32]
//hyperpb:stencil searchFuncU32xU32 Table.searchFunc[uint32, uint32]
//hyperpb:stencil searchFuncU64xU32 Table.searchFunc[uint64, uint32]
//hyperpb:stencil searchFuncU8xU64 Table.searchFunc[uint8, uint64]
//hyperpb:stencil searchFuncU32xU64 Table.searchFunc[uint32, uint64]
//hyperpb:stencil searchFuncU64xU64 Table.searchFunc[uint64, uint64]
//hyperpb:stencil searchFuncU8xP Table.searchFunc[uint8, unsafe.Pointer]
//hyperpb:stencil searchFuncU32xP Table.searchFunc[uint32, unsafe.Pointer]
//hyperpb:stencil searchFuncU64xP Table.searchFunc[uint64, unsafe.Pointer]

//hyperpb:stencil LookupI32xU32 Table.Lookup[int32, uint32] search -> searchI32xU32
//hyperpb:stencil LookupU32xU32 Table.Lookup[uint32, uint32] search -> searchU32xU32
//hyperpb:stencil LookupFuncU32xU32 Table.LookupFunc[uint32, uint32] searchFunc -> searchFuncU32xU32
//hyperpb:stencil LookupU64xU32 Table.Lookup[uint64, uint32] search -> searchU64xU32
//hyperpb:stencil searchI32xU32 Table.search[int32, uint32]
