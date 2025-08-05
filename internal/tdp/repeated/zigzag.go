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
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zigzag"
)

// Zigzags is like [Scalars], btu the elements are zigzag-encoded.
type Zigzags[ZC, E tdp.Number] struct {
	_ [0]ZC // Prevent sketchy casts.
	_ [0]E

	Raw slice.Untyped
}

// IsZC returns whether this Scalars is in zero-copy mode.
func (z *Zigzags[ZC, E]) IsZC() bool {
	return z.Raw.OffArena()
}

// Len returns the length of this repeated field.
func (z *Zigzags[ZC, E]) Len() int {
	if z == nil {
		return 0
	}

	return int(z.Raw.Len)
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (z *Zigzags[ZC, E]) Get(n int) E {
	if z.IsZC() {
		r := slice.CastUntyped[ZC](z.Raw).Raw()
		return zigzag.Decode(E(r[n]))
	}

	r := slice.CastUntyped[E](z.Raw).Raw()
	return zigzag.Decode(r[n])
}

// Values returns an iterator over the elements of s.
func (z *Zigzags[ZC, E]) Values() iter.Seq[E] {
	return func(yield func(E) bool) {
		if z == nil {
			return
		}

		if z.IsZC() {
			r := slice.CastUntyped[ZC](z.Raw).Raw()
			for _, v := range r {
				if !yield(zigzag.Decode(E(v))) {
					return
				}
			}
		} else {
			r := slice.CastUntyped[E](z.Raw).Raw()
			for _, v := range r {
				if !yield(zigzag.Decode(v)) {
					return
				}
			}
		}
	}
}

// All returns an iterator over the indices and elements of s.
func (z *Zigzags[ZC, E]) All() iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		if z == nil {
			return
		}

		if z.IsZC() {
			r := slice.CastUntyped[ZC](z.Raw).Raw()
			for i, v := range r {
				if !yield(i, zigzag.Decode(E(v))) {
					return
				}
			}
		} else {
			r := slice.CastUntyped[E](z.Raw).Raw()
			for i, v := range r {
				if !yield(i, zigzag.Decode(v)) {
					return
				}
			}
		}
	}
}

// Copy copies these scalars to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (z *Zigzags[ZC, E]) Copy(out []E) []E {
	if z == nil {
		return out
	}

	out = slices.Grow(out, z.Len())
	for v := range z.Values() {
		out = append(out, v)
	}
	return out
}

// ProtoReflect returns a reflection value for this list.
func (s *Zigzags[ZC, E]) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectZigzags[ZC, E]](s)
}

func (*Zigzags[_, _]) Repeated(DoNotImplement) {}
