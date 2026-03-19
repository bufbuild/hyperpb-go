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

package prototest

import (
	"bytes"
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Equal validates that two Protobuf messages have the same observable value.
func Equal(t testing.TB, expect, got proto.Message) {
	t.Helper()
	e := &equal{TB: t}

	panicked := true
	defer func() {
		if panicked {
			t.Errorf("panicked at %s", e.formatPath())
		}
	}()

	e.message(expect.ProtoReflect(), got.ProtoReflect(), true)
	panicked = false
}

type equal struct {
	testing.TB
	path []any
}

func (e *equal) any(v1, v2 any, rec bool) {
	e.Helper()

	if v, ok := v1.(protoreflect.Value); ok {
		v1 = v.Interface()
	}
	if v, ok := v2.(protoreflect.Value); ok {
		v2 = v.Interface()
	}

	switch a := v1.(type) {
	case string:
		b, ok := v2.(string)
		switch {
		case !ok:
			e.wrongType(a, v2)

		case a != b:
			e.fail("expected %q:`%x`, got %q:`%x` (%T)", a, a, b, b, b)
		}
	case []byte:
		b, ok := v2.([]byte)
		switch {
		case !ok:
			e.wrongType(a, v2)
		case !bytes.Equal(a, b):
			e.fail("expected %q:`%x`, got %q:`%x` (%T)", a, a, b, b, b)
		}

	case protoreflect.Message:
		b, ok := v2.(protoreflect.Message)
		if !ok {
			e.fail("expected protoreflect.Message, got %T", v2)
		}
		e.message(a, b, rec)

	case protoreflect.List:
		b, ok := v2.(protoreflect.List)
		if !ok {
			e.fail("expected protoreflect.List, got %T", v2)
		}
		e.list(a, b, rec)

	case protoreflect.Map:
		b, ok := v2.(protoreflect.Map)
		if !ok {
			e.fail("expected protoreflect.Map, got %T", v2)
		}
		e.map_(a, b, rec)

	default:
		if reflect.TypeOf(v1) != reflect.TypeOf(v2) {
			e.wrongType(v1, v2)
		}

		// Compare byte-wise. We want to get exact comparisons for floats, even
		// NaN payloads.
		b1 := xunsafe.AnyBytes(v1)
		b2 := xunsafe.AnyBytes(v2)

		if bytes.Equal(b1, b2) {
			break
		}

		b1 = slices.Clone(b1)
		b2 = slices.Clone(b2)
		slices.Reverse(b1)
		slices.Reverse(b2)

		diff := slices.Clone(b1)
		for i := range diff {
			diff[i] ^= b2[i]
		}

		e.fail("expected %v:0x%x, got %v:0x%x, diff: %x (%T)", v1, b1, v2, b2, diff, v2)
	}
}

func (e *equal) message(a, b protoreflect.Message, rec bool) {
	e.Helper()

	if a.Descriptor() != b.Descriptor() {
		e.fail("expected %p:%v, got %p:%v",
			a.Descriptor(), a.Descriptor().FullName(),
			b.Descriptor(), b.Descriptor().FullName())
		return
	}

	if a.IsValid() != b.IsValid() {
		e.fail("unequal IsValid: want %v, got %v", a.IsValid(), b.IsValid())
	}

	if !rec && !a.IsValid() && !b.IsValid() {
		return
	}

	// Can't just compare for equality, since go protobuf actually re-encodes
	// each unknown field minimally! This is not actually necessary to match
	// the contract of unknown fields.
	transcode := func(b []byte) []byte {
		empty := new(emptypb.Empty)
		_ = proto.Unmarshal(b, empty)
		return empty.ProtoReflect().GetUnknown()
	}

	if !bytes.Equal(transcode(a.GetUnknown()), transcode(b.GetUnknown())) {
		e.fail("unequal unknown fields: want `%x`, got `%x`", a.GetUnknown(), b.GetUnknown())
	}

	d := a.Descriptor()
	fds := d.Fields()
	for i := range fds.Len() {
		fd := fds.Get(i)
		e.push(fd.Name(), func() {
			e.Helper()
			if a.Has(fd) != b.Has(fd) {
				e.fail("unequal has: want %v, got %v", a.Has(fd), b.Has(fd))
			}
			e.any(a.Get(fd), b.Get(fd), a.IsValid() || b.IsValid())
		})
	}

	ods := d.Oneofs()
	for i := range ods.Len() {
		od := ods.Get(i)
		e.push(od.Name(), func() {
			if a.WhichOneof(od) != b.WhichOneof(od) {
				e.fail("unequal which: want %v, got %v", a.WhichOneof(od), b.WhichOneof(od))
			}
		})
	}
}

func (e *equal) list(a, b protoreflect.List, rec bool) {
	e.Helper()
	// Compare the common prefix.
	for i := range min(a.Len(), b.Len()) {
		e.push(i, func() {
			e.Helper()
			e.any(a.Get(i), b.Get(i), rec)
		})
	}

	if a.Len() != b.Len() {
		e.fail("unequal lengths: want %d, got %d", a.Len(), b.Len())
	}
}

func (e *equal) map_(a, b protoreflect.Map, rec bool) {
	e.Helper()
	if (a == nil) != (b == nil) {
		return
	}

	// Make a set of all of the keySet.
	keySet := make(map[any]struct{})
	for k := range a.Range {
		keySet[k.Interface()] = struct{}{}
	}
	for k := range b.Range {
		keySet[k.Interface()] = struct{}{}
	}

	// Next, sort the keys.
	keys := make([]protoreflect.MapKey, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, protoreflect.ValueOf(k).MapKey())
	}
	slices.SortFunc(keys, func(x, y protoreflect.MapKey) int {
		a, b := x.Interface(), y.Interface()
		ka, kb := reflect.TypeOf(a).Kind(), reflect.TypeOf(b).Kind()
		if d := cmp.Compare(ka, kb); d != 0 {
			return d
		}

		//nolint:errcheck // Already checked above by comparing reflect.Kind.
		switch a := a.(type) {
		case bool:
			b := b.(bool)
			switch {
			case a == b:
				return 0
			case a:
				return 1
			default:
				return -1
			}
		case int32:
			return cmp.Compare(a, b.(int32))
		case int64:
			return cmp.Compare(a, b.(int64))
		case uint32:
			return cmp.Compare(a, b.(uint32))
		case uint64:
			return cmp.Compare(a, b.(uint64))
		case string:
			return cmp.Compare(a, b.(string))
		default:
			panic("unreachable")
		}
	})

	// Now, check that the value for each key is the same.
	for _, k := range keys {
		e.push(k.Interface(), func() {
			e.Helper()
			e.any(a.Get(k), b.Get(k), rec)
		})
	}
}

func (e *equal) push(v any, f func()) {
	e.Helper()
	e.path = append(e.path, v)
	f()
	e.path = e.path[:len(e.path)-1]
}

func (e *equal) wrongType(a, b any) {
	e.Helper()
	e.fail("expected %T, got %T", a, b)
}

func (e *equal) fail(format string, args ...any) {
	e.Helper()
	e.Errorf("failure at %s: %v", e.formatPath(), debug.Fprintf(format, args...))
}

func (e *equal) formatPath() string {
	if len(e.path) == 0 {
		return "."
	}

	buf := new(strings.Builder)
	for _, e := range e.path {
		switch e := e.(type) {
		case protoreflect.Name, protoreflect.FullName:
			fmt.Fprintf(buf, ".%v", e)
		case string:
			fmt.Fprintf(buf, "[%q]", e)
		default:
			fmt.Fprintf(buf, "[%v]", e)
		}
	}

	return buf.String()
}
