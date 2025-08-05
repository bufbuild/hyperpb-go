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

package printer

import (
	"runtime"
	"strings"
)

var root = func() string {
	_, f, _, ok := runtime.Caller(1)
	if !ok {
		return ""
	}

	f, ok = strings.CutSuffix(f, "internal/gencode/printer/caller.go")
	if !ok {
		panic("hyperpb: caller.go appears to have moved")
	}

	return f
}()

// Caller returns the file and line of its caller, adjusted for pretty-printing.
//
// Returns "", 0 if the binary was stripped.
func Caller() (file string, line int) {
	_, file, line, _ = runtime.Caller(1)
	file = strings.TrimPrefix(file, root)
	return file, line
}
