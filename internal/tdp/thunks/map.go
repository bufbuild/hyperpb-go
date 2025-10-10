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
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/maps"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
	"buf.build/go/hyperpb/internal/zigzag"
)

// mapFields consists of archetypes for map fields. The first index is the key,
// the second is the value.
var mapFields = map[protoreflect.Kind]map[protoreflect.Kind]*compiler.Archetype{
	protoreflect.Int32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Int64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapV64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapV64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapV64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Uint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint32], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Uint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseMapV64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseMapV64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint64], parseMapV64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapZ32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapZ32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapZ32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapZ32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapZ32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapZ32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapZ32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapZ32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapZ32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapZ32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapZ32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapZ32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapZ32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapZ32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapZ32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapZ32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapZ32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapZ64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapZ64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapZ64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapZ64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapZ64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapZ64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapZ64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapZ64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapZ64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapZ64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapZ64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapZ64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapZ64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapZ64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapZ64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapZ64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapZ64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.Fixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseMapF32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseMapF32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint32], parseMapF32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Fixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseMapF64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseMapF64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint64], parseMapF64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sfixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapF32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapF32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapF32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sfixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapF64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapF64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapF64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.BoolKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMap2xI[int32], parseMap2xV32),
		protoreflect.Uint32Kind: mapArch(getMap2xI[uint32], parseMap2xV32),
		protoreflect.Sint32Kind: mapArch(getMap2xI[int32], parseMap2xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMap2xI[int64], parseMap2xV64),
		protoreflect.Uint64Kind: mapArch(getMap2xI[uint64], parseMap2xV64),
		protoreflect.Sint64Kind: mapArch(getMap2xI[int64], parseMap2xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMap2xI[uint32], parseMap2xF32),
		protoreflect.Sfixed32Kind: mapArch(getMap2xI[int32], parseMap2xF32),
		protoreflect.FloatKind:    mapArch(getMap2xI[float32], parseMap2xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMap2xI[uint64], parseMap2xF64),
		protoreflect.Sfixed64Kind: mapArch(getMap2xI[int64], parseMap2xF64),
		protoreflect.DoubleKind:   mapArch(getMap2xI[float64], parseMap2xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMap2xI[bool], parseMap2x2),
		protoreflect.EnumKind: mapArch(getMap2xI[protoreflect.EnumNumber], parseMap2xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMap2xS, parseMap2xS),
		protoreflect.BytesKind:  mapArch(getMap2xB, parseMap2xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMap2xM, parseMap2xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.EnumKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[protoreflect.EnumNumber, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[protoreflect.EnumNumber, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[protoreflect.EnumNumber, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[protoreflect.EnumNumber, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[protoreflect.EnumNumber], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[protoreflect.EnumNumber], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[protoreflect.EnumNumber], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseMapSxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseMapSxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseMapSxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseMapSxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseMapSxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseMapSxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseMapSxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseMapSxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseMapSxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseMapSxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseMapSxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseMapSxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseMapSx2),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseMapSxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseMapSxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseMapSxB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapSxM, parseMapSxM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	proto2StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseMapBxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseMapBxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseMapBxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseMapBxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseMapBxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseMapBxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseMapBxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseMapBxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseMapBxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseMapBxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseMapBxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseMapBxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseMapBx2),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseMapBxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseMapBxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseMapBxB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapSxM, parseMapBxM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
}

func init() {
	// Generate each of the entries for proto2StringKind by making copies of
	// the string archetype and using the bytes archetype's parser.
	for _, archs := range mapFields {
		arch := *archs[protoreflect.StringKind]
		arch.Parsers = archs[protoreflect.BytesKind].Parsers
		archs[proto2StringKind] = &arch
	}
}

// getMapIxI is a [getterThunk] for map<K, V> where K and V are both integer types.
func getMapIxI[K xprotoreflect.Int, V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.IntToScalar[K, V]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapIxS is a [getterThunk] for map<K, string> where K is an integer type.
func getMapIxS[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.IntToString[K]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapIxB is a [getterThunk] for map<K, bytes> where K is an integer type.
func getMapIxB[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.IntToBytes[K]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapIxM is a [getterThunk] for map<string, V> where V is an integer type.
func getMapIxM[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.IntToMessage[K, dynamic.Message]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapSxI is a [getterThunk] for map<string, V> where V is an integer type.
func getMapSxI[V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.StringToScalar[V]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapSxS is a [protoreflect.Map] for map<string, string>.
func getMapSxS(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.StringToString](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapSxS is a [protoreflect.Map] for map<string, bytes>.
func getMapSxB(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.StringToBytes](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMapSxM is a [getterThunk] for map<string, V> where V is a message type.
func getMapSxM(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.StringToMessage[dynamic.Message]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMap2xI is a [getterThunk] for map<bool, V> where V is an integer type.
func getMap2xI[V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.BoolToScalar[V]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMap2xS is a [getterThunk] for map<bool, string>.
func getMap2xS(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.BoolToString](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMap2xB is a [getterThunk] for map<bool, bytes>.
func getMap2xB(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.BoolToBytes](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// getMap2xM is a [getterThunk] for map<bool, V> where V is a message type.
func getMap2xM(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.LoadField[*maps.BoolToMessage[dynamic.Message]](m, getter.Offset)
	return protoreflect.ValueOfMap(v.ProtoReflect())
}

// mapArch is a helper for constructing map<K, V> archetypes, where K is not
// bool.
func mapArch(getter compiler.Getter, parser vm.Thunk) *compiler.Archetype {
	return &compiler.Archetype{
		// All maps use the same layout: a pointer to a swiss.Table.
		Layout:  layout.Of[*swiss.Table[int32, int32]](),
		Getter:  getter,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parser}},
	}
}

// mapItem is a type usable in any of the map parsers. This is essentially a
// shim for pushing slight custom behavior modifications to each of the stencils
// of e.g. [parseMapKxV].
type mapItem[V any] interface {
	// The wire type for this item.
	kind() protowire.Type

	// Parses a value of this item type and returns it.
	parse(vm.P1, vm.P2) (vm.P1, vm.P2, V)

	// Returns the key extraction function used with swiss.Table.Insert.
	extract(vm.P1, vm.P2) func(V) []byte
}

type (
	varint32Item struct{}
	varint64Item struct{}
	zigzag32Item struct{}
	zigzag64Item struct{}
	boolItem     struct{}
	fixed32Item  struct{}
	fixed64Item  struct{}
	stringItem   struct{}
	bytesItem    struct{}
)

var (
	_ mapItem[uint32] = varint32Item{}
	_ mapItem[uint64] = varint64Item{}
	_ mapItem[uint32] = zigzag32Item{}
	_ mapItem[uint64] = zigzag64Item{}
	_ mapItem[uint32] = fixed32Item{}
	_ mapItem[uint64] = fixed64Item{}
	_ mapItem[uint8]  = boolItem{}
	_ mapItem[uint64] = stringItem{}
	_ mapItem[uint64] = bytesItem{}
)

func (varint32Item) kind() protowire.Type { return protowire.VarintType }
func (varint64Item) kind() protowire.Type { return protowire.VarintType }
func (zigzag32Item) kind() protowire.Type { return protowire.VarintType }
func (zigzag64Item) kind() protowire.Type { return protowire.VarintType }
func (boolItem) kind() protowire.Type     { return protowire.VarintType }
func (fixed32Item) kind() protowire.Type  { return protowire.Fixed32Type }
func (fixed64Item) kind() protowire.Type  { return protowire.Fixed64Type }
func (stringItem) kind() protowire.Type   { return protowire.BytesType }
func (bytesItem) kind() protowire.Type    { return protowire.BytesType }

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (varint32Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint32) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	return p1, p2, uint32(n)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (varint64Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	return p1.Varint(p2)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (zigzag32Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint32) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	return p1, p2, zigzag.Decode64[uint32](n)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (zigzag64Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	return p1, p2, zigzag.Decode64[uint64](n)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func (boolItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint8) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	if n != 0 {
		n = 1
	}
	return p1, p2, uint8(n)
}

//go:nosplit
func (fixed32Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint32) {
	return p1.Fixed32(p2)
}

//go:nosplit
func (fixed64Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	return p1.Fixed64(p2)
}

//go:nosplit
func (stringItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	var r zc.Range
	p1, p2, r = p1.UTF8(p2)
	return p1, p2, uint64(r)
}

//go:nosplit
func (bytesItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	var r zc.Range
	p1, p2, r = p1.Bytes(p2)
	return p1, p2, uint64(r)
}

func (varint32Item) extract(vm.P1, vm.P2) func(uint32) []byte { return nil }
func (varint64Item) extract(vm.P1, vm.P2) func(uint64) []byte { return nil }
func (zigzag32Item) extract(vm.P1, vm.P2) func(uint32) []byte { return nil }
func (zigzag64Item) extract(vm.P1, vm.P2) func(uint64) []byte { return nil }
func (fixed32Item) extract(vm.P1, vm.P2) func(uint32) []byte  { return nil }
func (fixed64Item) extract(vm.P1, vm.P2) func(uint64) []byte  { return nil }
func (boolItem) extract(vm.P1, vm.P2) func(uint8) []byte      { return nil }
func (stringItem) extract(p1 vm.P1, _ vm.P2) func(uint64) []byte {
	return zc.ExtractFrom{Src: p1.Src()}.Bytes
}

func (bytesItem) extract(p1 vm.P1, _ vm.P2) func(uint64) []byte {
	return zc.ExtractFrom{Src: p1.Src()}.Bytes
}

//hyperpb:stencil parseMapV32xV32 parseMapKxV[varint32Item, varint32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapV32xV64 parseMapKxV[varint32Item, varint64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapV32xZ32 parseMapKxV[varint32Item, zigzag32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapV32xZ64 parseMapKxV[varint32Item, zigzag64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapV32xF32 parseMapKxV[varint32Item, fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapV32xF64 parseMapKxV[varint32Item, fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapV32x2   parseMapKxV[varint32Item, boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//hyperpb:stencil parseMapV32xS   parseMapKxV[varint32Item, stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapV32xB   parseMapKxV[varint32Item, bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//hyperpb:stencil parseMapV64xV32 parseMapKxV[varint64Item, varint32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapV64xV64 parseMapKxV[varint64Item, varint64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapV64xZ32 parseMapKxV[varint64Item, zigzag32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapV64xZ64 parseMapKxV[varint64Item, zigzag64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapV64xF32 parseMapKxV[varint64Item, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapV64xF64 parseMapKxV[varint64Item, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapV64x2   parseMapKxV[varint64Item, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//hyperpb:stencil parseMapV64xS   parseMapKxV[varint64Item, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapV64xB   parseMapKxV[varint64Item, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//hyperpb:stencil parseMapZ32xV32 parseMapKxV[zigzag32Item, varint32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapZ32xV64 parseMapKxV[zigzag32Item, varint64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapZ32xZ32 parseMapKxV[zigzag32Item, zigzag32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapZ32xZ64 parseMapKxV[zigzag32Item, zigzag64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapZ32xF32 parseMapKxV[zigzag32Item, fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapZ32xF64 parseMapKxV[zigzag32Item, fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapZ32x2   parseMapKxV[zigzag32Item, boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//hyperpb:stencil parseMapZ32xS   parseMapKxV[zigzag32Item, stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapZ32xB   parseMapKxV[zigzag32Item, bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//hyperpb:stencil parseMapZ64xV32 parseMapKxV[zigzag64Item, varint32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapZ64xV64 parseMapKxV[zigzag64Item, varint64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapZ64xZ32 parseMapKxV[zigzag64Item, zigzag32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapZ64xZ64 parseMapKxV[zigzag64Item, zigzag64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapZ64xF32 parseMapKxV[zigzag64Item, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapZ64xF64 parseMapKxV[zigzag64Item, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapZ64x2   parseMapKxV[zigzag64Item, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//hyperpb:stencil parseMapZ64xS   parseMapKxV[zigzag64Item, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapZ64xB   parseMapKxV[zigzag64Item, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//hyperpb:stencil parseMapF32xV32 parseMapKxV[fixed32Item, varint32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapF32xV64 parseMapKxV[fixed32Item, varint64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapF32xZ32 parseMapKxV[fixed32Item, zigzag32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapF32xZ64 parseMapKxV[fixed32Item, zigzag64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapF32xF32 parseMapKxV[fixed32Item, fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//hyperpb:stencil parseMapF32xF64 parseMapKxV[fixed32Item, fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapF32x2   parseMapKxV[fixed32Item, boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//hyperpb:stencil parseMapF32xS   parseMapKxV[fixed32Item, stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//hyperpb:stencil parseMapF32xB   parseMapKxV[fixed32Item, bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//hyperpb:stencil parseMapF64xV32 parseMapKxV[fixed64Item, varint32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapF64xV64 parseMapKxV[fixed64Item, varint64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapF64xZ32 parseMapKxV[fixed64Item, zigzag32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapF64xZ64 parseMapKxV[fixed64Item, zigzag64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapF64xF32 parseMapKxV[fixed64Item, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapF64xF64 parseMapKxV[fixed64Item, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapF64x2   parseMapKxV[fixed64Item, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//hyperpb:stencil parseMapF64xS   parseMapKxV[fixed64Item, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapF64xB   parseMapKxV[fixed64Item, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//hyperpb:stencil parseMapSxV32 parseMapKxV[stringItem, varint32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapSxV64 parseMapKxV[stringItem, varint64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapSxZ32 parseMapKxV[stringItem, zigzag32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapSxZ64 parseMapKxV[stringItem, zigzag64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapSxF32 parseMapKxV[stringItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapSxF64 parseMapKxV[stringItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapSx2   parseMapKxV[stringItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//hyperpb:stencil parseMapSxS   parseMapKxV[stringItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapSxB   parseMapKxV[stringItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//hyperpb:stencil parseMapBxV32 parseMapKxV[bytesItem, varint32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapBxV64 parseMapKxV[bytesItem, varint64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapBxZ32 parseMapKxV[bytesItem, zigzag32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapBxZ64 parseMapKxV[bytesItem, zigzag64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapBxF32 parseMapKxV[bytesItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//hyperpb:stencil parseMapBxF64 parseMapKxV[bytesItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapBx2   parseMapKxV[bytesItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//hyperpb:stencil parseMapBxS   parseMapKxV[bytesItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//hyperpb:stencil parseMapBxB   parseMapKxV[bytesItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//hyperpb:stencil parseMap2xV32 parseMapKxV[boolItem, varint32Item, uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//hyperpb:stencil parseMap2xV64 parseMapKxV[boolItem, varint64Item, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//hyperpb:stencil parseMap2xZ32 parseMapKxV[boolItem, zigzag32Item, uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//hyperpb:stencil parseMap2xZ64 parseMapKxV[boolItem, zigzag64Item, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//hyperpb:stencil parseMap2xF32 parseMapKxV[boolItem, fixed32Item, uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//hyperpb:stencil parseMap2xF64 parseMapKxV[boolItem, fixed64Item, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//hyperpb:stencil parseMap2x2   parseMapKxV[boolItem, boolItem, uint8, uint8] Init -> swiss.InitU8xU8 Insert -> swiss.InsertU8xU8
//hyperpb:stencil parseMap2xS   parseMapKxV[boolItem, stringItem, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//hyperpb:stencil parseMap2xB   parseMapKxV[boolItem, bytesItem, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64

// parseMapKxV parses a map type whose value is a non-message type.
func parseMapKxV[
	KI mapItem[K], VI mapItem[V],
	K swiss.Key, V any,
](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)

	p1, p2 = p1.SetScratch(p2, uint64(p1.EndAddr))
	p1.EndAddr = p1.PtrAddr.Add(n)

	var ki KI
	var vi VI
	var k K
	var v V

	kTag := protowire.EncodeTag(1, ki.kind())
	vTag := protowire.EncodeTag(2, vi.kind())

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.Len() == 0 {
		goto insert
	}
	p1.Log(p2, "first byte", "%#02x", *p1.Ptr())
	if *p1.Ptr() == byte(kTag) {
		p1.PtrAddr++
		p1, p2, k = ki.parse(p1, p2)
		if p1.Len() == 0 {
			goto insert
		}
		p1.Log(p2, "second byte", "%#02x", *p1.Ptr())
		if *p1.Ptr() == byte(vTag) {
			p1.PtrAddr++
			p1, p2, v = vi.parse(p1, p2)
			p1.Log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.PtrAddr, p1.EndAddr,
				k, xunsafe.Bytes(&k),
				v, xunsafe.Bytes(&v))
			if p1.PtrAddr == p1.EndAddr {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.PtrAddr < p1.EndAddr {
		var tag uint64
		p1, p2, tag = p1.Varint(p2)
		switch tag {
		case kTag:
			p1, p2, k = ki.parse(p1, p2)
		case vTag:
			p1, p2, v = vi.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.Buf())
			if m < 0 {
				p1.Fail(p2, -vm.ErrorCode(m))
			}
			p1.PtrAddr = p1.PtrAddr.Add(m)
		}
	}

insert:
	extract := ki.extract(p1, p2)
	var mp **swiss.Table[K, V]
	p1, p2, mp = vm.GetMutableField[*swiss.Table[K, V]](p1, p2)

	m := *mp
	if m == nil {
		cap := int(max(1, p2.Field().Preload))
		size, _ := swiss.Layout[K, V](cap)
		m = xunsafe.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		xunsafe.StoreNoWB(mp, m)
		m.Init(cap, nil, extract)
		xunsafe.StoreNoWB(&m.Scratch, p1.Shared().Src)
	}

	vp := m.Insert(k, extract)
	if vp == nil {
		size, _ := swiss.Layout[K, V](m.Len() + 1)
		m2 := xunsafe.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		xunsafe.StoreNoWB(mp, m2)
		m2.Init(m.Len()+1, m, extract)
		xunsafe.StoreNoWB(&m2.Scratch, p1.Shared().Src)
		vp = m2.Insert(k, extract)
	}

	*vp = v

	p1.EndAddr = xunsafe.Addr[byte](p2.Scratch())
	return p1, p2
}

//hyperpb:stencil parseMapV32xM parseMapKxM[varint32Item, uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//hyperpb:stencil parseMapV64xM parseMapKxM[varint64Item, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//hyperpb:stencil parseMapZ32xM parseMapKxM[zigzag32Item, uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//hyperpb:stencil parseMapZ64xM parseMapKxM[zigzag64Item, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//hyperpb:stencil parseMapF32xM parseMapKxM[fixed32Item, uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//hyperpb:stencil parseMapF64xM parseMapKxM[fixed64Item, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//hyperpb:stencil parseMapSxM   parseMapKxM[stringItem, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//hyperpb:stencil parseMapBxM   parseMapKxM[bytesItem, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//hyperpb:stencil parseMap2xM   parseMapKxM[boolItem, uint8] Init -> swiss.InitU8xP Insert -> swiss.InsertU8xP

// parseMapKxM parses a map type whose value is a message type.
func parseMapKxM[KI mapItem[K], K swiss.Key](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)

	p1, p2 = p1.SetScratch(p2, uint64(p1.EndAddr))
	p1.EndAddr = p1.PtrAddr.Add(n)

	var ki KI
	var k K
	var fast bool

	kTag := protowire.EncodeTag(1, ki.kind())
	vTag := protowire.EncodeTag(1, protowire.BytesType)

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.Len() == 0 {
		fast = true
		goto insert
	}
	p1.Log(p2, "first byte", "%#02x", *p1.Ptr())
	if *p1.Ptr() == byte(kTag) {
		p1.PtrAddr++
		p1, p2, k = ki.parse(p1, p2)
		if p1.Len() == 0 {
			fast = true
			goto insert
		}
		p1.Log(p2, "second byte", "%#02x", *p1.Ptr())
		if *p1.Ptr() == byte(vTag) {
			p1.PtrAddr++
			// Need to parse a length prefix and check if it reaches all the
			// way to the end of the message.
			p1, p2, n = p1.LengthPrefix(p2)
			if p1.EndAddr > p1.PtrAddr.Add(n) {
				fast = true
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.PtrAddr < p1.EndAddr {
		var tag uint64
		p1, p2, tag = p1.Varint(p2)
		switch tag {
		case kTag:
			p1, p2, k = ki.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.Buf())
			if m < 0 {
				p1.Fail(p2, -vm.ErrorCode(m))
			}
			p1.PtrAddr = p1.PtrAddr.Add(m)
		}
	}

	// Now we need to rewind back to the beginning.
	p1.PtrAddr = p1.EndAddr.Add(-n)

insert:
	type V = unsafe.Pointer

	extract := ki.extract(p1, p2)
	var mp **swiss.Table[K, V]
	p1, p2, mp = vm.GetMutableField[*swiss.Table[K, V]](p1, p2)

	m := *mp
	if m == nil {
		cap := int(max(1, p2.Field().Preload))
		size, _ := swiss.Layout[K, V](cap)
		m = xunsafe.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		xunsafe.StoreNoWB(mp, m)
		m.Init(cap, nil, extract)
		xunsafe.StoreNoWB(&m.Scratch, p1.Shared().Src)
	}

	vp := m.Insert(k, extract)
	if vp == nil {
		size, _ := swiss.Layout[K, V](m.Len() + 1)
		m2 := xunsafe.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		xunsafe.StoreNoWB(mp, m2)
		m2.Init(m.Len()+1, m, extract)
		xunsafe.StoreNoWB(&m2.Scratch, p1.Shared().Src)
		vp = m2.Insert(k, extract)
	}

	var v *dynamic.Message
	// Allocate unconditionally to match Go protobuf's behavior.
	// TODO: This could instead clear, but that optimization will almost never
	// be relevant, because no serializer will ever emit the same key twice.
	p1, p2, v = vm.AllocMessage(p1, p2)
	xunsafe.StoreNoWBUntyped(vp, unsafe.Pointer(v))

	// Unspill the old end pointer.
	p1.EndAddr = xunsafe.Addr[byte](p2.Scratch())
	p1, p2 = p1.SetScratch(p2, uint64(n))

	// Schedule a message parse.
	if fast {
		p1.Log(p2, "fast map entry", "%d", n)
		return p1.PushMessage(p2, v)
	}

	p1.Log(p2, "slow map entry", "%d", n)
	return p1.PushMapEntry(p2, v)
}
