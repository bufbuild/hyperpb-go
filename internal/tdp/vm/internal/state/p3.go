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

package state

import (
	"fmt"
	"strings"
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/vm/memory"
	"buf.build/go/hyperpb/internal/tdp/vm/options"
	"buf.build/go/hyperpb/internal/xsync"
	"buf.build/go/hyperpb/internal/xunsafe"
)

var (
	stackPool = xsync.Pool[[]Frame]{}
	p3Pool    = xsync.Pool[P3]{
		Reset: func(pp *P3) { *pp = P3{} },
	}
)

// P3 is parser state that is passed behind a pointer.
type P3 struct {
	_ xunsafe.NoCopy

	Err   tdp.Error
	Stack struct {
		Ptr         xunsafe.Addr[Frame]
		Top, Bottom xunsafe.Addr[Frame]
	}

	Type xunsafe.Addr[tdp.TypeParser]
	options.Options

	Frames *[]Frame
}

// Frame is a recursion frame for the parser.
type Frame struct {
	End     xunsafe.Addr[byte]
	Group   tdp.Tag
	Message xunsafe.Addr[dynamic.Message]
	Type    xunsafe.Addr[tdp.TypeParser]
	Field   xunsafe.Addr[tdp.FieldParser]
}

func NewP3(data []byte, shared *dynamic.Shared, options *options.Options) *P3 {
	shared.Lock.Lock()

	p3 := p3Pool.Get()
	p3.Options = *options

	data = memory.RelocatePageBoundary(data, !p3.AllowAlias)
	shared.Src = unsafe.SliceData(data)
	shared.Len = len(data)
	// The arena keeps m.context alive, so we don't need to KeepAlive src.

	p3.Frames = stackPool.Get()
	if cap(*p3.Frames) < p3.MaxDepth {
		*p3.Frames = make([]Frame, p3.MaxDepth)
	}

	p3.Stack.Top = xunsafe.AddrOf(unsafe.SliceData(*p3.Frames))
	p3.Stack.Bottom = p3.Stack.Top.Add(p3.MaxDepth)

	p3.Stack.Ptr = p3.Stack.Bottom

	return p3
}

func (p3 *P3) Done(shared *dynamic.Shared, err *error) {
	if tdp.GetCode(p3.Err) != tdp.ErrorOk && recover() != nil {
		// Make a copy of the error, since pp will get re-used by a future
		// run of this function.
		parseErr := p3.Err
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
	stackPool.Put(p3.Frames)
	p3.Frames = nil
	p3Pool.Put(p3)
	shared.Lock.Unlock()
}
