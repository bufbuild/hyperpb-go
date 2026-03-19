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

package dynamic

import (
	"sync"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Shared is state that is shared by all messages in a particular tree of
// messages.
//
// A zero Shared is ready to use.
type Shared struct {
	_ xunsafe.NoCopy

	// Shared is the only memory not allocated on the arena.
	arena arena.Arena
	lib   *tdp.Library

	Src *byte // Needs to be a pointer to root the source buffer.
	Len int

	// Must *not* be a pointer, it's one-past-the-end. This points to one past the
	// furthest address we are guaranteed to be able to load from.
	End xunsafe.Addr[byte]

	// Synchronizes calls to startParse() with this context.
	Lock sync.Mutex

	// Off-arena memory which holds arena pointers to "Cold" parts of a message.
	Cold []*Cold
}

// Arena returns the message tree's arena.
func (s *Shared) Arena() *arena.Arena {
	return &s.arena
}

// Library returns the message tree's library.
func (s *Shared) Library() *tdp.Library {
	return s.lib
}

// New allocates a new message in this context.
func (s *Shared) New(ty *tdp.Type) *Message {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	switch s.lib {
	case nil:
		s.lib = ty.Library
	case ty.Library:
		break
	default:
		panic("hyperpb: attempted to mix messages from different hyperpb.Library pointers")
	}

	data := s.arena.Alloc(int(ty.Size))
	m := xunsafe.Cast[Message](data)
	xunsafe.StoreNoWB(&m.Shared, s)
	m.TypeOffset = uint32(xunsafe.ByteSub(ty, s.lib.Base))
	m.ColdIndex = -1
	return m
}

// Free releases any resources held by this context, allowing them to be re-used.
//
// Any messages previously parsed using this context must not be reused.
func (s *Shared) Free() {
	s.arena.Free()
	s.lib = nil
	s.Src = nil

	clear(s.Cold)
	s.Cold = s.Cold[:0]
}
