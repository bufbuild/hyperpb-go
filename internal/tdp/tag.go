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

package tdp

import (
	"fmt"
	"math/bits"

	"google.golang.org/protobuf/encoding/protowire"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Tag is a specially-formatted tag for the parser.
//
// The tag is formatted in the way it would be when encoded in a Protobuf
// message, but with the high bit of each byte cleared.
type Tag uint64

// encode encodes this field tag from the given number and type.
func EncodeTag(n protowire.Number, t protowire.Type) Tag {
	var tag Tag
	protowire.AppendTag(xunsafe.Bytes(&tag)[:0], n, t)
	tag &^= SignBits
	return tag
}

// Decode decodes this field tag into a number and a type.
func (t Tag) Decode() uint64 {
	var tag uint64
	mask := uint64(0x7f)
	i := 0

	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++

	_, _ = i, mask
	return tag
}

// Format implements [fmt.Formatter].
func (t Tag) Format(s fmt.State, verb rune) {
	if ^t == 0 {
		fmt.Fprint(s, "</>")
		return
	}

	v := t.Decode()
	n, ty := protowire.DecodeTag(v)
	debug.Fprintf("%#x:%d:%d", uint64(t), n, ty).Format(s, verb)
}

// Returns whether this tag is "too large", i.e., if it has more than 32 bits
// when decoded.
func (t Tag) Overflows() bool {
	return bits.LeadingZeros64(uint64(t)) < (64 - 32 - 4)
}
