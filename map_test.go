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

package hyperpb_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"buf.build/go/hyperpb"
	testpb "buf.build/go/hyperpb/internal/gen/test"
)

func TestMapMessageOmittedValue(t *testing.T) {
	t.Parallel()

	structTy := hyperpb.CompileMessageDescriptor((&structpb.Struct{}).ProtoReflect().Descriptor())
	mapsTy := hyperpb.CompileMessageDescriptor((&testpb.MessageMaps{}).ProtoReflect().Descriptor())

	// google.protobuf.Struct.fields is map<string, Value>, field 1.
	// Value.string_value is field 3.
	hi := fieldBytes(3, []byte("hi"))
	// MessageMaps.scalars.a1, used as a map value payload.
	body := func(a1 uint64) []byte {
		return fieldBytes(1, fieldVarint(1, a1))
	}

	tests := []struct {
		name string
		ty   *hyperpb.MessageType
		new  func() proto.Message
		wire []byte
	}{
		{
			name: "struct/issue reproducer",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: []byte{
				0x0a, 0x03, 0x0a, 0x01, 'x', // key "x", value omitted
				0x0a, 0x05, 0x0a, 0x01, 'y', 0x12, 0x00, // key "y", empty Value
			},
		},
		{
			name: "struct/trailing key only",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: cat(
				fieldBytes(1, cat(stringKey("y"), fieldBytes(2, hi))),
				fieldBytes(1, stringKey("x")),
			),
		},
		{
			name: "struct/multiple key only",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: cat(
				fieldBytes(1, stringKey("a")),
				fieldBytes(1, stringKey("b")),
				fieldBytes(1, stringKey("c")),
			),
		},
		{
			name: "struct/key only at end of buffer",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, stringKey("x")),
		},
		{
			name: "struct/empty string key",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, stringKey("")),
		},
		{
			name: "struct/empty entry",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, nil),
		},
		{
			name: "struct/value only",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, fieldBytes(2, hi)),
		},
		{
			name: "struct/key and nonempty value",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, cat(stringKey("z"), fieldBytes(2, hi))),
		},
		{
			name: "struct/value then key",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, cat(fieldBytes(2, hi), stringKey("z"))),
		},
		{
			name: "struct/duplicate keys",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, cat(stringKey("old"), stringKey("new"), fieldBytes(2, hi))),
		},
		{
			name: "struct/value then unknown field",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, cat(stringKey("a"), fieldBytes(2, hi), fieldVarint(15, 1))),
		},
		{
			name: "struct/two values",
			ty:   structTy,
			new:  func() proto.Message { return &structpb.Struct{} },
			wire: fieldBytes(1, cat(
				stringKey("a"),
				fieldBytes(2, fieldBytes(3, []byte("one"))),
				fieldBytes(2, fieldBytes(3, []byte("two"))),
			)),
		},
		{
			name: "int32/key only then value",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x11, fieldVarint(1, 7)),
				fieldBytes(0x11, cat(fieldVarint(1, 8), fieldBytes(2, body(3)))),
			),
		},
		{
			name: "int32/trailing key only",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x11, cat(fieldVarint(1, 8), fieldBytes(2, body(3)))),
				fieldBytes(0x11, fieldVarint(1, 9)),
			),
		},
		{
			name: "int32/empty entry and value only",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x11, nil),
				fieldBytes(0x11, fieldBytes(2, body(4))),
			),
		},
		{
			name: "int32/value then key",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: fieldBytes(0x11, cat(fieldBytes(2, body(5)), fieldVarint(1, 6))),
		},
		{
			name: "sint32/key only",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x15, fieldVarint(1, zigzag32(-2))),
				fieldBytes(0x15, cat(fieldVarint(1, zigzag32(3)), fieldBytes(2, body(1)))),
			),
		},
		{
			name: "fixed32/key only",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x17, fieldFixed32(1, 0x11)),
				fieldBytes(0x17, cat(fieldFixed32(1, 0x22), fieldBytes(2, body(9)))),
			),
		},
		{
			name: "fixed64/key only at end",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: fieldBytes(0x18, fieldFixed64(1, 0x0102030405060708)),
		},
		{
			name: "string/key only between values",
			ty:   mapsTy,
			new:  func() proto.Message { return &testpb.MessageMaps{} },
			wire: cat(
				fieldBytes(0x1c, cat(stringKey("a"), fieldBytes(2, body(1)))),
				fieldBytes(0x1c, stringKey("b")),
				fieldBytes(0x1c, cat(stringKey("c"), fieldBytes(2, nil))),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := tt.new()
			require.NoError(t, proto.Unmarshal(tt.wire, want))

			shared := new(hyperpb.Shared)
			t.Cleanup(shared.Free)
			got := shared.NewMessage(tt.ty)
			require.NoError(t, got.Unmarshal(tt.wire))
			require.True(t, proto.Equal(got, want))
		})
	}
}

func cat(parts ...[]byte) []byte {
	var n int
	for _, p := range parts {
		n += len(p)
	}
	b := make([]byte, 0, n)
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}

func fieldBytes(num protowire.Number, v []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, num, protowire.BytesType), v)
}

func fieldVarint(num protowire.Number, v uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(nil, num, protowire.VarintType), v)
}

func fieldFixed32(num protowire.Number, v uint32) []byte {
	return protowire.AppendFixed32(protowire.AppendTag(nil, num, protowire.Fixed32Type), v)
}

func fieldFixed64(num protowire.Number, v uint64) []byte {
	return protowire.AppendFixed64(protowire.AppendTag(nil, num, protowire.Fixed64Type), v)
}

func stringKey(s string) []byte {
	return fieldBytes(1, []byte(s))
}

func zigzag32(n int32) uint64 {
	return uint64(uint32(n<<1) ^ uint32(n>>31))
}
