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
	"errors"
	"fmt"
	"unsafe"
	_ "unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
)

var (
	_ proto.Message        = new(Message)
	_ protoreflect.Message = new(Message)

	errInvalid = errors.New("hyperpb: invalid message")

	//go:linkname hyperpbMessage buf.build/go/hyperpb/internal/tdp/profile.hyperpbMessage
	hyperpbMessage = xunsafe.AnyType((*Message)(nil))
)

// Message implements [protoreflect.Message].
type Message struct {
	impl dynamic.Message
}

// NewMessage allocates a new [Message] of the given [MessageType].
//
// See [Shared.NewMessage].
func NewMessage(ty *MessageType) *Message {
	return new(Shared).NewMessage(ty)
}

// Unmarshal is like [proto.Unmarshal], but permits hyperpb-specific
// tuning options to be set.
//
// Calling this function may be much faster than calling proto.Unmarshal if
// the message is small; proto.Unmarshal includes several nanoseconds of
// overhead that can become noticeable for message in the 16 byte regime.
//
// The returned error may additionally implement a method with the signature
//
//	Offset() int
//
// This function will return the approximate offset into data at which the
// error occurred.
func (m *Message) Unmarshal(data []byte, options ...UnmarshalOption) error {
	opts := vm.NewOptions()
	for _, opt := range options {
		if opt.apply != nil {
			// Avoid having opt pointlessly escape to the heap.
			// Users cannot create their own UnmarshalOptions, so violation
			// of memory safety isn't a thing we need to worry about.
			opt.apply(xunsafe.NoEscape(&opts))
		}
	}
	return vm.Run(&m.impl, data, opts)
}

// Shared returns state shared by this message and its submessages.
func (m *Message) Shared() *Shared {
	return wrapShared(m.impl.Shared)
}

// ProtoReflect implements [proto.Message].
func (m *Message) ProtoReflect() protoreflect.Message {
	return m
}

// Descriptor returns message descriptor, which contains only the Protobuf
// type information for the message.
//
// Descriptor implements [protoreflect.Message].
func (m *Message) Descriptor() protoreflect.MessageDescriptor {
	return m.impl.Type().Descriptor
}

// Type returns the message type, which encapsulates both Go and Protobuf
// type information. If the Go type information is not needed,
// it is recommended that the message descriptor be used instead.
//
// Type implements [protoreflect.Message]; Always returns *[MessageType].
func (m *Message) Type() protoreflect.MessageType {
	return m.HyperType()
}

// HyperType returns the [MessageType] for this value.
func (m *Message) HyperType() *MessageType {
	return wrapType(m.impl.Type())
}

// New returns a newly allocated empty message.
//
// New implements [protoreflect.Message].
func (m *Message) New() protoreflect.Message {
	return wrapType(m.impl.Type()).New()
}

// Interface returns m.
//
// Interface implements [protoreflect.Message].
func (m *Message) Interface() protoreflect.ProtoMessage {
	return m
}

// Range iterates over every populated field in an undefined order,
// calling f for each field descriptor and value encountered.
// Range returns immediately if f returns false.
//
// Range implements [protoreflect.Message].
func (m *Message) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	m.impl.Range(yield)
}

// Has reports whether a field is populated.
//
// Some fields have the property of nullability where it is possible to
// distinguish between the default value of a field and whether the field
// was explicitly populated with the default value. Singular message fields,
// member fields of a oneof, and proto2 scalar fields are nullable. Such
// fields are populated only if explicitly set.
//
// In other cases (aside from the nullable cases above),
// a proto3 scalar field is populated if it contains a non-zero value, and
// a repeated field is populated if it is non-empty.
//
// Has implements [protoreflect.Message].
func (m *Message) Has(fd protoreflect.FieldDescriptor) bool {
	return m.impl.Has(fd)
}

// Clear panics, unless this message has not been unmarshaled yet.
//
// Clear implements [protoreflect.Message].
func (m *Message) Clear(protoreflect.FieldDescriptor) {
	if m.Shared().impl.Src == nil {
		return
	}
	panic(debug.Unsupported())
}

// Reset panics, unless this message has not been unmarshaled yet
//
// Implements an interface used to speed up [proto.Reset]. It is not part of
// the [protoreflect.Message] interface.
func (m *Message) Reset() { m.Clear(nil) }

