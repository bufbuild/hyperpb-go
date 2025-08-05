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
)

// Bools is a repeated field containing bools.
type Bools struct {
	_ [0]bool // Prevent sketchy casts.

	Raw slice.Untyped
}

// Len returns the length of this repeated field.
func (b *Bools) Len() int {
	if b == nil {
		return 0
	}

	return int(b.Raw.Len)
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (b *Bools) Get(n int) bool {
	r := slice.CastUntyped[byte](b.Raw).Raw()[n]
	return r != 0
}

// Values returns an iterator over the elements of s.
func (b *Bools) Values() iter.Seq[bool] {
	return func(yield func(bool) bool) {
		if b == nil {
			return
		}

		for _, v := range slice.CastUntyped[byte](b.Raw).Raw() {
			if !yield(v != 0) {
				return
			}
		}
	}
}

// All returns an iterator over the indices and elements of s.
func (b *Bools) All() iter.Seq2[int, bool] {
	return func(yield func(int, bool) bool) {
		if b == nil {
			return
		}

		for i, v := range slice.CastUntyped[byte](b.Raw).Raw() {
			if !yield(i, v != 0) {
				return
			}
		}
	}
}

// Copy copies these bools to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (b *Bools) Copy(out []bool) []bool {
	if b == nil {
		return out
	}

	out = slices.Grow(out, b.Len())
	for v := range b.Values() {
		out = append(out, v)
	}
	return out
}

// ProtoReflect returns a reflection value for this list.
func (b *Bools) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectBools](b)
}

func (*Bools) Repeated(DoNotImplement) {}
