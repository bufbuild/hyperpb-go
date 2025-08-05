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

// Package dynamic contains the implementation of hyperpb's dynamic message types.
package dynamic

import (
	"fmt"
	"math/bits"
	"strings"
	"unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/arena/slice"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
	"buf.build/go/hyperpb/internal/zc"
)

// Cold is portions of a message that are located in context.Cold.
type Cold struct {
	Unknown slice.Slice[zc.Range] // Unknown field chunks.
}

// Message is a dynamic message value.
//
// A *Message lives on some arena, and all of its submessages do too. Because
// arenas are designed such that if a pointer to any of its allocated data is
// reachable, the whole arena is reachable, simply holding a *Message into
// the arena will keep everything else alive.
//
// This means that *Message values not being directly operated on by the
// application do not need to be marked by the GC, because their memory already
// gets marked whenever the GC sweeps a *root. As such, all of the fields of
// a Message are laid out in memory that follows it.
type Message struct {
	_ xunsafe.NoCopy

	Shared     *Shared
	TypeOffset uint32

	// Index into context.cold; negative means no cold pointer.
	// Exported for open-coding in the parser.
	ColdIndex int32

	// Fields follow this. The bitset words are allocated immediately after
	// the end of the message, so they are easy to offset to.
	//
	// The field data follows that, and offsets into the field data are already
	// baked to include both the message header and the bitset words.
}

// Range iterates over every populated field.
func (m *Message) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	if m == nil {
		return
	}

	ty := m.Type()
	f := ty.ByIndex(0)
	i := 0
	for f.IsValid() {
		fd := ty.FieldDescriptors[i]
		v := f.Get(unsafe.Pointer(m))
		switch {
		case !v.IsValid():
			goto skip

		case fd.IsList():
			if v.List().Len() == 0 {
				goto skip
			}

		case fd.IsMap():
			if v.Map().Len() == 0 {
				goto skip
			}
		}

		if !yield(ty.FieldDescriptors[i], v) {
			return
		}

	skip:
		f = xunsafe.Add(f, 1)
		i++
	}
}

// Has reports whether a field is populated.
func (m *Message) Has(fd protoreflect.FieldDescriptor) bool {
	if m == nil {
		return false
	}

	f := m.Type().ByDescriptor(fd)
	if !f.IsValid() {
		return false
	}

	v := f.Get(unsafe.Pointer(m))
	switch {
	case !v.IsValid():
		return false

	case fd.IsList():
		return v.List().Len() > 0

	case fd.IsMap():
		return v.Map().Len() > 0

	default:
		return true
	}
}

// Get retrieves the value for a field.
func (m *Message) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	if m == nil {
		// We need to panic here because there's no "reasonable" way to return
		// a default for message-typed fields here.
		panic("called Get on nil hyperpb.Message")
	}

	f := m.Type().ByDescriptor(fd)
	if !f.IsValid() {
		return protoreflect.ValueOf(nil)
	}

	if v := f.Get(unsafe.Pointer(m)); v.IsValid() {
		// NOTE: non-singular fields always return a valid value.
		return v
	} else if fd.Cardinality() != protoreflect.Repeated && fd.Message() != nil {
		return protoreflect.ValueOfMessage(empty.NewMessage(f.Message))
	}

	return fd.Default()
}

// GetByIndex is like [Message.Get], but it takes a raw field index, performing
// no bounds checks.
func (m *Message) GetByIndexUnchecked(n int) protoreflect.Value {
	return m.Type().ByIndex(n).Get(unsafe.Pointer(m))
}

// GetField returns the field pointer for a given message.
//
// Returns nil if the field is cold and there is no cold region allocated.
func GetField[T any](m *Message, offset tdp.Offset) *T {
	if offset.Data < 0 {
		cold := m.Cold()
		if cold == nil {
			return nil
		}
		return xunsafe.ByteAdd[T](cold, ^offset.Data)
	}
	return xunsafe.ByteAdd[T](m, offset.Data)
}

// LoadField returns the field data for a given message.
//
// Returns nil if the field is cold and there is no cold region allocated.
func LoadField[T any](m *Message, offset tdp.Offset) T {
	if offset.Data < 0 {
		cold := m.Cold()
		if cold == nil {
			var z T
			return z
		}
		return *xunsafe.ByteAdd[T](cold, ^offset.Data)
	}
	return *xunsafe.ByteAdd[T](m, offset.Data)
}

// GetBit gets the value of the nth bit from this message's bitset.
func (m *Message) GetBit(n uint32) bool {
	words := xunsafe.Cast[uint32](xunsafe.Add(m, 1))
	word := xunsafe.Load(words, int(n)/32)
	mask := uint32(1) << (n % 32)
	return word&mask != 0
}

// StBit sets the value of the nth bit from this message's bitset.
func (m *Message) SetBit(n uint32, flag bool) {
	words := xunsafe.Cast[uint32](xunsafe.Add(m, 1))
	word := xunsafe.Add(words, int(n)/32)
	mask := uint32(1) << (n % 32)

	if flag {
		*word |= mask
	} else {
		*word &^= mask
	}
}

// Type returns this message's type.
func (m *Message) Type() *tdp.Type {
	return m.Shared.lib.AtOffset(m.TypeOffset)
}

// cold returns a pointer to the cold region, or nil if it hasn't been allocated.
func (m *Message) Cold() *Cold {
	if m.ColdIndex < 0 {
		return nil
	}
	return xunsafe.LoadSlice(m.Shared.Cold, m.ColdIndex)
}

