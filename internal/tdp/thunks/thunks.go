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

// Package thunks provides all thunks for the parser VM.
package thunks

import (
	_ "unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/profile"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/tdp/vm/varint"
)

//go:generate go run ../../tools/hyperstencil

// Custom field kinds used by archetype selection; they're all negative.
const (
	proto2StringKind protoreflect.Kind = ^iota
)

// SelectArchetype selects an archetype from among those in this package.
func SelectArchetype(fd protoreflect.FieldDescriptor, prof profile.Field) *compiler.Archetype {
	var a *compiler.Archetype
	od := fd.ContainingOneof()
	switch {
	case fd.IsMap():
		k := fieldKind(fd.MapKey(), prof)
		v := fieldKind(fd.MapValue(), prof)
		a = mapFields[k][v]
	case fd.IsList():
		a = repeatedFields[fieldKind(fd, prof)]
	case od != nil && od.Fields().Len() > 1:
		// One-element oneofs are treated like optional fields.
		a = oneofFields[fieldKind(fd, prof)]
	case fd.HasPresence():
		a = optionalFields[fieldKind(fd, prof)]
	default:
		a = singularFields[fieldKind(fd, prof)]
	}

	return a
}

// fieldKind extracts the field kind from a descriptor.
//
// This returns negative values for "custom" kinds.
func fieldKind(fd protoreflect.FieldDescriptor, prof profile.Field) protoreflect.Kind {
	switch k := fd.Kind(); k {
	case protoreflect.StringKind:
		fd2, ok := fd.(interface{ EnforceUTF8() bool })
		if !prof.AssumeUTF8 && (fd.Syntax() == protoreflect.Proto3 || (ok && fd2.EnforceUTF8())) {
			return protoreflect.StringKind
		}
		return proto2StringKind

	default:
		return k
	}
}

//go:nosplit
func varint32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	return varint.Varint32(p1, p2)
}

//go:nosplit
func varint64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	return varint.Varint64(p1, p2)
}
