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

// hypertag is a code generator for generating a constant corresponding to
// a build constraint. It generates two files, one for the constraint being
// true and one for it being false, and a constant within indicating which is
// which.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"runtime/debug"
	"strings"
)

var (
	tag  = flag.String("tag", "", "the constraint to generate files for")
	name = flag.String("name", "", "the const to generate")
)

func write(file string, data *bytes.Buffer) error {
	defer data.Reset()
	fmt, err := format.Source(data.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(file, fmt, 0666)
}

func run() error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("could not parse buildinfo")
	}
	warning := fmt.Sprintf("// Code generated %s DO NOT EDIT.", info.Main.Path)

	pkg := os.Getenv("GOPACKAGE")
	buf := new(bytes.Buffer)
	for _, b := range []bool{false, true} {
		sigil := ""
		if !b {
			sigil = "!"
		}

		fmt.Fprintf(buf, "%s\n\n", warning)
		fmt.Fprintf(buf, "//go:build %s%s\n\n", sigil, *tag)
		fmt.Fprintf(buf, "package %s\n\n", pkg)
		fmt.Fprintf(buf, "// %s indicates whether the build tag %s is set.\n", *name, *tag)
		fmt.Fprintf(buf, "const %s = %v\n", *name, b)
		if err := write(fmt.Sprintf("tag_%s.%v.go", strings.ToLower(*name), b), buf); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%e\n", err)
		os.Exit(1)
	}
}
