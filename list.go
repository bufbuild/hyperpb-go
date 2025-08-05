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

package hyperpb

import (
	"iter"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp/repeated"
)

// List is a type-generic version of [protoreflect.List], which is implemented
// by types for repeated fields returned by hyperpb gencode.
type List[E any] interface {
	// Len returns the length of the list.
	Len() int

	// Get returns the nth element of the list.
	//
	// Panics if n is negative or greater than or equal to Len().
	Get(n int) E

	// Copy copies the elements of this list to the given slice.
	Copy(to []E) []E

	// Values returns an iterator over the values of this list.
	Values() iter.Seq[E]

	// All returns an iterator over the values of this list and their indices.
	All() iter.Seq2[int, E]

	// ProtoReflect returns a protoreflect view of this list.
	ProtoReflect() protoreflect.List

	Repeated(repeated.DoNotImplement)
}
