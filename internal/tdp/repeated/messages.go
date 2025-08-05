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
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Messages is a repeated message field.
//
// Repeated messages use two different layouts, and the stride is used to
// differentiate them. The messages can either be packed into an arena slice,
// or the arena slice can contain *message pointers. These are called inlined
// and outlined modes; the stride is zero in the latter case. We switch to the
// outlined mode to avoid needing to copy parsed messages on slice resize.
//
// M *must* be some type which wraps a dynamic.Message, and P must be *M.
type Messages[M any, P interface{ ProtoReflect() protoreflect.Message }] struct {
	// Slice[byte] if stride is nonzero, Slice[*Message] otherwise.
	Raw slice.Untyped
	// The array stride for when raw is an inlined message list.
	Stride uint32
}

type UntypedMessages = Messages[dynamic.Message, *dynamic.Message]

// Len returns the length of this repeated field.
func (m *Messages[_, _]) Len() int {
	if m == nil {
		return 0
	}

	if m.Stride != 0 {
		return int(m.Raw.Len) / int(m.Stride)
	}

	return int(m.Raw.Len)
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (m *Messages[M, _]) Get(n int) *M {
	if m.Stride != 0 {
		xunsafe.BoundsCheck(n, int(m.Raw.Len)/int(m.Stride))

		return xunsafe.ByteAdd[M](
			m.Raw.Ptr.AssertValid(),
			n*int(m.Stride),
		)
	}

	return slice.CastUntyped[*M](m.Raw).Raw()[n]
}

// Values returns an iterator over the elements of m.
func (m *Messages[M, _]) Values() iter.Seq[*M] {
	return func(yield func(*M) bool) {
		if m == nil {
			return
		}

		if m.Stride != 0 {
			for k := 0; k < int(m.Raw.Len); k += int(m.Stride) {
				p := xunsafe.ByteAdd[M](m.Raw.Ptr.AssertValid(), k)
				if !yield(p) {
					return
				}
			}
		}

		for _, p := range slice.CastUntyped[*M](m.Raw).Raw() {
			if !yield(p) {
				return
			}
		}
	}
}

// All returns an iterator over the indices and elements of m.
func (m *Messages[M, _]) All() iter.Seq2[int, *M] {
	return func(yield func(int, *M) bool) {
		if m == nil {
			return
		}

		if m.Stride != 0 {
			i := 0
			for k := 0; k < int(m.Raw.Len); k += int(m.Stride) {
				p := xunsafe.ByteAdd[M](m.Raw.Ptr.AssertValid(), k)
				if !yield(i, p) {
					return
				}
				i++
			}
		}

		for i, p := range slice.CastUntyped[*M](m.Raw).Raw() {
			if !yield(i, p) {
				return
			}
		}
	}
}

// Copy copies these scalars to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (m *Messages[M, _]) Copy(out []*M) []*M {
	if m == nil {
		return out
	}

	if m.Stride == 0 {
		return append(out, slice.CastUntyped[*M](m.Raw).Raw()...)
	}

	out = slices.Grow(out, m.Len())
	for v := range m.Values() {
		out = append(out, v)
	}
	return out
}

// ProtoReflect returns a reflection value for this list.
func (m *Messages[M, P]) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectMessages[M, P]](m)
}

func (*Messages[_, _]) Repeated(DoNotImplement) {}
