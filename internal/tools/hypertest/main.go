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

// xtest is a helper for running tests that adds a few useful features:
//
// 1. Benchmark output as CSV and as a table.
// 2. Running tests on remote hosts over SSH.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"buf.build/go/hyperpb/internal/xerrors"
)

var (
	goTool   = flag.String("go-tool", "go", "path to the go tool")
	pkgs     = flag.String("p", ".", "test target to run")
	output   = flag.String("o", "", "output directory to use; must be set")
	tags     = flag.String("tags", "", "build tags to pass to go build")
	profile  = flag.Bool("profile", false, "whether to collect CPU profiles")
	remote   = flag.String("remote", "", "SSH remote to run tests at")
	checkptr = flag.Bool("checkptr", false, "build with checkptr (crappy asan) instrumentation")
	race     = flag.Bool("race", false, "build with -race")
	unopt    = flag.Bool("unopt", false, "build with optimizations turned off")

	benchCsv   = flag.String("csv", "", "file for benchmark csv output")
	benchTable = flag.String("table", "", "file for benchmark table output")
)

func open(path string) (*os.File, func(), error) {
	if path == "-" {
		return os.Stdout, func() {}, nil
	}

	f, err := os.Create(path)
	return f, func() { _ = f.Close() }, err
}

func run() error {
	flag.Parse()
	if *output == "" {
		fmt.Println("must provide set -o")
		os.Exit(1)
	}

	r := &runner{
		tool:     *goTool,
		pkgs:     *pkgs,
		output:   *output,
		tags:     *tags,
		profile:  *profile,
		checkptr: *checkptr,
		race:     *race,
		unopt:    *unopt,
		args:     flag.Args(),
	}

	tests, err := r.build()
	if err != nil {
		return err
	}

	var output string
	if *remote == "" {
		output, err = r.runLocally(tests)
	} else {
		output, err = r.runOverSSH(*remote, tests)
	}
	if err != nil {
		return err
	}

	if *benchCsv == "" && *benchTable == "" {
		return nil
	}

	benchmarks := parseBenchmarkOutput(output)
	if *benchCsv != "" {
		f, close, err := open(*benchCsv)
		if err != nil {
			return err
		}
		defer close()

		if err := benchmarks.toCSV(f); err != nil {
			return err
		}
	}
	if *benchTable != "" {
		f, close, err := open(*benchTable)
		if err != nil {
			return err
		}
		defer close()

		if err := benchmarks.toMarkdown(f); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		if exit, ok := xerrors.As[*exec.ExitError](err); ok {
			fmt.Printf("%s\n", exit.Stderr)
			os.Exit(exit.ExitCode())
		}
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
