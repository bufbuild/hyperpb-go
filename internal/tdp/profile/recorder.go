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

package profile

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	_ "unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/stats"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xsync"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// hyperpbMessage is the itab for *hyperpb.Message.
//
// This is connected to the root package via linkname.
//
//go:linkname hyperpbMessage
var hyperpbMessage xunsafe.Type

// Recorder is a profile recorder, which walks a message to record information
// about its fields after a successful parse.
type Recorder struct {
	library  *tdp.Library
	profiles xsync.Map[*tdp.Field, *metrics]
}

// NewRecorder returns a new recorder for the given type library.
func NewRecorder(library *tdp.Library) *Recorder {
	return &Recorder{library: library}
}

// Record records information from the given message. This function may be
// called concurrently from multiple goroutines.
func (r *Recorder) Record(m *dynamic.Message) {
	if r.library != m.Type().Library {
		panic("hyperpb: attempted to record message from incompatible type library")
	}

	for fd, pv := range m.Range {
		ty, _ := r.library.Type(fd.ContainingMessage())
		f := ty.ByDescriptor(fd)
		debug.Assert(f != nil, "invalid field in Record()")

		metrics, _ := r.profiles.LoadOrStore(f, func() *metrics {
			return &metrics{
				desc:  fd,
				count: *stats.NewMedian(1 << 12),
			}
		})
		metrics.parse.Record(1)

		if m := xprotoreflect.UnsafeUnwrap(pv, hyperpbMessage); m != nil {
			r.Record((*dynamic.Message)(m))
			continue
		}

		if l := xprotoreflect.List(pv); l.IsValid() {
			metrics.count.Record(float64(l.Len()))
			for i := range l.Len() {
				pv := l.Get(i)
				m := xprotoreflect.UnsafeUnwrap(pv, hyperpbMessage)
				if m == nil {
					break // None of these are going to be messages.
				}
				r.Record((*dynamic.Message)(m))
			}
			continue
		}

		if m := xprotoreflect.Map(pv); m.IsValid() {
			metrics.count.Record(float64(m.Len()))
			for _, pv := range m.Range {
				m := xprotoreflect.UnsafeUnwrap(pv, hyperpbMessage)
				if m == nil {
					break // None of these are going to be messages.
				}
				r.Record((*dynamic.Message)(m))
			}
			continue
		}
	}
}

// ForField implements [Profile].
func (r *Recorder) ForField(site Site) Field {
	profile := site.DefaultProfile()

	ty, _ := r.library.Type(site.Field.ContainingMessage())
	if ty == nil {
		return profile
	}
	f := ty.ByDescriptor(site.Field)
	if f == nil {
		return profile
	}

	m, ok := r.profiles.Load(f)
	if !ok {
		profile.DecodeProbability = 0 // We never saw it!
		return profile
	}

	profile.DecodeProbability = m.parse.Get()
	profile.ExpectedCount = int(m.count.Get())

	return profile
}

// Dump dumps this recorder's profile.
func (r *Recorder) Dump() string {
	var ms []*metrics
	for _, v := range r.profiles.All() {
		ms = append(ms, v)
	}
	slices.SortFunc(ms, func(a, b *metrics) int {
		return cmp.Compare(a.desc.FullName(), b.desc.FullName())
	})

	out := new(strings.Builder)
	for _, m := range ms {
		fmt.Fprintf(out,
			"%s: parse: %v, count: %v\n",
			m.desc.FullName(), m.parse.Get(), m.count.Get(),
		)
	}
	return out.String()
}

// metrics are metrics that [Recorder] records.
type metrics struct {
	desc  protoreflect.FieldDescriptor
	parse stats.Mean
	count stats.Median
}
