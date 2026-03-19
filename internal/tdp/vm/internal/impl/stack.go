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

package impl

import (
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// stack is the recursion stack for the VM.
type stack struct {
	ptr         xunsafe.Addr[frame]
	top, bottom xunsafe.Addr[frame]
}

// frame is a recursion frame for the parser.
type frame struct {
	end     xunsafe.Addr[byte]
	group   tdp.Tag
	message xunsafe.Addr[dynamic.Message]
	Type    xunsafe.Addr[tdp.TypeParser]
	field   xunsafe.Addr[tdp.FieldParser]
}

// PushMessage pushes a new message to be parsed onto the parser stack.
//
// The length of the message should be in p2.Scratch().
//
//go:nosplit
func (p1 P1) PushMessage(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch())
	if len == 0 {
		return p1, p2
	}

	p1.Log(p2, "n", "%d", len)

	if p1.EndGroup != NotAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.EndGroup = NotAGroup
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.P3().ty = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = _PushMessage_log(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushMapEntry pushes a new map entry to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushMapEntry(p2 P2, m *dynamic.Message) (P1, P2) {
	len := int(p2.Scratch())
	if len == 0 {
		return p1, p2
	}

	if p1.EndGroup != NotAGroup || p1.PtrAddr.Add(len) != p1.EndAddr {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.PtrAddr.Add(len))
	}

	p1.EndGroup = NotAGroup
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser.MapEntry
	p2.P3().ty = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = _PushMessage_log(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

// PushGroup pushes a new group to be parsed onto the parser stack.
//
//go:nosplit
func (p1 P1) PushGroup(p2 P2, m *dynamic.Message) (P1, P2) {
	start := tdp.Tag(p2.Scratch())

	// Indeed, we can just +1, because we need to replace the low three
	// 0b011 bits with 0b100. Much simpler than clearing and overwriting those
	// bits!
	end := start + 1

	p1, p2 = p1.push(p2, p1.EndAddr)

	p1.EndGroup = end
	p2.MessageAddr = xunsafe.AddrOf(m)

	t := p2.Message().Type().Parser
	p2.P3().ty = xunsafe.AddrOf(t)
	if debug.Enabled {
		p1, p2 = _PushMessage_log(p1, p2)
	}

	p2.FieldAddr = xunsafe.AddrOf(&t.Entrypoint)

	return p1, p2
}

func (p3 *P3) stackSlice() []frame {
	n := p3.stack.bottom.Sub(p3.stack.ptr)
	return unsafe.Slice(p3.stack.ptr.AssertValid(), n)
}

// push pushes a parser frame.
//
//go:nosplit
func (p1 P1) push(p2 P2, end xunsafe.Addr[byte]) (P1, P2) {
	if debug.Enabled {
		p1, p2 = _push_log(p1, p2)
	}

	if p2.P3().stack.ptr == p2.P3().stack.top {
		p1.Fail(p2, tdp.ErrorRecursionDepth)
	}

	p2.P3().stack.ptr = p2.P3().stack.ptr.Add(-1)

	// Note: a single frame is just too large to hit Go's SROA pass (same bug
	// that results in p1/p2 being two structs). Thus, we write each field
	// separately to avoid wasteful stack traffic.
	frame := p2.P3().stack.ptr.AssertValid()
	frame.end = p1.EndAddr
	frame.group = p1.EndGroup
	frame.message = p2.MessageAddr
	frame.Type = p2.P3().ty
	frame.field = p2.FieldAddr

	p1.EndAddr = end
	return p1, p2
}

// Pop pops a parser frame.
//
// Returns whether the last frame was popped.
//
//go:nosplit
func (p1 P1) Pop(p2 P2) (P1, P2, bool) {
	if debug.Enabled {
		p1.Log(
			p2, "finish", "%v, ty: %p:%s %v",
			p2.MessageAddr,
			p2.Message().Type(),
			p2.Message().Type().Descriptor.FullName(),
			p2.Message().Type(),
		)

		s := &p2.P3().stack
		p1.Log(p2, "pop", "%v/%v/%v\n%s", s.top, s.ptr, s.bottom,
			p2.Message().Dump())
	}

	last := p2.P3().stack.ptr.AssertValid()
	p1.EndAddr = last.end
	p1.EndGroup = last.group
	p2.MessageAddr = last.message
	p2.P3().ty = last.Type
	p2.FieldAddr = last.field
	p2.P3().stack.ptr = p2.P3().stack.ptr.Add(1)

	return p1, p2, p2.P3().stack.ptr == p2.P3().stack.bottom
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func _PushMessage_log(p1 P1, p2 P2) (P1, P2) {
	p1.Log(
		p2, "new", "%#x, %v",
		p2.MessageAddr,
		p2.Message().Type(),
	)
	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func _push_log(p1 P1, p2 P2) (P1, P2) {
	p1.Log(p2, "push", "%v/%v/%v", p2.P3().stack.top, p2.P3().stack.ptr, p2.P3().stack.bottom)
	return p1, p2
}
