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

package inlinetest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertInlined returns whether the compiler is willing to inline the given
// symbols in the package being tested.
//
// The symbols must be either a single identifier or of the form Type.Method.
// Pointer-receiver methods should not use the (*Type).Method syntax.
func AssertInlined(t *testing.T, symbols ...string) {
	t.Helper()
	for _, symbol := range symbols {
		_, ok := inlined[symbol]
		assert.True(t, ok, "%s is not inlined", symbol)
	}
}

var inlined = make(map[string]struct{})

func init() {
	if !testing.Testing() {
		panic("inlinetest: cannot import inlinetest except in a test")
	}

	// This is based on a pattern of tests appearing in several places in Go's
	// standard library.
	tool := "go"
	if env, ok := os.LookupEnv("GO"); ok {
		tool = env
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		panic(errors.New("inlinetest: could not read build info"))
	}

	//nolint:gosec
	out, err := exec.Command(
		tool,
		"build",
		"--gcflags=-m", // -m records optimization decisions.
		strings.TrimSuffix(info.Path, ".test"),
	).CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("inlinetest: go build failed: %w, %s", err, out))
	}

	remarkRe := regexp.MustCompile(`(?m)^\./\S+\.go:\d+:\d+: can inline (.+?)$`)
	ptrRe := regexp.MustCompile(`\(\*(.+)\)\.`)
	for _, match := range remarkRe.FindAllSubmatch(out, -1) {
		match := string(match[1])
		match = ptrRe.ReplaceAllString(match, "$1.")
		inlined[match] = struct{}{}
	}
}
