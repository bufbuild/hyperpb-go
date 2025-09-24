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
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/arena/slice"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/repeated"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xunsafe"
)

func getRepeatedMessage(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeated.Messages[dynamic.Message]](m, getter.Offset)
	return protoreflect.ValueOfList(p.ProtoReflect())
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseRepeatedMessage(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)
	p1, p2 = p1.SetScratch(p2, uint64(n))
	p1, p2, m := allocRepeatedMessage(p1, p2)
	return p1.PushMessage(p2, m)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func parseRepeatedGroup(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2, m := allocRepeatedMessage(p1, p2)
	return p1.PushGroup(p2, m)
}

func allocRepeatedMessage(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, *dynamic.Message) {
	if debug.Enabled {
		return allocRepeatedMessageSplit(p1, p2)
	}
	return allocRepeatedMessage2(p1, p2)
}

//go:noinline
func allocRepeatedMessageSplit(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, *dynamic.Message) {
	return allocRepeatedMessage2(p1, p2)
}

// //go:nosplit // TODO(#30): Enable once upstream is fixed.
func allocRepeatedMessage2(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, *dynamic.Message) {
	var r *repeated.Messages[dynamic.Message]
	p1, p2, r = vm.GetMutableField[repeated.Messages[dynamic.Message]](p1, p2)
	p1.Log(p2, "repeated message", "%v", r.Raw)

	var m *dynamic.Message

	if r.Raw.Ptr != 0 && r.Stride == 0 {
		goto pointers
	}

	{
		ty := p1.Shared().Library().AtOffset(p2.Field().Message.TypeOffset)
		stride := int(ty.Size)
		s := slice.CastUntyped[byte](r.Raw)

		if r.Raw.Ptr == 0 {
			p1, p2, r = newInlineRepeatedField(p1, p2, r)
		} else if s.Len()+stride > s.Cap() {
			p1, p2 = spillInlineRepeatedField(p1, p2, r)
			p1.Log(p2, "repeated message spill", "%v->%v", s.Addr(), *r)

			goto pointers
		}

		s = slice.CastUntyped[byte](r.Raw)
		p := xunsafe.Add(s.Ptr(), s.Len())
		s = s.SetLen(s.Len() + stride)
		r.Raw = s.Addr().Untyped()

		p1.Log(p2, "inline repeated message", "%v, %p/%d", s.Addr(), p, stride)
		return vm.AllocInPlace(p1, p2, p)
	}

pointers:
	{
		p1, p2, m = vm.AllocMessage(p1, p2)

		r := xunsafe.Cast[slice.Addr[xunsafe.Addr[dynamic.Message]]](r)
		s := r.AssertValid()
		if s.Len() == s.Cap() {
			p1, p2, m = appendOneMessage(p1, p2, m)
			p1.Log(p2, "outline repeated message", "%v, %p", *r, m)
			return p1, p2, m
		}

		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, xunsafe.AddrOf(m))
		p1.Log(p2, "outline repeated message", "%v, %p", s.Addr(), m)
		*r = s.Addr()
	}

	return p1, p2, m
}

//go:noinline
func newInlineRepeatedField(p1 vm.P1, p2 vm.P2, r *repeated.Messages[dynamic.Message]) (vm.P1, vm.P2, *repeated.Messages[dynamic.Message]) {
	// First element of this field. Allocate a byte array large enough to
	// hold one element.
	ty := p1.Shared().Library().AtOffset(p2.Field().Message.TypeOffset)
	stride := ty.Size

	preload := max(1, p2.Field().Preload)
	s := slice.Make[byte](p1.Arena(), int(stride)*int(preload))
	s = s.SetLen(0)

	r.Raw = s.Addr().Untyped()
	r.Stride = stride

	return p1, p2, r
}

//go:noinline
func spillInlineRepeatedField(p1 vm.P1, p2 vm.P2, r *repeated.Messages[dynamic.Message]) (vm.P1, vm.P2) {
	ty := p1.Shared().Library().AtOffset(p2.Field().Message.TypeOffset)
	stride := int(ty.Size)
	s := slice.CastUntyped[byte](r.Raw)

	// Spill all of the messages onto a pointer slice.
	spill := slice.Make[xunsafe.Addr[dynamic.Message]](p1.Arena(), s.Cap()/stride*2)
	var j int
	for i := 0; i < s.Len(); i += stride {
		m := xunsafe.Cast[dynamic.Message](xunsafe.Add(s.Ptr(), i))
		spill.Store(j, xunsafe.AddrOf(m))
		j++
	}
	spill = spill.SetLen(j)

	r.Raw = spill.Addr().Untyped()
	r.Stride = 0 // Mark this as an outlined message.

	return p1, p2
}

//go:noinline
func appendOneMessage(p1 vm.P1, p2 vm.P2, m *dynamic.Message) (vm.P1, vm.P2, *dynamic.Message) {
	var r *repeated.Messages[dynamic.Message]
	p1, p2, r = vm.GetMutableField[repeated.Messages[dynamic.Message]](p1, p2)
	s := slice.CastUntyped[xunsafe.Addr[dynamic.Message]](r.Raw)
	r.Raw = s.AppendOne(p1.Arena(), xunsafe.AddrOf(m)).Addr().Untyped()
	return p1, p2, m
}
