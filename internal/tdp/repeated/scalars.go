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
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// Scalars is a repeated field containing some varint-encoded type.
//
// A Scalars can be either in zero-copy mode, in which case Raw is a slice of
// ZC, or in on-arena mode, in which case it is a slice of E. These can be
// distinct in the case of varint fields, which are only zero-copy when they
// are filled with byte-sized values.
type Scalars[ZC, E tdp.Number] struct {
	_ [0]ZC // Prevent sketchy casts.
	_ [0]E

	Raw slice.Untyped
}

// IsZC returns whether this Scalars is in zero-copy mode.
func (s *Scalars[ZC, E]) IsZC() bool {
	return s.Raw.OffArena()
}

// Len returns the length of this repeated field.
func (s *Scalars[ZC, E]) Len() int {
	if s == nil {
		return 0
	}

	return int(s.Raw.Len)
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (s *Scalars[ZC, E]) Get(n int) E {
	if s.IsZC() {
		r := slice.CastUntyped[ZC](s.Raw).Raw()
		return E(r[n])
	}

	r := slice.CastUntyped[E](s.Raw).Raw()
	return r[n]
}

// Values returns an iterator over the elements of s.
func (s *Scalars[ZC, E]) Values() iter.Seq[E] {
	return func(yield func(E) bool) {
		if s == nil {
			return
		}

		if s.IsZC() {
			r := slice.CastUntyped[ZC](s.Raw).Raw()
			for _, v := range r {
				if !yield(E(v)) {
					return
				}
			}
		} else {
			r := slice.CastUntyped[E](s.Raw).Raw()
			for _, v := range r {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// All returns an iterator over the indices and elements of s.
func (s *Scalars[ZC, E]) All() iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		if s == nil {
			return
		}

		if s.IsZC() {
			r := slice.CastUntyped[ZC](s.Raw).Raw()
			for i, v := range r {
				if !yield(i, E(v)) {
					return
				}
			}
		} else {
			r := slice.CastUntyped[E](s.Raw).Raw()
			for i, v := range r {
				if !yield(i, v) {
					return
				}
			}
		}
	}
}

// Copy copies these scalars to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (s *Scalars[ZC, E]) Copy(out []E) []E {
	if s == nil {
		return out
	}

	if layout.Size[ZC]() == layout.Size[E]() || !s.IsZC() {
		return append(out, slice.CastUntyped[E](s.Raw).Raw()...)
	}

	out = slices.Grow(out, s.Len())
	for v := range s.Values() {
		out = append(out, v)
	}
	return out
}

// ProtoReflect returns a reflection value for this list.
func (s *Scalars[ZC, E]) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectScalars[ZC, E]](s)
}

func (*Scalars[_, _]) Repeated(DoNotImplement) {}