// Initialized returns whether m contains any unset required fields.
//
// Returns an error if any fields are not set.
func (m *Message) Initialized() error {
	// NOTE: This method does not implement an interface, but it does need to
	// match a protoiface method interface. See requiredShim below.

	if !m.IsValid() {
		return errInvalid
	}

	r := m.impl.Type().Required
	if r == nil {
		// Fast path!
		return nil
	}

	for _, idx := range r {
		if idx >= 0 {
			// This field is guaranteed to be singular.
			f := m.impl.Type().ByIndex(int(idx))
			v := f.Get(unsafe.Pointer(m))
			if !v.IsValid() {
				return fmt.Errorf(
					"hyperpb: uninitialized required field: %v",
					m.impl.Type().FieldDescriptors[idx],
				)
			}
			if _, empty := v.Interface().(empty.Message); empty {
				return fmt.Errorf(
					"hyperpb: uninitialized required field: %v",
					m.impl.Type().FieldDescriptors[idx],
				)
			}
			continue
		}

		// This is a message field, which we need to recurse into.
		f := m.impl.Type().ByIndex(int(^idx))

		switch v := f.Get(unsafe.Pointer(m)).Interface().(type) {
		case nil:
			continue
		case *Message:
			if err := v.Initialized(); err != nil {
				return err
			}
		case protoreflect.List:
			for i := range v.Len() {
				m := xprotoreflect.GetMessage[*Message](v.Get(i))
				if err := m.Initialized(); err != nil {
					return err
				}
			}
		case protoreflect.Map:
			for _, v := range v.Range {
				m := xprotoreflect.GetMessage[*Message](v)
				if err := m.Initialized(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Get retrieves the value for a field.
//
// For unpopulated scalars, it returns the default value, where
// the default value of a bytes scalar is guaranteed to be a copy.
// For unpopulated composite types, it returns an empty, read-only view
// of the value.
//
// Get implements [protoreflect.Message].
func (m *Message) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	return m.impl.Get(fd)
}

// Set panics.
//
// Set implements [protoreflect.Message].
func (m *Message) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(debug.Unsupported())
}

// Mutable panics.
//
// Mutable implements [protoreflect.Message].
func (m *Message) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(debug.Unsupported())
}

// NewField panics.
//
// NewField implements [protoreflect.Message].
func (m *Message) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(debug.Unsupported())
}

// WhichOneof reports which field within the oneof is populated,
// returning nil if none are populated.
// It panics if the oneof descriptor does not belong to this message.
//
// WhichOneof implements [protoreflect.Message].
func (m *Message) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	if !m.IsValid() {
		return nil
	}

	fd := od.Fields().Get(0)
	f := m.impl.Type().ByDescriptor(fd)
	if !f.IsValid() {
		panic("invalid oneof descriptor " + string(od.FullName()) + " for message " + string(m.Descriptor().FullName()))
	}

	if f.Offset.Number == 0 {
		// Not implemented internally as a oneof.
		if !m.Has(fd) {
			return nil
		}
		return fd
	}

	which := xunsafe.ByteLoad[uint32](m, f.Offset.Bit)
	return fd.ContainingMessage().Fields().ByNumber(protoreflect.FieldNumber(which))
}

// GetUnknown retrieves the entire list of unknown fields.
//
// GetUnknown implements [protoreflect.Message].
func (m *Message) GetUnknown() protoreflect.RawFields {
	cold := m.impl.Cold()
	if cold == nil {
		return nil
	}

	if cold.Unknown.Len() == 1 {
		return cold.Unknown.Ptr().Bytes(m.Shared().impl.Src)
	}

	var out []byte
	for _, zc := range cold.Unknown.Raw() {
		out = append(out, zc.Bytes(m.Shared().impl.Src)...)
	}
	return out
}

// SetUnknown panics, unless raw is zero-length, in which case it does nothing.
//
// SetUnknown implements [protoreflect.Message].
func (m *Message) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(debug.Unsupported())
}

// IsValid reports whether the message is valid.
//
// An invalid message is an empty, read-only value.
//
// An invalid message often corresponds to a nil pointer of the concrete
// message type, but the details are implementation dependent.
// Validity is not part of the protobuf data model, and may not
// be preserved in marshaling or other operations.
//
// IsValid implements [protoreflect.Message].
func (m *Message) IsValid() bool {
	return m != nil
}

// ProtoMethods returns optional fast-path implementations of various operations.
//
// ProtoMethods implements [protoreflect.Message].
func (m *Message) ProtoMethods() *protoiface.Methods {
	return &m.impl.Type().Methods
}

// unmarshalShim implements [protoiface.Methods].Unmarshal.
func unmarshalShim(in protoiface.UnmarshalInput) (out protoiface.UnmarshalOutput, err error) {
	// Gencode can pass non-Message types here, but they're all *dynamic.Messages internally.
	raw := xunsafe.AnyData(in.Message)
	m := xunsafe.Cast[Message](raw)

	err = m.Unmarshal(
		in.Buf,
		WithDiscardUnknown(in.Flags&protoiface.UnmarshalDiscardUnknown != 0),
	)
	if m.impl.Type().Required == nil {
		out.Flags |= protoiface.UnmarshalInitialized
	}
	return out, err
}

// requiredShim implements [protoiface.Methods].CheckInitialized.
func requiredShim(in protoiface.CheckInitializedInput) (out protoiface.CheckInitializedOutput, err error) {
	//nolint:errcheck // This conversion will never fail.
	return out, in.Message.(*Message).Initialized()
}

// wrapMessage wraps an internal Message pointer.
func wrapMessage(m *dynamic.Message) *Message {
	return xunsafe.Cast[Message](m)
}

//go:linkname protoReflect buf.build/go/hyperpb/internal/tdp/dynamic.hyperpb_ProtoReflect
func protoReflect(m *dynamic.Message) protoreflect.Message {
	return wrapMessage(m)
}
