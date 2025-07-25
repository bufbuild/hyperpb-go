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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: rsb/options.proto

// buf:lint:ignore PACKAGE_VERSION_SUFFIX
// buf:lint:ignore PACKAGE_DIRECTORY_MATCH

package rsb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MessageOptions struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MaxDepth      *int32                 `protobuf:"varint,1,opt,name=max_depth,json=maxDepth,proto3,oneof" json:"max_depth,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MessageOptions) Reset() {
	*x = MessageOptions{}
	mi := &file_rsb_options_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MessageOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageOptions) ProtoMessage() {}

func (x *MessageOptions) ProtoReflect() protoreflect.Message {
	mi := &file_rsb_options_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MessageOptions.ProtoReflect.Descriptor instead.
func (*MessageOptions) Descriptor() ([]byte, []int) {
	return file_rsb_options_proto_rawDescGZIP(), []int{0}
}

func (x *MessageOptions) GetMaxDepth() int32 {
	if x != nil && x.MaxDepth != nil {
		return *x.MaxDepth
	}
	return 0
}

// Options for generating a field value.
type FieldOptions struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	P             float64                `protobuf:"fixed64,1,opt,name=p,proto3" json:"p,omitempty"` // Probability of being set.
	Int           *FieldOptions_Int      `protobuf:"bytes,2,opt,name=int,proto3" json:"int,omitempty"`
	Uint          *FieldOptions_Uint     `protobuf:"bytes,3,opt,name=uint,proto3" json:"uint,omitempty"`
	Len           *FieldOptions_Len      `protobuf:"bytes,10,opt,name=len,proto3" json:"len,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FieldOptions) Reset() {
	*x = FieldOptions{}
	mi := &file_rsb_options_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FieldOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FieldOptions) ProtoMessage() {}

func (x *FieldOptions) ProtoReflect() protoreflect.Message {
	mi := &file_rsb_options_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FieldOptions.ProtoReflect.Descriptor instead.
func (*FieldOptions) Descriptor() ([]byte, []int) {
	return file_rsb_options_proto_rawDescGZIP(), []int{1}
}

func (x *FieldOptions) GetP() float64 {
	if x != nil {
		return x.P
	}
	return 0
}

func (x *FieldOptions) GetInt() *FieldOptions_Int {
	if x != nil {
		return x.Int
	}
	return nil
}

func (x *FieldOptions) GetUint() *FieldOptions_Uint {
	if x != nil {
		return x.Uint
	}
	return nil
}

func (x *FieldOptions) GetLen() *FieldOptions_Len {
	if x != nil {
		return x.Len
	}
	return nil
}

type FieldOptions_Int struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Min           *int64                 `protobuf:"zigzag64,1,opt,name=min,proto3,oneof" json:"min,omitempty"`
	Max           *int64                 `protobuf:"zigzag64,2,opt,name=max,proto3,oneof" json:"max,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FieldOptions_Int) Reset() {
	*x = FieldOptions_Int{}
	mi := &file_rsb_options_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FieldOptions_Int) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FieldOptions_Int) ProtoMessage() {}

func (x *FieldOptions_Int) ProtoReflect() protoreflect.Message {
	mi := &file_rsb_options_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FieldOptions_Int.ProtoReflect.Descriptor instead.
func (*FieldOptions_Int) Descriptor() ([]byte, []int) {
	return file_rsb_options_proto_rawDescGZIP(), []int{1, 0}
}

func (x *FieldOptions_Int) GetMin() int64 {
	if x != nil && x.Min != nil {
		return *x.Min
	}
	return 0
}

func (x *FieldOptions_Int) GetMax() int64 {
	if x != nil && x.Max != nil {
		return *x.Max
	}
	return 0
}

type FieldOptions_Uint struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Min           *uint64                `protobuf:"varint,1,opt,name=min,proto3,oneof" json:"min,omitempty"`
	Max           *uint64                `protobuf:"varint,2,opt,name=max,proto3,oneof" json:"max,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FieldOptions_Uint) Reset() {
	*x = FieldOptions_Uint{}
	mi := &file_rsb_options_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FieldOptions_Uint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FieldOptions_Uint) ProtoMessage() {}

