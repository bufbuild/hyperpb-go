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

package thunks

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
	"buf.build/go/hyperpb/internal/zigzag"
)

// Singular fields are implemented as a single field of the appropriate type.
// For integers (and floats, which we never materialize as floats in the parser)
// this is either uint32 or uint64. For bool, this is uint8.
//
// Strings and bytes are stored as a [zc].
//
// Messages, which are optional even when singular, are stored as a *message
// pointer. The pointer is nil when the field is not set.

// singularFields consists of archetypes for singular (i.e., non-optional)
// fields of each field type.
var singularFields = map[protoreflect.Kind]*compiler.Archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		Layout:  layout.Of[int32](),
		Getter:  getScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseVarint32}},
	},
	protoreflect.Uint32Kind: {
		Layout:  layout.Of[uint32](),
		Getter:  getScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseVarint32}},
	},
	protoreflect.Sint32Kind: {
		Layout:  layout.Of[int32](),
		Getter:  getScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		Layout:  layout.Of[int64](),
		Getter:  getScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseVarint64}},
	},
	protoreflect.Uint64Kind: {
		Layout:  layout.Of[uint64](),
		Getter:  getScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseVarint64}},
	},
	protoreflect.Sint64Kind: {
		Layout:  layout.Of[int64](),
		Getter:  getScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		Layout:  layout.Of[uint32](),
		Getter:  getScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		Layout:  layout.Of[int32](),
		Getter:  getScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseFixed32}},
	},
	protoreflect.FloatKind: {
		Layout:  layout.Of[float32](),
		Getter:  getFloat32,
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		Layout:  layout.Of[uint64](),
		Getter:  getScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		Layout:  layout.Of[int64](),
		Getter:  getScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseFixed64}},
	},
	protoreflect.DoubleKind: {
		Layout:  layout.Of[float64](),
		Getter:  getFloat64,
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		Layout:  layout.Of[[0]byte](),
		Bits:    1,
		Getter:  getBool,
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseBool}},
	},
	protoreflect.EnumKind: {
		Layout:  layout.Of[protoreflect.EnumNumber](),
		Getter:  getScalar[protoreflect.EnumNumber],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		Layout:  layout.Of[zc.Range](),
		Getter:  getString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseString}},
	},
	proto2StringKind: {
		Layout:  layout.Of[zc.Range](),
		Getter:  getString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseBytes}},
	},
	protoreflect.BytesKind: {
		Layout:  layout.Of[zc.Range](),
		Getter:  getBytes,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseBytes}},
	},

	// Message types.
	// A singular message is laid out as a single *message pointer.
	protoreflect.MessageKind: {
		Layout:  layout.Of[*dynamic.Message](),
		Getter:  getMessage,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseMessage}},
	},
	protoreflect.GroupKind: {
		Layout:  layout.Of[*dynamic.Message](),
		Getter:  getMessage,
		Parsers: []compiler.Parser{{Kind: protowire.StartGroupType, Thunk: parseGroup}},
	},
}

func getScalar[T tdp.Scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[T](m, getter.Offset)
	if p == nil {
		return protoreflect.Value{}
	}

	v := *p
	var zero T
	if v == zero {
		return protoreflect.Value{}
	}

	return xprotoreflect.ValueOfScalar(v)
}

func getBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	b := m.GetBit(getter.Offset.Bit)
	if !b {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBool(true)
}

