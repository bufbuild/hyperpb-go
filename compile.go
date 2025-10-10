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

package hyperpb

import (
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/types/descriptorpb"

	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/profile"
	"buf.build/go/hyperpb/internal/tdp/thunks"
)

// CompileFileDescriptorSet unmarshals a google.protobuf.FileDescriptorSet from schema,
// looks up a message with the given name, and compiles a type for it.
func CompileFileDescriptorSet(fds *descriptorpb.FileDescriptorSet, messageName protoreflect.FullName, options ...CompileOption) (*MessageType, error) {
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}
	return CompileFileRegistry(files, messageName, options...)
}

// CompileMessageDescriptor compiles a descriptor into a [MessageType], for optimized parsing.
//
// Panics if md is too complicated (i.e. it exceeds internal limitations for the compiler).
func CompileMessageDescriptor(md protoreflect.MessageDescriptor, options ...CompileOption) *MessageType {
	opts := compiler.Options{
		Backend: (*backend)(nil),
	}

	for _, opt := range options {
		if opt.apply != nil {
			opt.apply(&opts)
		}
	}

	ty := compiler.Compile(md, opts)
	ty.Library.Metadata = options

	return wrapType(ty)
}

// CompileFileRegistry looks up a [protoreflect.MessageDescriptor] from a [protoregistry.Files] and
// compiles a [MessageType] for it.
//
// Returns an error if no message type exists in the registry with that name.
func CompileFileRegistry(files *protoregistry.Files, messageName protoreflect.FullName, options ...CompileOption) (*MessageType, error) {
	desc, err := files.FindDescriptorByName(messageName)
	if err != nil {
		return nil, err
	}
	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, protoregistry.NotFound
	}

	// Allow the caller to override the extension registry by placing our
	// default registry first.
	options = append([]CompileOption{WithExtensionsFromFiles(files)}, options...)
	return CompileMessageDescriptor(msgDesc, options...), nil
}

// backend implements the compiler backend interface.
type backend struct{}

func (*backend) SelectArchetype(fd protoreflect.FieldDescriptor, prof profile.Field) *compiler.Archetype {
	return thunks.SelectArchetype(fd, prof)
}

func (*backend) PopulateMethods(methods *protoiface.Methods) {
	methods.Flags = protoiface.SupportUnmarshalDiscardUnknown
	methods.Unmarshal = unmarshalShim
	methods.CheckInitialized = requiredShim
}
