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

package xsync

import (
	"math"
	"sync/atomic"
)

// AtomicFloat64 is an atomic float64 variable.
type AtomicFloat64 atomic.Uint64

// Load atomically loads the wrapped float64.
func (x *AtomicFloat64) Load() float64 {
	return math.Float64frombits((*atomic.Uint64)(x).Load())
}

// Store atomically stores the passed float64.
func (x *AtomicFloat64) Store(val float64) {
	(*atomic.Uint64)(x).Store(math.Float64bits(val))
}

// Swap atomically stores the given float64 and returns the old
// value.
func (x *AtomicFloat64) Swap(val float64) (old float64) {
	return math.Float64frombits((*atomic.Uint64)(x).Swap(math.Float64bits(val)))
}

// BitwiseCompareAndSwap atomically stores the given float64 if x currently holds a float with
// the same bit-pattern as val.
//
// That is to say, this does *not* perform a floating-point comparison!
func (x *AtomicFloat64) BitwiseCompareAndSwap(old, new float64) (swapped bool) {
	return (*atomic.Uint64)(x).CompareAndSwap(math.Float64bits(old), math.Float64bits(new))
}

// Add atomically adds delta to this value and returns the result.
//
// This will not compile down to a single instruction, because no one provides
// that. Instead, this just does a CAS loop.
func (x *AtomicFloat64) Add(delta float64) (new float64) {
retry:
	old := x.Load()
	new = old + delta
	if !x.BitwiseCompareAndSwap(old, new) {
		goto retry
	}

	return new
}
