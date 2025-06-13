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

package fastpb

import (
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Shared is state that is shared by all messages in a particular tree of
// messages.
//
// A zero context is ready to use.
type Shared struct {
	impl dynamic.Shared
}

// New allocates a new message in this context.
func (s *Shared) New(ty *Type) *Message {
	if s == nil {
		s = new(Shared)
	}

	// Previously, this code was here:
	//
	// // Easy mistake to make: the memory allocated in alloc() contains no
	// // pointers, so even though ty is "reachable" through m, it's not reachable
	// // from the GC's perspective, so we need to mark it as alive here.
	// //
	// // This implicitly marks all other types reachable from ty as alive, meaning
	// // we only need to do this for top-level calls to New().
	// c.arena.KeepAlive(ty)
	//
	// It is now redundant, because Context stores ty.Library(). The comment is
	// kept for posterity about a nasty bug.

	return newMessage(s.impl.New(&ty.impl))
}

// Free releases any resources held by this context, allowing them to be re-used.
//
// Any messages previously parsed using this context must not be reused.
func (s *Shared) Free() { s.impl.Free() }

// newShared wraps an internal Shared pointer.
func newShared(s *dynamic.Shared) *Shared {
	return unsafe2.Cast[Shared](s)
}
