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

package gencode

import (
	"math"
	"unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/repeated"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// UnsafeMessage is a helper for extracting fields of specific types from a
// message type without performing any redundant safety checks.
type UnsafeMessage dynamic.Message

// Raw returns the underlying dynamic message value.
func (u *UnsafeMessage) Raw() *dynamic.Message {
	return xunsafe.Cast[dynamic.Message](u)
}

// Has returns whether this singular field is populated or not.
func (u *UnsafeMessage) Has(index int) bool {
	return u.Raw().GetByIndexUnchecked(index).IsValid()
}

// GetInt32 gets a singular int32 field.
func (u *UnsafeMessage) GetInt32(index int, ifNil int32) int32 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return int32(r)
}

// GetInt64 gets a singular int64 field.
func (u *UnsafeMessage) GetInt64(index int, ifNil int64) int64 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return int64(r)
}

// GetUint32 gets a singular uint32 field.
func (u *UnsafeMessage) GetUint32(index int, ifNil uint32) uint32 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return uint32(r)
}

// GetUint64 gets a singular uint64 field.
func (u *UnsafeMessage) GetUint64(index int, ifNil uint64) uint64 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return r
}

// GetFloat32 gets a singular float32 field.
func (u *UnsafeMessage) GetFloat32(index int, ifNil float32) float32 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return math.Float32frombits(uint32(r))
}

// GetFloat64 gets a singular float64 field.
func (u *UnsafeMessage) GetFloat64(index int, ifNil float64) float64 {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return math.Float64frombits(r)
}

// GetBool gets a singular bool field.
func (u *UnsafeMessage) GetBool(index int, ifNil bool) bool {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	r := xprotoreflect.GetRawInt(v)
	return r != 0
}

// GetEnum gets a singular enum field.
func (u *UnsafeMessage) GetEnum(index int, ifNil int32) protoreflect.EnumNumber {
	return protoreflect.EnumNumber(u.GetInt32(index, ifNil))
}

// GetString gets a singular string field.
func (u *UnsafeMessage) GetString(index int, ifNil string) string {
	if u == nil {
		return ifNil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	n := xprotoreflect.GetRawInt(v)
	return unsafe.String((*byte)(p), n)
}

// GetBytes gets a singular bytes field.
func (u *UnsafeMessage) GetBytes(index int, ifNil string) []byte {
	if u == nil {
		return []byte(ifNil)
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	n := xprotoreflect.GetRawInt(v)
	return unsafe.Slice((*byte)(p), n)
}

// GetMessage returns a singular message field.
func (u *UnsafeMessage) GetMessage(index int) unsafe.Pointer {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return p
}

// GetRepeatedInt32 gets a repeated int32 field.
func (u *UnsafeMessage) GetRepeatedInt32(index int) *repeated.Scalars[byte, int32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[byte, int32])(p)
}

// GetRepeatedInt64 gets a singular int64 field.
func (u *UnsafeMessage) GetRepeatedInt64(index int) *repeated.Scalars[byte, int64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[byte, int64])(p)
}

// GetRepeatedUint32 gets a singular uint32 field.
func (u *UnsafeMessage) GetRepeatedUint32(index int) *repeated.Scalars[byte, uint32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[byte, uint32])(p)
}

// GetRepeatedUint64 gets a singular uint64 field.
func (u *UnsafeMessage) GetRepeatedUint64(index int) *repeated.Scalars[byte, uint64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[byte, uint64])(p)
}

// GetRepeatedSfixed32 gets a repeated int32 field.
func (u *UnsafeMessage) GetRepeatedSfixed32(index int) *repeated.Scalars[int32, int32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[int32, int32])(p)
}

// GetRepeatedSfixed64 gets a singular int64 field.
func (u *UnsafeMessage) GetRepeatedSfixed64(index int) *repeated.Scalars[int64, int64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[int64, int64])(p)
}

// GetRepeatedFixed32 gets a singular uint32 field.
func (u *UnsafeMessage) GetRepeatedFixed32(index int) *repeated.Scalars[uint32, uint32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[uint32, uint32])(p)
}

// GetRepeatedFixed64 gets a singular uint64 field.
func (u *UnsafeMessage) GetRepeatedFixed64(index int) *repeated.Scalars[uint64, uint64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[uint64, uint64])(p)
}

// GetRepeatedSint32 gets a repeated sint32 field.
func (u *UnsafeMessage) GetRepeatedSint32(index int) *repeated.Zigzags[byte, int32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Zigzags[byte, int32])(p)
}

// GetRepeatedSint64 gets a singular sint64 field.
func (u *UnsafeMessage) GetRepeatedSint64(index int) *repeated.Zigzags[byte, int64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Zigzags[byte, int64])(p)
}

// GetRepeatedFloat32 gets a singular float32 field.
func (u *UnsafeMessage) GetRepeatedFloat32(index int) *repeated.Scalars[float32, float32] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[float32, float32])(p)
}

// GetRepeatedFloat64 gets a singular float64 field.
func (u *UnsafeMessage) GetRepeatedFloat64(index int) *repeated.Scalars[float64, float64] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[float64, float64])(p)
}

// GetRepeatedBool gets a singular bool field.
func (u *UnsafeMessage) GetRepeatedBool(index int) *repeated.Bools {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Bools)(p)
}

// GetRepeatedEnum gets a singular enum field.
func (u *UnsafeMessage) GetRepeatedEnum(index int) *repeated.Scalars[byte, protoreflect.EnumNumber] {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Scalars[byte, protoreflect.EnumNumber])(p)
}

// GetRepeatedString gets a singular string field.
func (u *UnsafeMessage) GetRepeatedString(index int) *repeated.Strings {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Strings)(p)
}

// GetRepeatedBytes gets a singular bytes field.
func (u *UnsafeMessage) GetRepeatedBytes(index int) *repeated.Bytes {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.Bytes)(p)
}

// GetRepeatedMessage returns a singular message field.
func (u *UnsafeMessage) GetRepeatedMessage(index int) *repeated.UntypedMessages {
	if u == nil {
		return nil
	}

	v := u.Raw().GetByIndexUnchecked(index)
	p := xprotoreflect.GetRawPointer(v)
	return (*repeated.UntypedMessages)(p)
}
