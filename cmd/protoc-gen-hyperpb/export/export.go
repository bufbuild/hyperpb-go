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

// Package export contains exported types and functions for use by gencode.
//
// Importing this package outside of the gencode voids any API compatibility
// guarantees.
package export

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/gencode"
	"buf.build/go/hyperpb/internal/tdp/repeated"
	"buf.build/go/hyperpb/internal/xunsafe"
)

type (
	DoNotImplement = gencode.DoNotImplement
	Type           = gencode.Type
	UnsafeMessage  = gencode.UnsafeMessage
)

func Reflect[P gencode.Message[M], M any](m P) protoreflect.Message { return gencode.Reflect(m) }

func CastRepeatedEnum[E ~int32](r *repeated.Scalars[byte, protoreflect.EnumNumber]) *repeated.Scalars[byte, E] {
	return xunsafe.Cast[repeated.Scalars[byte, E]](r)
}

func CastRepeatedMessage[P gencode.Message[M], M any](r *repeated.UntypedMessages) *repeated.Messages[M, P] {
	return xunsafe.Cast[repeated.Messages[M, P]](r)
}
