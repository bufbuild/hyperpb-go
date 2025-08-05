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

// Strings is a repeated field containing strings.
//
// The elements are zero-copy ranges relative to a source pointer.
type Strings struct {
	_ [0]string // Prevent sketchy casts.

	Src *byte
	Raw slice.Slice[zc.Range]
}

// Len returns the length of this repeated field.
func (s *Strings) Len() int {
	if s == nil {
		return 0
	}

	return s.Raw.Len()
}

// Get extracts a value at the given index.
//
// Panics if the index is out-of-bounds.
func (s *Strings) Get(n int) string {
	r := s.Raw.Raw()[n]
	return r.String(s.Src)
}

// Values returns an iterator over the elements of s.
func (s *Strings) Values() iter.Seq[string] {
	return func(yield func(string) bool) {
		if s == nil {
			return
		}

		for _, v := range s.Raw.Raw() {
			if !yield(v.String(s.Src)) {
				return
			}
		}
	}
}

// All returns an iterator over the indices and elements of s.
func (s *Strings) All() iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		if s == nil {
			return
		}

		for i, v := range s.Raw.Raw() {
			if !yield(i, v.String(s.Src)) {
				return
			}
		}
	}
}

// Copy copies these strings to a slice, appending to out.
//
// To get a fresh slice, pass nil to this function.
func (s *Strings) Copy(out []string) []string {
	return s.CopyAlias(out, false)
}

// Copy copies these strings to a slice, appending to out.
//
// If alias is true, this will not make defensive copies of the returned strings.
//
// To get a fresh slice, pass nil to this function.
func (s *Strings) CopyAlias(out []string, alias bool) []string {
	if s == nil {
		return out
	}

	if alias {
		out = slices.Grow(out, s.Len())
		for v := range s.Values() {
			out = append(out, v)
		}
		return out
	}

	var total int
	for v := range s.Values() {
		total += len(v)
	}

	// Allocate a single buffer for all of the string copies.
	buf := make([]byte, 0, total)

	out = slices.Grow(out, s.Len())
	for v := range s.Values() {
		buf = append(buf, v...)
		chunk := buf[len(buf)-len(v):]
		out = append(out, xunsafe.SliceToString(chunk))
	}

	return out
}

// ProtoReflect returns a reflection value for this list.
func (s *Strings) ProtoReflect() protoreflect.List {
	return xunsafe.Cast[reflectStrings](s)
}

func (*Strings) Repeated(DoNotImplement) {}
