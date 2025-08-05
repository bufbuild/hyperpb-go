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

package repeated

import (
	"iter"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/arena/slice"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

// Bytes is a repeated field containing bytes.
type Bytes struct {
	_ [0][]byte // Prevent sketchy casts.

	Src *byte
	Raw slice.Slice[zc.Range]
}

// Len returns the length of this repeated field.
func (b *Bytes) Len() int {
	if b == nil {
		return 0
	}

	return b.Raw.Len()
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (b *Bytes) Get(n int) []byte {
	r := b.Raw.Raw()[n]
	return r.Bytes(b.Src)
}

// Values returns an iterator over the elements of b.
func (b *Bytes) Values() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if b == nil {
			return
		}

		for _, v := range b.Raw.Raw() {
			if !yield(v.Bytes(b.Src)) {
				return
			}
		}
	}
}

// All returns an iterator over the indices and elements of b.
func (b *Bytes) All() iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		if b == nil {
			return
		}

		for i, v := range b.Raw.Raw() {
			if !yield(i, v.Bytes(b.Src)) {
				return
			}
		}
	}
}

// Copy copies these strings to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (b *Bytes) Copy(out [][]byte) [][]byte {
	return b.CopyAlias(out, false)
}

// Copy copies these strings to a slice, appending to out.
//
// If alias is true, this will not make defensive copies of the returned strings.
//
// To get a fresh slice, pass nil to this function.
func (b *Bytes) CopyAlias(out [][]byte, alias bool) [][]byte {
	if b == nil {
		return out
	}

	if alias {
		out = slices.Grow(out, b.Len())
		for v := range b.Values() {
			out = append(out, v)
		}
		return out
	}

	var total int
	for v := range b.Values() {
		total += len(v)
	}

	// Allocate a single buffer for all of the string copies.
	buf := make([]byte, 0, total)

	out = slices.Grow(out, b.Len())
	for v := range b.Values() {
		buf = append(buf, v...)
		chunk := buf[len(buf)-len(v):]
		out = append(out, slices.Clip(chunk))
	}

	return out
}

// ProtoReflect returns a reflection value for this list.
func (b *Bytes) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectBytes](b)
}

func (*Bytes) Repeated(DoNotImplement) {}
