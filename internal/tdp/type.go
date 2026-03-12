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

package tdp

import (
	"fmt"
	"iter"
	_ "unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/xunsafe"
)

const SignBits = 0x80_80_80_80_80_80_80_80

type Type struct {
	_ xunsafe.NoCopy
	*Aux

	// The number of bytes of memory that must be allocated for a *message of
	// this type. This includes the Size of the header. Alignment is implicitly
	// that of uint64.
	Size, ColdSize uint32

	// The "unspecialized" Parser for this type.
	Parser *TypeParser

	// Maps field Numbers to offsets in fields.
	Numbers *swiss.Table[int32, uint32]

	// The number of fields that follow this type, not including the special
	// padding field with number equal to zero.
	Count uint32

	// Followed by:
	// 1. An array of fields of length equal to count+1.
	// 2. A table.Table that maps field numbers to entires in the
	//    aforementioned field table.
}

// ByIndex returns the nth byIndex (in byIndex number order) for this type.
//
// If n == 0 and this type has no fields, returns a byIndex with an invalid byIndex number.
//
// This function does not perform bounds checks.
func (t *Type) ByIndex(n int) *Field {
	return xunsafe.Beyond[Field](t).Get(n)
}

// ByDescriptor returns the field with the given descriptor.
func (t *Type) ByDescriptor(fd protoreflect.FieldDescriptor) *Field {
	switch {
	case fd == nil:
		return nil
	case fd.ContainingMessage() != t.Descriptor:
		return nil
	case fd.IsExtension():
		idx := swiss.LookupI32xU32(t.Numbers, int32(fd.Number()))
		if idx == nil {
			return nil
		}
		return t.ByIndex(int(*idx))
	default:
		return t.ByIndex(fd.Index())
	}
}

// Submessages returns an iterator over the types of submessage fields in this
// type.
func (t *Type) Submessages() iter.Seq[*Type] {
	return func(yield func(*Type) bool) {
		for i := range t.Count {
			m := t.ByIndex(int(i)).Message
			if m != nil && !yield(m) {
				return
			}
		}
	}
}

// Format implements [fmt.Formatter].
func (t *Type) Format(s fmt.State, verb rune) {
	debug.Dict(
		debug.Fprintf("%p", t),
		"name", t.Aux.Descriptor.FullName(),
		"methods", t.Aux.Methods,
		"size", t.Size,
		"count", t.Count,
		"parser", debug.Fprintf("%p", t.Parser),
	).Format(s, verb)
}

// ProtoReflect wraps this type for reflection.
func (t *Type) ProtoReflect() protoreflect.MessageType {
	return hyperpb_ProtoReflect(t)
}

// wrapType is a callback to construct the root package's message type.
//
// It is connected to the root package via linkname.
//
//go:linkname hyperpb_ProtoReflect
func hyperpb_ProtoReflect(*Type) protoreflect.MessageType

// Aux is data on a typeHeader that is stored behind a pointer and kept
// alive in the traces struct in [compiler.compile]. These rarely-accessed
// fields ensure that parser-relevant data is closer together in cache.
type Aux struct {
	Layout debug.Value[TypeLayout]

	Library          *Library
	Descriptor       protoreflect.MessageDescriptor
	Methods          protoiface.Methods
	FieldDescriptors []protoreflect.FieldDescriptor

	// Field indices that are required or contain required fields.
	// Negative numbers are the complement of a message field which
	// might contain required fields.
	Required []int32
}

// TypeLayout is layout information for a [Type]. Only for debugging.
type TypeLayout struct {
	BitWords int           // Number of 32-bit words in the type.
	Fields   []FieldLayout // Sorted in offset order.
}

// TypeParser is a parser for some [Type]. A [Type] may have multiple parsers.
type TypeParser struct {
	_ xunsafe.NoCopy

	// Maps offsets to field tags for the first 128 field tags. A value of
	// -1 means that if there is a parser at that position, it is farther away
	// than the first 256 fields.
	TagLUT [128]uint8

	TypeOffset     uint32 // The type that this parser parses.
	DiscardUnknown bool   // Should unknown fields be kept?

	// Maps field tags to offsets in fields.
	Tags *swiss.Table[int32, uint32]

	// If this is an ordinary parser, this is the parser for parsing this
	// message as a "map entry"; that is, it will have a single field with
	// number 2 that forwards to this parser.
	MapEntry *TypeParser

	Entrypoint FieldParser

	// Followed by an unspecified number of fieldParser values.
}

// Fields returns a raw pointer to this parser's field array.
func (p *TypeParser) Fields() *xunsafe.VLA[FieldParser] {
	// Don't use Beyond, since Go does not inline it in a critical place.
	// TypeParser and FieldParser have the same alignment, so this can be
	// a pure pointer increment.
	return xunsafe.Cast[xunsafe.VLA[FieldParser]](xunsafe.Add(p, 1))
}

// Format implements [fmt.Formatter].
func (p *TypeParser) Format(s fmt.State, verb rune) {
	debug.Dict(
		debug.Fprintf("%p", p),
		"ty", p.TypeOffset,
		"tags", p.Tags,
	).Format(s, verb)
}
