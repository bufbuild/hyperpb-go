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

// Package xunsafe provides a more convenient interface for performing unsafe
// operations than Go's built-in package unsafe.
package xunsafe

import (
	"sync"
	"unsafe"

	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// NoCopy is a type that go vet will complain about having been moved.
//
// It does so by implementing [sync.Locker].
type NoCopy [0]sync.Mutex

// Int is any integer type.
type Int = layout.Int

// BitCast performs an unsafe bitcast from one type to another.
func BitCast[To, From any](v From) To {
	return *(*To)(unsafe.Pointer(&v))
}

// Ping reminds the processor that *p should be loaded into the data cache.
func Ping[P ~*E, E any](p P) {
	_ = ByteLoad[byte](NoEscape(p), 0)
}
