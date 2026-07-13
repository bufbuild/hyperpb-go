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
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
)

// Oneofs are implemented as an actual union. The tag (or "which word") lives
// after the hasbits array in the message; each oneof's field offset has the
// offset for its which word baked into it. Only oneofs with at least two
// variants are turned into a tagged union; otherwise they use an optional
// field archetype.
//
// A oneof variant is active when its which word contains the oneof's number.
// Note that we never give out a pointer to a oneof's union storage, which
// avoids potential temporal memory safety issues with concurrent mutation.
//
// Aliasing a region between integer and pointer memory is safe because we only
// ever place arena pointers into that memory, which do not need to be scanned
// by the garbage collector.
//
// Bool-valued oneof members are stored as a uint8, not a bool or an element of
// a bitset or anything like that.

var oneofFields = map[protoreflect.Kind]*compiler.Archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		Layout:  layout.Of[int32](),
		Oneof:   true,
		Getter:  getOneofScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofVarint32}},
	},
	protoreflect.Uint32Kind: {
		Layout:  layout.Of[uint32](),
		Oneof:   true,
		Getter:  getOneofScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofVarint32}},
	},
	protoreflect.Sint32Kind: {
		Layout:  layout.Of[int32](),
		Oneof:   true,
		Getter:  getOneofScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		Layout:  layout.Of[int64](),
		Oneof:   true,
		Getter:  getOneofScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofVarint64}},
	},
	protoreflect.Uint64Kind: {
		Layout:  layout.Of[uint64](),
		Oneof:   true,
		Getter:  getOneofScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofVarint64}},
	},
	protoreflect.Sint64Kind: {
		Layout:  layout.Of[int64](),
		Oneof:   true,
		Getter:  getOneofScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		Layout:  layout.Of[uint32](),
		Oneof:   true,
		Getter:  getOneofScalar[uint32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOneofFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		Layout:  layout.Of[int32](),
		Oneof:   true,
		Getter:  getOneofScalar[int32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOneofFixed32}},
	},
	protoreflect.FloatKind: {
		Layout:  layout.Of[float32](),
		Oneof:   true,
		Getter:  getOneofScalar[float32],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed32Type, Thunk: parseOneofFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		Layout:  layout.Of[uint64](),
		Oneof:   true,
		Getter:  getOneofScalar[uint64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOneofFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		Layout:  layout.Of[int64](),
		Oneof:   true,
		Getter:  getOneofScalar[int64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOneofFixed64}},
	},
	protoreflect.DoubleKind: {
		Layout:  layout.Of[float64](),
		Oneof:   true,
		Getter:  getOneofScalar[float64],
		Parsers: []compiler.Parser{{Kind: protowire.Fixed64Type, Thunk: parseOneofFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		Layout:  layout.Of[byte](),
		Oneof:   true,
		Getter:  getOneofBool,
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofBool}},
	},
	protoreflect.EnumKind: {
		Layout:  layout.Of[protoreflect.EnumNumber](),
		Oneof:   true,
		Getter:  getOneofScalar[protoreflect.EnumNumber],
		Parsers: []compiler.Parser{{Kind: protowire.VarintType, Thunk: parseOneofVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		Layout:  layout.Of[zc.Range](),
		Oneof:   true,
		Getter:  getOneofString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOneofString}},
	},
	proto2StringKind: {
		Layout:  layout.Of[zc.Range](),
		Oneof:   true,
		Getter:  getOneofString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOneofBytes}},
	},
	protoreflect.BytesKind: {
		Layout:  layout.Of[zc.Range](),
		Oneof:   true,
		Getter:  getOneofBytes,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOneofBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		Layout:  layout.Of[*dynamic.Message](),
		Oneof:   true,
		Getter:  getOneofMessage,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Thunk: parseOneofMessage}},
	},
	protoreflect.GroupKind: {
		Layout:  layout.Of[*dynamic.Message](),
		Oneof:   true,
		Getter:  getOneofMessage,
		Parsers: []compiler.Parser{{Kind: protowire.StartGroupType, Thunk: parseOneofGroup}},
	},
}

func getOneofScalar[T tdp.Scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := xunsafe.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.Value{}
	}
	v := *dynamic.GetField[T](m, getter.Offset)
	return xprotoreflect.ValueOfScalar(v)
}

func getOneofBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := xunsafe.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.Value{}
	}
	v := *dynamic.GetField[byte](m, getter.Offset)
	return protoreflect.ValueOfBool(v != 0)
}

func getOneofString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := xunsafe.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.Value{}
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOfString(r.String(m.Shared.Src))
}

func getOneofBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := xunsafe.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.Value{}
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOfBytes(r.Bytes(m.Shared.Src))
}

func getOneofMessage(m *dynamic.Message, ty *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := xunsafe.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOfMessage(empty.NewMessage(ty))
	}
	ptr := *dynamic.GetField[*dynamic.Message](m, getter.Offset)
	return protoreflect.ValueOfMessage(ptr.ProtoReflect())
}

//go:nosplit
func parseOneofVarint32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseVarint32(p1, p2)
}

//go:nosplit
func parseOneofVarint64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseVarint64(p1, p2)
}

//go:nosplit
func parseOneofZigZag32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseZigZag32(p1, p2)
}

//go:nosplit
func parseOneofZigZag64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseZigZag64(p1, p2)
}

//go:nosplit
func parseOneofFixed32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseFixed32(p1, p2)
}

//go:nosplit
func parseOneofFixed64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseFixed64(p1, p2)
}

//go:nosplit
func parseOneofString(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseString(p1, p2)
}

//go:nosplit
func parseOneofBytes(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseBytes(p1, p2)
}

func parseOneofBool(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	if n != 0 {
		n = 1
	}
	p1, p2 = p1.SetScratch(p2, n)
	p1, p2 = vm.StoreFromScratch[byte](p1, p2)
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

func parseOneofMessage(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseMessage(p1, p2)
}

func parseOneofGroup(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	xunsafe.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseGroup(p1, p2)
}