// MutableCold returns a pointer to the cold region, allocating one if needed.
func (m *Message) MutableCold() *Cold {
	if m.ColdIndex < 0 {
		size := int(m.Type().ColdSize)
		cold := xunsafe.Cast[Cold](m.Shared.arena.Alloc(size))

		m.ColdIndex = int32(len(m.Shared.Cold))
		m.Shared.Cold = append(m.Shared.Cold, cold)
		return cold
	}
	return xunsafe.LoadSlice(m.Shared.Cold, m.ColdIndex)
}

// ProtoReflect is a callback to construct the root package's message type.
//
// It is connected to the root package via linkname.
func (m *Message) ProtoReflect() protoreflect.Message {
	return hyperpb_ProtoReflect(m)
}

//go:linkname hyperpb_ProtoReflect
func hyperpb_ProtoReflect(*Message) protoreflect.Message

// Dump dumps the internal state of a message.
func (m *Message) Dump() string {
	buf := new(strings.Builder)
	cold := m.Cold()

	fmt.Fprintf(buf, "type: %p:%v, %p/%#x\n",
		m.Type().Descriptor, m.Type().Descriptor.FullName(), m.Type(), m.TypeOffset)
	fmt.Fprintf(buf, "hot:  %p, %d/%#[2]x\n", m, m.Type().Size)
	fmt.Fprintf(buf, "cold: %p, %d/%#[2]x\n", cold, m.Type().ColdSize)
	fmt.Fprintf(buf, "ctx:  %p\n", m.Shared)

	if !debug.Enabled {
		fmt.Fprintln(buf, "bits: ???")
		fmt.Fprintln(buf, "fields: ???")
		return buf.String()
	}

	tLayout := m.Type().Layout.Get()

	// Print out the bit words.
	if tLayout.BitWords > 0 {
		fmt.Fprint(buf, "bits:")
		words := xunsafe.Beyond[byte](m).Slice(tLayout.BitWords * 4)
		for i, word := range words {
			if i > 0 && i%4 == 0 {
				fmt.Fprintf(buf, "\n     ")
			}
			fmt.Fprintf(buf, " %08b", bits.Reverse8(word))
		}
		fmt.Fprintln(buf)
	}

	// Print out each field.
	if len(tLayout.Fields) > 0 {
		fmt.Fprintln(buf, "fields:")
		oneofs := m.Type().Descriptor.Oneofs()

		var maxBits uint32
		for _, field := range tLayout.Fields {
			maxBits = max(field.Bits, maxBits)
		}

		for _, field := range tLayout.Fields {
			start := buf.Len()
			data := GetField[byte](m, field.Offset)

			switch {
			case field.Size == 0:
				fmt.Fprint(buf, "  0x----/0x")

				nybbles := (bits.Len(uint(xunsafe.AddrOf(data))) + 3) / 4
				for range nybbles {
					fmt.Fprint(buf, "-")
				}
			case field.Offset.Data < 0:
				fmt.Fprintf(buf, " ^%#04x/%p", ^field.Offset.Data, data)
			default:
				fmt.Fprintf(buf, "  %#04x/%p", field.Offset.Data, data)
			}

			if field.Index >= 0 {
				fd := m.Type().FieldDescriptors[field.Index]
				if fd.IsExtension() {
					fmt.Fprintf(buf, " [%s]/%d:", fd.Name(), fd.Number())
				} else {
					fmt.Fprintf(buf, " %s/%d:", fd.Name(), fd.Number())
				}
			} else {
				od := oneofs.Get(^field.Index)
				fmt.Fprintf(buf, " %s/", od.Name())
				for i := range od.Fields().Len() {
					if i > 0 {
						buf.WriteByte(',')
					}
					fmt.Fprint(buf, od.Fields().Get(i).Number())
				}
				buf.WriteByte(':')
			}

			for buf.Len()-start < 32 {
				buf.WriteByte(' ')
			}

			if maxBits > 0 {
				fmt.Fprint(buf, " ")
				for i := range field.Bits {
					if m.GetBit(i + field.Offset.Bit) {
						fmt.Fprint(buf, "1")
					} else {
						fmt.Fprint(buf, "0")
					}
				}
				for range maxBits - field.Bits {
					fmt.Fprint(buf, "-")
				}
			}

			// Print each byte of data, grouped into words of four.
			if data == nil {
				fmt.Fprintln(buf, "---")
				continue
			}

			for i := range field.Size + field.Padding {
				if i == field.Size {
					fmt.Fprint(buf, " | ")
				} else if i%4 == 0 {
					fmt.Fprint(buf, " ")
				}
				fmt.Fprintf(buf, "%02x", xunsafe.ByteLoad[byte](data, i))
			}

			if int(field.Size) == layout.Size[zc.Range]() {
				zc := xunsafe.ByteLoad[zc.Range](data, 0)
				start, end := zc.Start(), zc.End()
				if start <= m.Shared.Len && end <= m.Shared.Len && start < end {
					fmt.Fprintf(buf, " %q", zc.Bytes(m.Shared.Src))
				}
			}

			fmt.Fprintln(buf)
		}
	}

	if cold != nil && cold.Unknown.Len() > 0 {
		fmt.Fprint(buf, "unknown:")
		for _, unknown := range cold.Unknown.Raw() {
			fmt.Fprintf(buf, "  %v `%x`\n", unknown, unknown.Bytes(m.Shared.Src))
		}
		fmt.Fprintln(buf)
	}

	return buf.String()
}
