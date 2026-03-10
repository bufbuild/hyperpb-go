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

package vm

import (
	"buf.build/go/hyperpb/internal/xunsafe"
)

// pageBoundary is the alignment of the smallest physical memory page on any
// system we support (4K). If we are allowed to load memory from any address in
// a page, we assume that loading (and discarding) memory from anywhere else is
// also ok.
const pageBoundary = 0x1000

// RelocatePageBoundary ensures that it is always possible to read nine bytes
// beyond the end of data. This allows us to elide virtually all bounds checks
// in the parser, since it will only ever look ahead at most nine bytes (to
// parse a rare ten-byte varint).
//
// This function accomplishes this by checking that loading nine bytes from the
// end of data does not cross a 4K page boundary. If it does not, it means that
// we can always load past the end a little bit, because page protection is not
// more granular than that on any platform we care about. If this condition is
// not met, we copy the slice in such a way as to force this condition to be
// met.
//
// If forceCopy is set, this copy is performed unconditionally.
//
// Exported for use by benchmarks.
//
//go:nosplit
func RelocatePageBoundary(data []byte, force bool) []byte {
	if !force {
		// Check if there is capacity to spare.
		if cap(data)-len(data) >= 15 {
			return data
		}

		// If not, we need to check if there is a page boundary beyond this
		// slice.
		if xunsafe.EndOf(data).Padding(pageBoundary) >= 15 {
			// All good, we have nine or more bytes ahead of us before the next
			// page boundary.
			return data
		}
	}

	// Copy to a new slice with just enough capacity.
	return append(data[:cap(data)], make([]byte, 9)...)[:len(data):cap(data)]
}
