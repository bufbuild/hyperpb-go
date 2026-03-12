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
)

// Format returns either v, or if v is a SIMD vector, a type which formats it
// as if it were a slice.
func Format(v any) any {
	t := reflect.TypeOf(v)
	if t == nil || t.Size() == 0 || t.PkgPath() != "simd/archsimd" {
		return t
	}
	return &format{v}
}

type format struct {
	v any
}

func (f format) Format(s fmt.State, v rune) {
	r := reflect.ValueOf(f.v)
	d := reflect.New(r.Type())
	d.Elem().Set(r)

	get, _ := r.Type().MethodByName("GetElem")
	elem := get.Type.Out(0)

	n := int(r.Type().Size() / elem.Size())
	a := reflect.NewAt(reflect.ArrayOf(n, elem), d.UnsafePointer())

	format := fmt.FormatString(s, v)
	fmt.Fprint(s, "[")
	for i := range n {
		if i > 0 {
			fmt.Fprint(s, " ")
		}
		fmt.Fprintf(s, format, a.Index(i))
	}
	fmt.Fprint(s, "]")
}
