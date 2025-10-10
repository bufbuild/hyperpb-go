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
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
)

// Optionals are implemented as one bit for presence in the hasbits array, and
// storage for the singular equivalent (see field_singular.go).
//
// Optional bool is two bits; one hasbit and one value. Optional message is
// equivalent to singular message.

var optionalFields = map[protoreflect.Kind]*compiler.Archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		Layout:  layout.Of[int32](),
		Bits:    1,
		Getter:  getOptionalScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalVarint32}},
	},
	protoreflect.Uint32Kind: {
		Layout:  layout.Of[uint32](),
		Bits:    1,
		Getter:  getOptionalScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalVarint32}},
	},
	protoreflect.Sint32Kind: {
		Layout:  layout.Of[int32](),
		Bits:    1,
		Getter:  getOptionalScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		Layout:  layout.Of[int64](),
		Bits:    1,
		Getter:  getOptionalScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalVarint64}},
	},
	protoreflect.Uint64Kind: {
		Layout:  layout.Of[uint64](),
		Bits:    1,
		Getter:  getOptionalScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalVarint64}},
	},
	protoreflect.Sint64Kind: {
		Layout:  layout.Of[int64](),
		Bits:    1,
		Getter:  getOptionalScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		Layout:  layout.Of[uint32](),
		Bits:    1,
		Getter:  getOptionalScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOptionalFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		Layout:  layout.Of[int32](),
		Bits:    1,
		Getter:  getOptionalScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOptionalFixed32}},
	},
	protoreflect.FloatKind: {
		Layout:  layout.Of[float32](),
		Bits:    1,
		Getter:  getOptionalScalar[float32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOptionalFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		Layout:  layout.Of[uint64](),
		Bits:    1,
		Getter:  getOptionalScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOptionalFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		Layout:  layout.Of[int64](),
		Bits:    1,
		Getter:  getOptionalScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOptionalFixed64}},
	},
	protoreflect.DoubleKind: {
		Layout:  layout.Of[float64](),
		Bits:    1,
		Getter:  getOptionalScalar[float64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOptionalFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		Layout:  layout.Of[struct{}](),
		Bits:    2,
		Getter:  getOptionalBool,
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalBool}},
	},
	protoreflect.EnumKind: {
		Layout:  layout.Of[protoreflect.EnumNumber](),
		Bits:    1,
		Getter:  getOptionalScalar[protoreflect.EnumNumber],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOptionalVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		Layout:  layout.Of[zc.Range](),
		Bits:    1,
		Getter:  getOptionalString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOptionalString}},
	},
	proto2StringKind: {
		Layout:  layout.Of[zc.Range](),
		Bits:    1,
		Getter:  getOptionalString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOptionalBytes}},
	},
	protoreflect.BytesKind: {
		Layout:  layout.Of[zc.Range](),
		Bits:    1,
		Getter:  getOptionalBytes,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOptionalBytes}},
	},

	// Same layout as for singular.
	protoreflect.MessageKind: singularFields[protoreflect.MessageKind],
	protoreflect.GroupKind:   singularFields[protoreflect.GroupKind],
}

func getOptionalScalar[T tdp.Scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.Value{}
	}
	v := *dynamic.GetField[T](m, getter.Offset)
	return xprotoreflect.ValueOfScalar(v)
}

func getOptionalBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBool(m.GetBit(getter.Offset.Bit + 1))
}

func getOptionalString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.Value{}
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOfString(r.String(m.Shared.Src))
}

func getOptionalBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.Value{}
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOfBytes(r.Bytes(m.Shared.Src))
}

//go:nosplit
func parseOptionalVarint32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseVarint32(p1, p2)
}

//go:nosplit
func parseOptionalVarint64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseVarint64(p1, p2)
}

//go:nosplit
func parseOptionalZigZag32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseZigZag32(p1, p2)
}

//go:nosplit
func parseOptionalZigZag64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseZigZag64(p1, p2)
}

//go:nosplit
func parseOptionalFixed32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseFixed32(p1, p2)
}

//go:nosplit
func parseOptionalFixed64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseFixed64(p1, p2)
}

//go:nosplit
func parseOptionalString(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseString(p1, p2)
}

//go:nosplit
func parseOptionalBytes(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	vm.SetBit(p1, p2)
	return parseBytes(p1, p2)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseOptionalBool(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	p1, p2 = vm.SetBit(p1, p2)
	p2.Message().SetBit(p2.Field().Offset.Bit+1, n != 0)

	return p1, p2
}
