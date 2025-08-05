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

// Package gencode contains helpers used by protoc-gen-hyperpb's gencode.
package gencode

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Reflect returns a type reflection value for M.
func Reflect[P Message[M], M any](m P) protoreflect.Message {
	return xunsafe.BitCast[*reflectMessage[P, M]](m)
}

// reflectMessage is a wrapper that implements [protoreflect.Message] for M.
type reflectMessage[P Message[M], M any] struct{ message }

func (r *reflectMessage[P, M]) Interface() proto.Message {
	//nolint:errcheck
	return xunsafe.BitCast[P](r.message.Interface().(*hyperpb.Message))
}

func (r *reflectMessage[P, M]) Type() protoreflect.MessageType {
	return xunsafe.Cast[reflectType[P, M]](r.HyperType())
}

func (r *reflectMessage[P, M]) New() protoreflect.Message {
	return xunsafe.Cast[reflectMessage[P, M]](hyperpb.NewMessage(r.HyperType()))
}

// reflectType is a wrapper that implements [protoreflect.MessageType] for M.
type reflectType[P Message[M], M any] struct{ typ }

func (t *reflectType[P, M]) HyperType() *hyperpb.MessageType {
	return &t.typ
}

func (t *reflectType[P, M]) New() protoreflect.Message {
	return xunsafe.Cast[reflectMessage[P, M]](hyperpb.NewMessage(t.HyperType()))
}

func (t *reflectType[P, M]) Zero() protoreflect.Message {
	return (*reflectMessage[P, M])(nil)
}

type (
	message = hyperpb.Message
	typ     = hyperpb.MessageType
)