func (x *FieldOptions_Uint) ProtoReflect() protoreflect.Message {
	mi := &file_rsb_options_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FieldOptions_Uint.ProtoReflect.Descriptor instead.
func (*FieldOptions_Uint) Descriptor() ([]byte, []int) {
	return file_rsb_options_proto_rawDescGZIP(), []int{1, 1}
}

func (x *FieldOptions_Uint) GetMin() uint64 {
	if x != nil && x.Min != nil {
		return *x.Min
	}
	return 0
}

func (x *FieldOptions_Uint) GetMax() uint64 {
	if x != nil && x.Max != nil {
		return *x.Max
	}
	return 0
}

type FieldOptions_Len struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Min           *int32                 `protobuf:"varint,1,opt,name=min,proto3,oneof" json:"min,omitempty"`
	Max           *int32                 `protobuf:"varint,2,opt,name=max,proto3,oneof" json:"max,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *FieldOptions_Len) Reset() {
	*x = FieldOptions_Len{}
	mi := &file_rsb_options_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *FieldOptions_Len) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FieldOptions_Len) ProtoMessage() {}

func (x *FieldOptions_Len) ProtoReflect() protoreflect.Message {
	mi := &file_rsb_options_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FieldOptions_Len.ProtoReflect.Descriptor instead.
func (*FieldOptions_Len) Descriptor() ([]byte, []int) {
	return file_rsb_options_proto_rawDescGZIP(), []int{1, 2}
}

func (x *FieldOptions_Len) GetMin() int32 {
	if x != nil && x.Min != nil {
		return *x.Min
	}
	return 0
}

func (x *FieldOptions_Len) GetMax() int32 {
	if x != nil && x.Max != nil {
		return *x.Max
	}
	return 0
}

var file_rsb_options_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*MessageOptions)(nil),
		Field:         777777,
		Name:          "hyperpb.rsb.m",
		Tag:           "bytes,777777,opt,name=m",
		Filename:      "rsb/options.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*FieldOptions)(nil),
		Field:         777777,
		Name:          "hyperpb.rsb.f",
		Tag:           "bytes,777777,opt,name=f",
		Filename:      "rsb/options.proto",
	},
}

// Extension fields to descriptorpb.MessageOptions.
var (
	// optional hyperpb.rsb.MessageOptions m = 777777;
	E_M = &file_rsb_options_proto_extTypes[0]
)

// Extension fields to descriptorpb.FieldOptions.
var (
	// optional hyperpb.rsb.FieldOptions f = 777777;
	E_F = &file_rsb_options_proto_extTypes[1]
)

var File_rsb_options_proto protoreflect.FileDescriptor

const file_rsb_options_proto_rawDesc = "" +
	"\n" +
	"\x11rsb/options.proto\x12\vhyperpb.rsb\x1a google/protobuf/descriptor.proto\"@\n" +
	"\x0eMessageOptions\x12 \n" +
	"\tmax_depth\x18\x01 \x01(\x05H\x00R\bmaxDepth\x88\x01\x01B\f\n" +
	"\n" +
	"_max_depth\"\x82\x03\n" +
	"\fFieldOptions\x12\f\n" +
	"\x01p\x18\x01 \x01(\x01R\x01p\x12/\n" +
	"\x03int\x18\x02 \x01(\v2\x1d.hyperpb.rsb.FieldOptions.IntR\x03int\x122\n" +
	"\x04uint\x18\x03 \x01(\v2\x1e.hyperpb.rsb.FieldOptions.UintR\x04uint\x12/\n" +
	"\x03len\x18\n" +
	" \x01(\v2\x1d.hyperpb.rsb.FieldOptions.LenR\x03len\x1aC\n" +
	"\x03Int\x12\x15\n" +
	"\x03min\x18\x01 \x01(\x12H\x00R\x03min\x88\x01\x01\x12\x15\n" +
	"\x03max\x18\x02 \x01(\x12H\x01R\x03max\x88\x01\x01B\x06\n" +
	"\x04_minB\x06\n" +
	"\x04_max\x1aD\n" +
	"\x04Uint\x12\x15\n" +
	"\x03min\x18\x01 \x01(\x04H\x00R\x03min\x88\x01\x01\x12\x15\n" +
	"\x03max\x18\x02 \x01(\x04H\x01R\x03max\x88\x01\x01B\x06\n" +
	"\x04_minB\x06\n" +
	"\x04_max\x1aC\n" +
	"\x03Len\x12\x15\n" +
	"\x03min\x18\x01 \x01(\x05H\x00R\x03min\x88\x01\x01\x12\x15\n" +
	"\x03max\x18\x02 \x01(\x05H\x01R\x03max\x88\x01\x01B\x06\n" +
	"\x04_minB\x06\n" +
	"\x04_max:L\n" +
	"\x01m\x12\x1f.google.protobuf.MessageOptions\x18\xb1\xbc/ \x01(\v2\x1b.hyperpb.rsb.MessageOptionsR\x01m:H\n" +
	"\x01f\x12\x1d.google.protobuf.FieldOptions\x18\xb1\xbc/ \x01(\v2\x19.hyperpb.rsb.FieldOptionsR\x01fB\x93\x01\n" +
	"\x0fcom.hyperpb.rsbB\fOptionsProtoP\x01Z%buf.build/go/hyperpb/internal/gen/rsb\xa2\x02\x03HRX\xaa\x02\vHyperpb.Rsb\xca\x02\vHyperpb\\Rsb\xe2\x02\x17Hyperpb\\Rsb\\GPBMetadata\xea\x02\fHyperpb::Rsbb\x06proto3"

var (
	file_rsb_options_proto_rawDescOnce sync.Once
	file_rsb_options_proto_rawDescData []byte
)

func file_rsb_options_proto_rawDescGZIP() []byte {
	file_rsb_options_proto_rawDescOnce.Do(func() {
		file_rsb_options_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_rsb_options_proto_rawDesc), len(file_rsb_options_proto_rawDesc)))
	})
	return file_rsb_options_proto_rawDescData
}

var file_rsb_options_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_rsb_options_proto_goTypes = []any{
	(*MessageOptions)(nil),              // 0: hyperpb.rsb.MessageOptions
	(*FieldOptions)(nil),                // 1: hyperpb.rsb.FieldOptions
	(*FieldOptions_Int)(nil),            // 2: hyperpb.rsb.FieldOptions.Int
	(*FieldOptions_Uint)(nil),           // 3: hyperpb.rsb.FieldOptions.Uint
	(*FieldOptions_Len)(nil),            // 4: hyperpb.rsb.FieldOptions.Len
	(*descriptorpb.MessageOptions)(nil), // 5: google.protobuf.MessageOptions
	(*descriptorpb.FieldOptions)(nil),   // 6: google.protobuf.FieldOptions
}
var file_rsb_options_proto_depIdxs = []int32{
	2, // 0: hyperpb.rsb.FieldOptions.int:type_name -> hyperpb.rsb.FieldOptions.Int
	3, // 1: hyperpb.rsb.FieldOptions.uint:type_name -> hyperpb.rsb.FieldOptions.Uint
	4, // 2: hyperpb.rsb.FieldOptions.len:type_name -> hyperpb.rsb.FieldOptions.Len
	5, // 3: hyperpb.rsb.m:extendee -> google.protobuf.MessageOptions
	6, // 4: hyperpb.rsb.f:extendee -> google.protobuf.FieldOptions
	0, // 5: hyperpb.rsb.m:type_name -> hyperpb.rsb.MessageOptions
	1, // 6: hyperpb.rsb.f:type_name -> hyperpb.rsb.FieldOptions
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	5, // [5:7] is the sub-list for extension type_name
	3, // [3:5] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_rsb_options_proto_init() }
func file_rsb_options_proto_init() {
	if File_rsb_options_proto != nil {
		return
	}
	file_rsb_options_proto_msgTypes[0].OneofWrappers = []any{}
	file_rsb_options_proto_msgTypes[2].OneofWrappers = []any{}
	file_rsb_options_proto_msgTypes[3].OneofWrappers = []any{}
	file_rsb_options_proto_msgTypes[4].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_rsb_options_proto_rawDesc), len(file_rsb_options_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 2,
			NumServices:   0,
		},
		GoTypes:           file_rsb_options_proto_goTypes,
		DependencyIndexes: file_rsb_options_proto_depIdxs,
		MessageInfos:      file_rsb_options_proto_msgTypes,
		ExtensionInfos:    file_rsb_options_proto_extTypes,
	}.Build()
	File_rsb_options_proto = out.File
	file_rsb_options_proto_goTypes = nil
	file_rsb_options_proto_depIdxs = nil
}
