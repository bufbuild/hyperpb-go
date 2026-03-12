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
	"fmt"
	"strings"
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/vm/internal/options"
	"buf.build/go/hyperpb/internal/tdp/vm/memory"
	"buf.build/go/hyperpb/internal/xsync"
	"buf.build/go/hyperpb/internal/xunsafe"
)

var (
	stackPool = xsync.Pool[[]frame]{}
	p3Pool    = xsync.Pool[P3]{
		Reset: func(pp *P3) { *pp = P3{} },
	}
)

// New creates new VM state.
//
// Make sure to defer [Done] after calling this function.
func New(data []byte, m *dynamic.Message, options *options.Options) (P1, P2) {
	m.Shared.Lock.Lock()

	p3 := p3Pool.Get()
	p3.Options = *options

	data = memory.RelocatePageBoundary(data, !p3.AllowAlias, 15)
	m.Shared.Src = unsafe.SliceData(data)
	m.Shared.Len = len(data)
	// The arena keeps m.context alive, so we don't need to KeepAlive src.

	p3.frames = stackPool.Get()
	if cap(*p3.frames) < p3.MaxDepth {
		*p3.frames = make([]frame, p3.MaxDepth)
	}

	p3.stack.top = xunsafe.AddrOf(unsafe.SliceData(*p3.frames))
	p3.stack.bottom = p3.stack.top.Add(p3.MaxDepth)

	p3.stack.ptr = p3.stack.bottom

	p1 := P1{
		shared:  xunsafe.AddrOf(m.Shared),
		PtrAddr: xunsafe.AddrOf(m.Shared.Src),
	}
	p2 := P2{
		p3Addr: xunsafe.AddrOf(p3),
	}

	if debug.Enabled {
		p1.Log(p2, "start", "%p:%d `%x`, %p:%v",
			m.Shared.Src, m.Shared.Len, data, m.Type(), m.Type().Descriptor.FullName())
	}

	p1, p2 = p1.SetScratch(p2, uint64(m.Shared.Len))
	p1, p2 = p1.PushMessage(p2, m)
	p1, p2 = p1.SetScratch(p2, 0)

	return p1, p2
}

// Done should be called in a defer immediately after [New].
func (p3 *P3) Done(shared *dynamic.Shared, err *error) {
	if tdp.GetCode(p3.err) != tdp.ErrorOk && recover() != nil {
		// Make a copy of the error, since pp will get re-used by a future
		// run of this function.
		parseErr := p3.err
		*err = &parseErr

		if debug.Enabled {
			buf := new(strings.Builder)
			for _, frame := range p3.stackSlice() {
				fmt.Fprintf(buf, "- %#v\n", frame)
			}

			debug.Log(nil, "fail",
				"%v\n"+
					"trace to fail() call:\n%s"+
					"stack:\n%s", err, debug.Stack(6), buf)
		}
	}

	// These would all normally go in their own defers, but having a single
	// defer is noticeably faster.
	stackPool.Put(p3.frames)
	p3.frames = nil
	p3Pool.Put(p3)
	shared.Lock.Unlock()
}
