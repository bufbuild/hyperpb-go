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

// Package xprotoreflect contains helpers for working around inefficiencies in
// protoreflect.
package xprotoreflect

import (
	"fmt"
	"unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Int is any integer type that a [protoreflect.Value] will contain inline
// rather than as a pointer to an interface.
type Int interface {
	protoreflect.EnumNumber | int32 | uint32 | int64 | uint64
}

// ValueOfScalar is like protoreflect.ValueOf, but it assumes that v is stored directly
// inside of a protoreflect.Value. Unlike protoreflect.ValueOf, it will not
// cause v to escape.
func ValueOfScalar(v any) protoreflect.Value {
	switch v := v.(type) {
	case nil:
		return protoreflect.Value{}
	case bool:
		return protoreflect.ValueOfBool(v)
	case int32:
		return protoreflect.ValueOfInt32(v)
	case int64:
		return protoreflect.ValueOfInt64(v)
	case uint32:
		return protoreflect.ValueOfUint32(v)
	case uint64:
		return protoreflect.ValueOfUint64(v)
	case float32:
		return protoreflect.ValueOfFloat32(v)
	case float64:
		return protoreflect.ValueOfFloat64(v)
	case protoreflect.EnumNumber:
		return protoreflect.ValueOfEnum(v)
	default:
		panic(fmt.Sprintf("invalid type: %T", v))
	}
}

// GetInt extracts a scalar value out of a [protoreflect.Value].
//
// Panics if this is the wrong type.
func GetInt[T Int](v protoreflect.Value) T {
	var z T
	r := unwrapValue(v)
	if r.typ != xunsafe.AnyType(z) {
		panic(typeMismatch(protoreflect.ValueOf(z), v))
	}

	return T(r.num)
}

// GetString is like [protoreflect.Value.String], but it panics if the contained
// type is not a string.
func GetString(v protoreflect.Value) string {
	r := unwrapValue(v)
	if r.typ != xunsafe.AnyType("") {
		panic(typeMismatch(protoreflect.ValueOf(""), v))
	}

	return unsafe.String(r.data, r.num)
}

// GetRawInt pulls the raw integer value out of v.
func GetRawInt(v protoreflect.Value) uint64 {
	return unwrapValue(v).num
}

// GetRawPointer pulls the raw pointer value out of v.
func GetRawPointer(v protoreflect.Value) unsafe.Pointer {
	return unsafe.Pointer(unwrapValue(v).data)
}

// Is returns whether v contains a value of the given type.
func Is[T any](v protoreflect.Value) bool {
	r := unwrapValue(v)
	var z T
	return r.typ == xunsafe.AnyType(z)
}

// GetMessage extracts an message value out of a [protoreflect.Value].
// This is faster than just calling v.Interface(), since that has a massive
// type switch that performs slow sidecasts, rather than a direct downcast.
//
// See https://mcyoung.xyz/2024/12/12/go-abi/#codegen-for-interface-operations
// for more information on interface operations.
//
// Note that this does not work with gencode types, since those do not implement
// protoreflect.Message directly.
//
// Panics if this is the wrong type.
func GetMessage[T protoreflect.Message](v protoreflect.Value) T {
	r := unwrapValue(v)
	x, ok := xunsafe.MakeAny(r.typ, r.data).(T)
	if !ok {
		panic(typeMismatch(protoreflect.ValueOf(x), v))
	}
	return x
}

// List returns the value of v as a list, or an empty immutable list if it isn't
// one.
func List(v protoreflect.Value) protoreflect.List {
	r := unwrapValue(v)
	x, ok := xunsafe.MakeAny(r.typ, r.data).(protoreflect.List)
	if !ok {
		x = empty.List{}
	}
	return x
}

// Map returns the value of v as a map, or an empty immutable map if it isn't
// one.
func Map(v protoreflect.Value) protoreflect.Map {
	r := unwrapValue(v)
	x, ok := xunsafe.MakeAny(r.typ, r.data).(protoreflect.Map)
	if !ok {
		x = empty.Map{}
	}
	return x
}

// UnsafeUnwrap unwraps a [protoreflect.Value] into a raw pointer, checking
// that it has a particular type.
func UnsafeUnwrap(v protoreflect.Value, ty uintptr) unsafe.Pointer {
	r := unwrapValue(v)
	if r.typ != ty {
		return nil
	}
	return unsafe.Pointer(r.data)
}

// rawValue matches the layout of protoreflect.Value exactly.
type rawValue struct {
	// It is slightly funny that they store typ as an unsafe.Pointer, since most
	// of the runtime stores itabs as uintptrs, because all itabs live in
	// permanent memory.
	typ  uintptr
	data *byte
	num  uint64
}

// unwrapValue unwraps a protoreflect.Value so that we can access its internal
// structures.
func unwrapValue(v protoreflect.Value) rawValue {
	return xunsafe.BitCast[rawValue](v)
}

func typeMismatch(want, got protoreflect.Value) string {
	return fmt.Sprintf("type mismatch: cannot convert %v to %s",
		typeName(want), typeName(got))
}

// reflectTypeName is a copy of [protoreflect.Value.typeName].
func typeName(v protoreflect.Value) string {
	switch v.Interface().(type) {
	case nil:
		return "nil"
	case bool:
		return "bool"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case protoreflect.EnumNumber:
		return "enum"
	case protoreflect.Message:
		return "message"
	case protoreflect.List:
		return "list"
	case protoreflect.Map:
		return "map"
	default:
		return fmt.Sprintf("<unknown: %T>", v)
	}
}
