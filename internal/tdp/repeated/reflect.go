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
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// reflectScalars wraps a repeated.Scalars so that it implements protoreflect.List.
type reflectScalars[ZC, E tdp.Number] struct {
	empty.List
	raw Scalars[ZC, E]
}

// IsValid implements [protoreflect.List].
func (r *reflectScalars[_, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectScalars[_, _]) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectScalars[Z, E]) Get(n int) protoreflect.Value {
	return xprotoreflect.ValueOfScalar(r.raw.Get(n))
}

// reflectZigzags wraps a repeated.Zigzags so that it implements protoreflect.List.
type reflectZigzags[ZC, E tdp.Number] struct {
	empty.List
	raw Zigzags[ZC, E]
}

// IsValid implements [protoreflect.List].
func (r *reflectZigzags[_, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectZigzags[_, _]) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectZigzags[Z, E]) Get(n int) protoreflect.Value {
	return xprotoreflect.ValueOfScalar(r.raw.Get(n))
}

// reflectBools wraps a repeated.Bools so that it implements protoreflect.List.
type reflectBools struct {
	empty.List
	raw Bools
}

// IsValid implements [protoreflect.List].
func (r *reflectBools) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectBools) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectBools) Get(n int) protoreflect.Value {
	return protoreflect.ValueOfBool(r.raw.Get(n))
}

// reflectStrings wraps a repeated.Strings so that it implements protoreflect.List.
type reflectStrings struct {
	empty.List
	raw Strings
}

// IsValid implements [protoreflect.List].
func (r *reflectStrings) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectStrings) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectStrings) Get(n int) protoreflect.Value {
	return protoreflect.ValueOfString(r.raw.Get(n))
}

// reflectBytes wraps a repeated.Bytes so that it implements protoreflect.List.
type reflectBytes struct {
	empty.List
	raw Bytes
}

// IsValid implements [protoreflect.List].
func (r *reflectBytes) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectBytes) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectBytes) Get(n int) protoreflect.Value {
	return protoreflect.ValueOfBytes(r.raw.Get(n))
}

// reflectMessages wraps a repeated.Bytes so that it implements protoreflect.List.
type reflectMessages[M any, P interface{ ProtoReflect() protoreflect.Message }] struct {
	empty.List
	raw Messages[M, P]
}

// IsValid implements [protoreflect.List].
func (r *reflectMessages[M, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *reflectMessages[M, _]) Len() int {
	return r.raw.Len()
}

// Get implements [protoreflect.List].
func (r *reflectMessages[M, P]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOfMessage(xunsafe.BitCast[P](r.raw.Get(n)).ProtoReflect())
}
