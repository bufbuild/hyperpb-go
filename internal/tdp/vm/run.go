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
	"fmt"
	"math"
	"math/bits"
	"runtime"
	"strings"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/bufbuild/fastpb/internal/debug"
	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/zc"
)

// Options is options for [Run].
type Options struct {
	// Max tries before hitting the tag table.
	MaxMisses int

	// Maximum recursion depth.
	MaxDepth int
}

// NewOptions returns the default settings for [Options].
func NewOptions() Options {
	return Options{
		MaxMisses: 4,
		MaxDepth:  1000,
	}
}

// Thunk is a callback for parsing a field. This is the "true" type of
// [tdp.FieldParser].Parser.
type Thunk func(P1, P2) (P1, P2)

// Run is the top-level entry point for message parsing.
func Run(m *dynamic.Message, data []byte, options Options) (err error) {
	if m.Shared.Src != nil {
		panic("fastpb: attempted to parse message using in-use Context")
	}

	if len(data) > math.MaxUint32 {
		return &ParseError{code: ErrorTooBig}
	}

	m.Shared.Lock.Lock()

	p3 := p3Pool.Get()
	p3.Options = options

	m.Shared.Src = conditionInputBuffer(data, false)
	m.Shared.Len = len(data)
	// The arena keeps m.context alive, so we don't need to KeepAlive src.

	if m.Shared.Len == 0 {
		return nil
	}

	stack := stackPool.Get()
	if cap(*stack) < p3.MaxDepth {
		*stack = make([]frame, p3.MaxDepth)
	}

	p3.stack.top = unsafe2.AddrOf(unsafe.SliceData(*stack))
	p3.stack.bottom = p3.stack.top.Add(p3.MaxDepth)

	p3.stack.ptr = p3.stack.bottom

	defer func() {
		if p3.err.code != 0 && recover() != nil {
			// Make a copy of the error, since pp will get re-used by a future
			// run of this function.
			parseErr := p3.err
			err = &parseErr

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
		stackPool.Put(stack)
		p3Pool.Put(p3)
		m.Shared.Lock.Unlock()
	}()

	p1 := P1{
		shared:  unsafe2.AddrOf(m.Shared),
		PtrAddr: unsafe2.AddrOf(m.Shared.Src),
	}
	p2 := P2{
		p3Addr: unsafe2.AddrOf(p3),
	}

	if debug.Enabled {
		p1.Log(p2, "start", "%p:%d, %p:%v",
			m.Shared.Src, m.Shared.Len, m.Type(), m.Type().Descriptor.FullName())
	}

	p1, p2 = p1.PushMessage(p2, m.Shared.Len, m)
	p2.Scratch = 0
	loop(p1, p2)
	return nil
}

// loop is the core parser loop. This function is not recursive.
func loop(p1 P1, p2 P2) {
	// Need this to match the ABI of returning from a thunk.
	p2.fieldAddr = unsafe2.AddrOf(p2.Field().NextOk)

	// This code is dynamically unreachable, but it forces Go to schedule
	// the fail block slightly differently in a way that is more in our favor
	// for branch scheduling.
	if p2.Scratch != 0 {
		goto truncated
	}

checkDone:
	if p1.Len() == 0 {
		goto pop
	}

number:
	{
		var tag tdp.Tag
		// The purpose of this block is to decode a varint without actually doing
		// any of the shifts to delete the sign bits. Instead:
		//
		// 1. Load n := 8 bytes from p1.b. Machinery elsewhere ensures this load
		//    will not segfault.
		//
		// 2. Determine how many bytes are in this varint using ctz(^n & K), where
		//    K has all of its sign bits set. This is the number of bit places up to
		//    the first cleared sign bit; it is always equal to 7 mod 8 unless no
		//    sign bits are present.
		//
		//    To ensure we can subtract off 7, we want to clear the highest sign bit
		//    of n. If it is set, which is a rare case, then we need to check for
		//    potential overflow in the next eight bytes.
		//
		//    This ensures that ctz(^n & K) is (8 - n) * 8 + 7, where n is the
		//    number of sign bits set in the word, up to 7.
		//
		// 3. We mask off all bytes past the first byte without a sign bit.
		//
		// 4. We set all of the sign bits to zero.
		//
		// This means that if a varint is over-long encoded, all of the extra
		// bytes turn into zeros. For example, if we have 0xaabbccddeeff0081
		// (litte-endian), we get a value of 15 for ctz(^n & K), so there are 6
		// irrelevant bytes past the 00. We mask those off and get 0x0081, and after
		// removing sign bits, we get 0x0001, which is the minimal encoding.

		// This block cannot be outlined into its own function for performance
		// reasons.

		// Load up to eight bytes for the varint (at most 5 will be used).
		tag = unsafe2.ByteLoad[tdp.Tag](p1.Ptr(), 0)
		// Flip all of the sign bits. This essentially clears the sign bits
		// of all of the varint bytes except the highest one's.
		tag ^= tdp.SignBits

		// Determine the number of cleared sign bits. This will tell us how
		// many bits to mask off as "irrelevant".
		//
		// In a varint (big-endian order) like 0a8b8c8d, this will be looking
		// at ctz(80000000) = 31. Thus we need to mask off 64 - 31 = 33 bits.
		tagBits := uint(bits.TrailingZeros64(uint64(tag) & tdp.SignBits))

		// tagMask will have its first (bits+1) bytes set to 0xff. Here, we shift
		// 0x100 to save on an add instruction on bytes.
		// The &63 is to ensure that Go does not generate a cmov to implement
		// the x<<64 == 0 case.
		tag &= (tdp.Tag(0b10) << ((tagBits - 1) & 63)) - 1
		// No need to strip the sign bits, the ^= above already did that.

		// Consume the tag.
		tagBytes := (tagBits / 8) + 1
		p1.Log(p2, "number", "%v (%d bytes)", tag, tagBytes)
		if tagBytes > uint(p1.Len()) {
			goto truncated
		}
		p1 = p1.Advance(int(tagBytes))

		p2.Scratch = uint64(tag)
		if tagBits < 64 {
			goto field
		}

		p1, p2 = checkLargeVarint(p1, p2)
	}

field:
	{
		tries := p2.P3().MaxMisses
		tag := tdp.Tag(p2.Scratch)
		for {
			p1.Log(p2, "try", "%v, %v, %v", tag, tries, p2.Field())

			if debug.Enabled {
				// Run the GC frequently in debug mode to smoke out bugs where
				// we've left a stack root unmarked.
				runtime.GC()
			}

			if p2.Field().Tag == tag {
				// Try to keep the Context in L1 cache by loading a byte from it
				// before every thunk. This makes sure that short thunks that
				// do not allocate any memory do not cause it to fall out of
				// the cache, slowing down memory allocations due to the need
				// to pull the arena's internal pointers from L2 cache.
				unsafe2.Ping(p1.Shared())

				thunk := (*unsafe2.PC[Thunk])(&p2.Field().Parse)
				p1.Log(p2, "call", "%v, %#x", debug.Func(thunk.Get()), p2.fieldAddr)
				p1, p2 = thunk.Get()(p1, p2)
				p1.Log(p2, "ret", "%v, %#x", debug.Func(thunk.Get()), p2.fieldAddr)

				p2.fieldAddr = unsafe2.AddrOf(p2.Field().NextOk)

				p2.Scratch = 0 // Make sure no one relies on this being preserved.
				goto checkDone
			}

			p2.fieldAddr = unsafe2.AddrOf(p2.Field().NextErr)

			tries--
			if tries > 0 {
				continue
			}

			break
		}

		p1.Log(p2, "miss", "%v", tag)
		// Check for tag overflow.
		if tag > maxFieldTag {
			p1.Fail(p2, ErrorOverflow)
		}

		// Finish parsing number into a varint.
		// This is a manual inlining of tag.decode.
		mask := tdp.Tag(0x7f)
		i := 0
		// Repeated 5 times.
		var tag2 uint64
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		// Repeat end.
		p1.Log(p2, "decode number", "%d", tag2)
		_, _ = i, mask

		// Check if we know about this field number.
		p1, p2, tag2 = p1.byTag(p2, tag2)
		if p2.Field() != nil {
			p1.Log(p2, "goto field", "%d", tag2)
			goto field
		}

		// Skip this field, and keep skipping fields until we find a field
		// number we recognize.
		for {
			p1, p2 = handleUnknown(p1, p2, tag2)
			if p1.Len() == 0 {
				goto pop
			}

			p2.Scratch = uint64(p1.PtrAddr)
			p1, p2, tag2 = p1.Varint(p2)
			if tag2 > math.MaxInt32<<3 {
				p1.Fail(p2, ErrorOverflow)
			}

			p1, p2, tag2 = p1.byTag(p2, tag2)
			if p2.Field() != nil {
				p1.PtrAddr = unsafe2.Addr[byte](p2.Scratch)
				p1.Log(p2, "goto number", "%d", tag2)
				goto number
			}
		}
	}

pop:
	{
		if debug.Enabled {
			p1.Log(
				p2, "finish", "%v, ty: %p:%s %v",
				p2.messageAddr,
				p2.Message().Type(),
				p2.Message().Type().Descriptor.FullName(),
				p2.Message().Type(),
			)
		}

		var done bool
		p1, p2, done = p1.pop(p2)
		if done {
			return
		}

		// Only need to pop once: message() makes sure to avoid creating multiple
		// frames with the same end pointer.

		goto number
	}

truncated:
	// Route all failures in loop() here to force Go to schedule them as the
	// cold side of the branch leading to it.
	p1.Fail(p2, ErrorTruncated)
}

// handleUnknown handles an handleUnknown field with the given tag. Outlined to improve
// branch scheduling in [loop].
//
//go:noinline
func handleUnknown(p1 P1, p2 P2, tag2 uint64) (P1, P2) {
	if tag2&^0xffffffff != 0 {
		p1.Fail(p2, ErrorOverflow)
	}

	// Rewind the stream to find the start offset of this field. We can do this
	// because we know that tag2 is nonzero, so first we can trim off leading
	// zero bytes for an over-long varint, and then skip back the minimum
	// number of bytes needed to store tag2.
	start := p1.PtrAddr
	start--
	for *start.AssertValid()&0x7f == 0 {
		start--
	}
	start = start.Add(1 - protowire.SizeVarint(tag2))

	num := protowire.Number(tag2 >> 3)
	ty := protowire.Type(tag2 & 0b111)
	if num == 0 {
		p1.Fail(p2, ErrorFieldNumber)
	}

	m := protowire.ConsumeFieldValue(num, ty, p1.Buf())
	p1.Log(p2, "unknown", "%d, %d, %d bytes", num, ty, m)
	if m < 0 {
		p1.Fail(p2, ErrorCode(-m))
	}
	p1 = p1.Advance(m)

	if !p2.Type().DiscardUnknown {
		r := zc.New(p1.Src(), start.AssertValid(), int(p1.PtrAddr-start))
		cold := p2.Message().MutableCold()
		if cold.Unknown.Len() > 0 {
			last := unsafe2.Add(cold.Unknown.Ptr(), cold.Unknown.Len()-1)
			if r.Start() == last.End() {
				*last = zc.NewRaw(last.Start(), last.Len()+r.Len())
				return p1, p2
			}
		}
		cold.Unknown = cold.Unknown.AppendOne(p1.Arena(), r)
	}

	return p1, p2
}

// checkLargeVarint is part of the varint decoder in [loop]. Outlined because
// this function is almost never called, improving code locality.
//
//go:noinline
func checkLargeVarint(p1 P1, p2 P2) (P1, P2) {
	// This is a very large varint. We need to check the next two words.
	// This is a slow path, so we can afford to not be efficient.
	switch unsafe2.Load(p1.Ptr(), -1) {
	case 0x00:
	case 0x80:
		if unsafe2.Load(p1.Ptr(), 0) != 0x00 {
			p1.Fail(p2, ErrorOverflow)
		}
		p1 = p1.Advance(1)
	default:
		p1.Fail(p2, ErrorOverflow)
	}

	return p1, p2
}
