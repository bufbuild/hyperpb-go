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

//go:build debug

// Package debug includes debugging helpers.
package debug

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"buf.build/go/hyperpb/internal/xflag"
	"github.com/timandy/routine"
)

// Enabled is true if the compiler is being built with the debug tag, which
// enables various debugging features.
const Enabled = true

var (
	debugPattern = xflag.Func("hyperpb.filter", "regexp to filter debug logs by", regexp.Compile)
	nocapture    = flag.Bool("hyperpb.nocapture", false, "disables capturing debug logs as test logs")
)

// Log prints debugging information to stderr.
//
// context is optional args for `fmt.Printf` that are printed before
// operation. This is useful for cases where you want to have
// information that identifies a set of operations that are related to appear
// before operation does.
func Log(context []any, operation string, format string, args ...any) {
	// Determine the package and file which called us.
	skip := 1
again:
	pc, file, line, _ := runtime.Caller(skip)

	fn := runtime.FuncForPC(pc)
	name := fn.Name()
	name = name[strings.LastIndex(name, ".")+1:]
	if strings.HasPrefix(name, "log") || strings.Contains(name, "Log") {
		skip++
		goto again
	}

	pkg := fn.Name()
	pkg = strings.TrimPrefix(pkg, "github.com/bufbuild/")
	pkg = strings.TrimPrefix(pkg, "hyperpb/internal/")
	pkg = pkg[:strings.Index(pkg, ".")]

	file = filepath.Base(file)

	buf := new(strings.Builder)

	_, _ = fmt.Fprintf(buf, "%s/%s:%d [g%04d", pkg, file, line, routine.Goid())
	if len(context) >= 1 {
		_, _ = fmt.Fprintf(buf, ", "+context[0].(string), context[1:]...)
	}
	_, _ = fmt.Fprintf(buf, "] %s: ", operation)
	_, _ = fmt.Fprintf(buf, format, args...)

	if *debugPattern != nil &&
		!(*debugPattern).MatchString(buf.String()) {
		return
	}

	t := tls.Get()
	if !*nocapture && t != nil {
		t.Log(buf.String())
		return
	}

	_, _ = buf.Write([]byte{'\n'})
	_, _ = os.Stderr.WriteString(buf.String())
	_ = os.Stderr.Sync()
}

// Assert panics if cond is false, but only in debug mode.
func Assert(cond bool, format string, args ...any) {
	if !cond {
		panic(fmt.Errorf("hyperpb: internal assertion failed: "+format, args...))
	}
}

// Value is a value of any type that only exists when the debug tag is
// enabled. When disabled, this struct is replaced with an empty struct.
type Value[T any] struct {
	x T
}

// Get returns a pointer to this value. Panics if not in debug mode.
func (v *Value[T]) Get() *T { return &v.x }
