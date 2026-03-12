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

package varint

import "buf.build/go/hyperpb/internal/tdp/vm/internal/state"

//go:noinline
func ScalarSplit(p1 state.P1, p2 state.P2) (state.P1, state.P2, uint64) {
	return Scalar(p1, p2)
}

//go:noinline
func AVX32Split(p1 state.P1, p2 state.P2) (state.P1, state.P2, uint64) {
	return AVX32(p1, p2)
}

//go:noinline
func AVX64Split(p1 state.P1, p2 state.P2) (state.P1, state.P2, uint64) {
	return AVX64(p1, p2)
}