// We can't use the stencil above due to negative zero: 0.0 == -0.0 according
// to float equality, but proto3 implicit presence requires that we report
// a -0.0 as present, but 0.0 as not present.
//
// This also avoids a potential equality comparison with a signaling NaN, which
// can cause all sorts of mayhem.
func getFloat32(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[uint32](m, getter.Offset)
	if p == nil {
		return protoreflect.Value{}
	}

	v := *p
	if v == 0 {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfFloat32(math.Float32frombits(v))
}

func getFloat64(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[uint64](m, getter.Offset)
	if p == nil {
		return protoreflect.Value{}
	}

	v := *p
	if v == 0 {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfFloat64(math.Float64frombits(v))
}

func getString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[zc.Range](m, getter.Offset)
	if p == nil {
		return protoreflect.Value{}
	}

	r := *p
	data := r.String(m.Shared.Src)

	if data == "" {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfString(data)
}

func getBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[zc.Range](m, getter.Offset)
	if p == nil {
		return protoreflect.Value{}
	}

	r := *p
	data := r.Bytes(m.Shared.Src)

	if len(data) == 0 {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfBytes(data)
}

func getMessage(m *dynamic.Message, ty *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[*dynamic.Message](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOfMessage(empty.NewMessage(ty))
	}

	sub := *p
	if sub == nil {
		return protoreflect.ValueOfMessage(empty.NewMessage(ty))
	}
	return protoreflect.ValueOfMessage(sub.ProtoReflect())
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
//
//hyperpb:stencil parseVarint32 parseVarint[uint32] StoreFromScratch -> StoreFromScratch32
//hyperpb:stencil parseVarint64 parseVarint[uint64] StoreFromScratch -> StoreFromScratch64
func parseVarint[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2 = vm.P1.SetScratch(p1.Varint(p2))

	var p *T
	p1, p2, p = vm.GetMutableField[T](p1, p2)
	*p = T(p2.Scratch())

	return p1, p2
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
//
//hyperpb:stencil parseZigZag32 parseZigZag[uint32] StoreFromScratch -> StoreFromScratch32
//hyperpb:stencil parseZigZag64 parseZigZag[uint64] StoreFromScratch -> StoreFromScratch64
func parseZigZag[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2 = vm.P1.SetScratch(p1.Varint(p2))
	p1, p2 = p1.SetScratch(p2, uint64(zigzag.Decode64[T](p2.Scratch())))

	var p *T
	p1, p2, p = vm.GetMutableField[T](p1, p2)
	*p = T(p2.Scratch())

	return p1, p2
}

//go:nosplit
//hyperpb:stencil parseFixed32 parseFixed[uint32]
//hyperpb:stencil parseFixed64 parseFixed[uint64]
func parseFixed[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	if p1.Len() < layout.Size[T]() {
		p1.Fail(p2, vm.ErrorTruncated)
	}
	var p *T
	p1, p2, p = vm.GetMutableField[T](p1, p2)
	*p = *xunsafe.Cast[T](p1.PtrAddr.AssertValid())
	p1 = p1.Advance(layout.Size[T]())

	return p1, p2
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseString(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var r zc.Range
	p1, p2, r = p1.UTF8(p2)
	p1, p2 = p1.SetScratch(p2, uint64(r))

	var p *zc.Range
	p1, p2, p = vm.GetMutableField[zc.Range](p1, p2)
	*p = zc.Range(p2.Scratch())

	return p1, p2
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseBytes(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var r zc.Range
	p1, p2, r = p1.Bytes(p2)
	p1, p2 = p1.SetScratch(p2, uint64(r))

	var p *zc.Range
	p1, p2, p = vm.GetMutableField[zc.Range](p1, p2)
	*p = zc.Range(p2.Scratch())

	return p1, p2
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseBool(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	p2.Message().SetBit(p2.Field().Offset.Bit, n != 0)

	return p1, p2
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseMessage(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)
	p1, p2 = p1.SetScratch(p2, uint64(n))

	var mp **dynamic.Message
	p1, p2, mp = vm.GetMutableField[*dynamic.Message](p1, p2)
	m := *mp
	if m == nil {
		p1, p2, m = vm.AllocMessage(p1, p2)
		xunsafe.StoreNoWB(mp, m)
	}

	return p1.PushMessage(p2, m)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseGroup(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var mp **dynamic.Message
	p1, p2, mp = vm.GetMutableField[*dynamic.Message](p1, p2)
	m := *mp
	if m == nil {
		p1, p2, m = vm.AllocMessage(p1, p2)
		xunsafe.StoreNoWB(mp, m)
	}

	return p1.PushGroup(p2, m)
}
