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

// Package wkt contains descriptors for well-known-type components.
package wkt

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	_ "google.golang.org/protobuf/types/descriptorpb"
)

var (
	FileDescriptorProto          = message("google.protobuf.FileDescriptorProto")
	FileDescriptorProto_Syntax   = field(FileDescriptorProto, "syntax")
	FileDescriptorProto_Package  = field(FileDescriptorProto, "package")
	FileDescriptorProto_Messages = field(FileDescriptorProto, "message_type")

	MessageDescriptorProto          = message("google.protobuf.DescriptorProto")
	MessageDescriptorProto_Messages = field(MessageDescriptorProto, "nested_type")
	MessageDescriptorProto_Fields   = field(MessageDescriptorProto, "field")
)

func message(path string) protoreflect.MessageDescriptor {
	ty, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(path))
	if err != nil {
		panic(fmt.Errorf("not found: %s", path))
	}

	return ty.Descriptor()
}

func field(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	field := md.Fields().ByName(protoreflect.Name(name))
	if field == nil {
		panic(fmt.Errorf("not found: %s.%s", md.FullName(), name))
	}
	return field
}
