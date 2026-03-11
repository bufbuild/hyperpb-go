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

package xsimd

import (
	"fmt"
	"reflect"
	"unsafe"

	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// Formatter returns a value wrapping v which formats it as if it were a slice.
func Formatter[T any](v vector[T]) any {
	return &format[T]{v}
}

type vector[T any] interface {
	StoreSlice([]T)
}

type format[T any] struct {
	v any
}

func (f format[T]) Format(s fmt.State, v rune) {
	r := reflect.ValueOf(f.v)
	d := reflect.New(r.Type())
	d.Elem().Set(r)

	n := int(r.Type().Size()) / layout.Size[T]()
	p := unsafe.Slice((*T)(d.UnsafePointer()), n)

	format := fmt.FormatString(s, v)
	fmt.Fprint(s, "[")
	for i, v := range p {
		if i > 0 {
			fmt.Fprint(s, " ")
		}
		fmt.Fprintf(s, format, v)
	}
	fmt.Fprint(s, "]")
}
