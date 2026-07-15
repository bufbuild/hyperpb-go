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

package linker

import (
	"reflect"
	"unsafe"

	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

const (
	Unknown Kind = iota
	Address      // Target-specific pointer in the final output buffer.
	Abs32        // Absolute (relative to the program base) 32-bit offset.
)

// Kind is a relocation kind.
type Kind byte

// Sym is all of the metadata associated with a symbol in [Linker].
type Sym struct {
	name  any
	align int
	data  []byte
	rels  []Rel

	offset int // Assigned during Link().
}

// Rel is a relocation within a [Symbol].
type Rel struct {
	Symbol any     // The name of the symbol this relocation references.
	Offset uintptr // Offset of the relocation within [Sym.data].

	Kind Kind
}

// At returns a mutable reference to the data in the given range.
//
// Pushing more data to this symbol may cause this to become invalidated.
func (s *Sym) At(i, j int) []byte {
	return s.data[i:j]
}

// Push appends a value to this symbol, ensuring that it is correctly-aligned.
//
// Returns the offset of the pushed data.
func (s *Sym) Push(v any) int {
	align := reflect.TypeOf(v).Align()
	return s.PushBytes(align, xunsafe.AnyBytes(v))
}

// PushBytes appends raw bytes to this symbol, ensuring that it is correctly-aligned.
//
// Returns the offset of the pushed data.
func (s *Sym) PushBytes(align int, data []byte) int {
	s.align = max(s.align, align)
	s.data = layout.PadSlice(s.data, align)
	offset := len(s.data)
	s.data = append(s.data, data...)
	return offset
}

// Reserve reserves a region of the given layout in this symbol and returns it.
func (s *Sym) Reserve(size, align int) []byte {
	s.align = max(s.align, align)
	s.data = layout.PadSlice(s.data, align)

	offset := len(s.data)
	s.data = append(s.data, make([]byte, size)...)
	return s.data[offset:]
}

// PushTable pushes a swiss.Table onto a symbol.
func PushTable[K swiss.Key, V comparable](s *Sym, entries ...swiss.Entry[K, V]) {
	buf := s.Reserve(swiss.Layout[K, V](len(entries)))

	table := xunsafe.Cast[swiss.Table[K, V]](unsafe.SliceData(buf))
	table.Init(len(entries), nil, nil)

	for _, e := range entries {
		*table.Insert(e.Key, nil) = e.Value
	}
}

// Rel appends relocations to this symbol.
//
// Relocations are all relative to the end of the data written to this
// symbol so far.
func (s *Sym) Rel(rels ...Rel) {
	for i := range rels {
		rels[i].Offset += uintptr(len(s.data))
	}

	s.rels = append(s.rels, rels...)
}
