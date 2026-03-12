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

//go:build !amd64

package varint

import "buf.build/go/hyperpb/internal/tdp/vm/internal/impl"

func AVX32(p1 impl.P1, p2 impl.P2) (impl.P1, impl.P2, uint64) /*{
	// Unimplemented, calling this function causes a linker error.
}*/

func AVX64(p1 impl.P1, p2 impl.P2) (impl.P1, impl.P2, uint64) /*{
	// Unimplemented, calling this function causes a linker error.
}*/
