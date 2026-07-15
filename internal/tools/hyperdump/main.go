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

// hyperdump cleans up the output of go tool objdump into something readable.
package main

// Based on the Go integration in compiler-explorer (aka godbolt.com). Translated
// partly from TypeScript.
//
// See https://github.com/compiler-explorer/compiler-explorer/blob/gh-15542/lib/compilers/golang.ts#L55.
//
// BSD 2-Clause License
//
// Copyright (c) 2012-2022, Compiler Explorer Authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"buf.build/go/hyperpb/internal/xerrors"
)

var (
	jump = regexp.MustCompile(`^(J|B|CB|TB|CMPB|CMPUB).*`)
	text = regexp.MustCompile(`^TEXT\s+(.+?)\(SB\)\s*(.+)`)
	inst = regexp.MustCompile(`^\s*(.*?:-?\d+)\s+(0x[\da-f]+)\s+([\da-f]+)\s*([\w\.?]+)(.*)`)

	pcrel = regexp.MustCompile(`(-?\d+)\(PC\)$`)
	hex   = regexp.MustCompile(`(0x[\da-f]+)$`)
)

var (
	symbolPrefix = flag.String("prefix", "", "prefix to strip from symbols")
	info         = flag.String("info", "", "set to 'fileline' for line info and 'gnu' for GCC syntax")
	nops         = flag.Bool("nops", false, "if set, no-ops won't be filtered out")
	filter       = flag.String("s", "", "regexp to filter symbols by")
	output       = flag.String("o", "-", "location to dump to; defaults to stdout")
)

// Func is a function symbol extracted from an object file dump.
type Func struct {
	Name string // The symbol name.
	File string // The file it came from.
	Code []Inst // Its code, parsed as a sequence of instructions.
}

// Inst is an instruction in a [Func].
type Inst struct {
	Loc      string   // The location (file:line).
	PC       uint64   // The program counter for this instruction.
	Hex      string   // The instruction's encoding as a hex string.
	Mnemonic string   // The instructions's opcode mnemonic.
	Args     []string // Operands for the instruction.

	GCC string // A gcc-formatted version of this instruction.

	Label string // A disassembler label attached to this instruction.
}

// dumpObjectFile shells out to go tool objdump.
func dumpObjectFile(binary string) (string, error) {
	argv0, ok := os.LookupEnv("GO")
	if !ok {
		argv0 = "go"
	}

	stdout := new(strings.Builder)

	cmd := exec.Command(argv0, "tool", "objdump", "-gnu", "-s", *filter, binary)
	fmt.Fprintln(os.Stderr, "running:", strings.Join(cmd.Args, " "))
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	return stdout.String(), err
}

// parseDump parses the output of go tool objdump.
func parseDump(data string) (fns []Func, err error) {
	prefix := *symbolPrefix

	symCleanup := func(arg string) string {
		arg = strings.TrimSpace(arg)
		arg = strings.ReplaceAll(arg, prefix+"/internal/", ".")
		arg = strings.ReplaceAll(arg, prefix, "")

		// Convert the generic prefix into as ~ for compactness.
		arg = strings.ReplaceAll(arg, "go.shape.", "~")

		return arg
	}

	for line := range strings.SplitSeq(data, "\n") {
		if line == "" {
			continue
		}

		if match := text.FindStringSubmatch(line); match != nil {
			fns = append(fns, Func{
				Name: symCleanup(match[1]),
				File: match[2],
			})
			continue
		}

		match := inst.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("invalid line: %s", line)
		}

		var inst Inst

		inst.Loc = match[1]
		inst.PC, _ = strconv.ParseUint(match[2], 0, 64)
		inst.Hex = match[3]
		inst.Mnemonic = match[4]
		args, gnu, _ := strings.Cut(match[5], "//")
		inst.Args = strings.Split(args, ",")
		inst.GCC = strings.TrimSpace(strings.TrimPrefix(gnu, "//"))

		for i := range inst.Args {
			inst.Args[i] = symCleanup(inst.Args[i])
		}

		if !*nops {
			switch inst.Mnemonic {
			case "NOP", "NOPW", "NOPL", "NOPQ", "NOOP", "?", "INT":
				continue
			}
		}

		cur := &fns[len(fns)-1]
		cur.Code = append(cur.Code, inst)
	}

	return fns, err
}

// generateLabels annotates a function's branch instructions with labels.
func generateLabels(fn *Func) {
	callers := make(map[uint64][]*Inst)
	for i := range fn.Code {
		inst := &fn.Code[i]
		if !jump.MatchString(inst.Mnemonic) {
			continue
		}

		if len(inst.Args) == 0 {
			continue
		}
		arg := inst.Args[len(inst.Args)-1]

		// Calculate the offset this jump is jumping to. We only really
		// support either direct jumps (like x86) or indirect jumps
		// (like aarch64).
		var target uint64
		if match := pcrel.FindStringSubmatch(arg); match != nil {
			displacement, _ := strconv.Atoi(match[1])
			target = inst.PC + uint64(displacement)*4
		} else if match := hex.FindStringSubmatch(arg); match != nil {
			target, _ = strconv.ParseUint(match[1], 0, 64)
		}

		// Delete the last argument from the instruction.
		inst.Args = inst.Args[:len(inst.Args)-1]

		// Record the instruction so we can generate a label for it.
		callers[target] = append(callers[target], inst)
	}

	name, _, _ := strings.Cut(fn.Name, "[")
	name = name[strings.LastIndex(name, "."):]

	// Now, annotate each instruction with the appropriate jump targets.
	// We want to iterate jumps in order so that the labels are assigned
	// consecutive values.
	idx := 1
	for i := range fn.Code {
		inst := &fn.Code[i]

		jumps := callers[inst.PC]
		if jumps == nil {
			continue
		}

		inst.Label = fmt.Sprintf("%s.L%d", name, idx)
		idx++
		for _, jump := range jumps {
			jump.Args = append(jump.Args, inst.Label)
		}
	}
}

// dumpFuncs re-dumps parsed [Func]s into pretty-printed output.
func dumpFuncs(fns []Func, out io.Writer) error {
	_, err := fmt.Fprint(out, "//go:build disable\n\n")
	if err != nil {
		return err
	}

	var fileLine, gnu bool
	switch *info {
	case "":
	case "fileline":
		fileLine = true
	case "gnu":
		gnu = true
	default:
		return fmt.Errorf("invalid value for -info: %v", *info)
	}

	// Pretty-print each function.
	line := new(bytes.Buffer)
	for _, fn := range fns {
		_, err = fmt.Fprintf(out, "TEXT %s(SB)\n", fn.Name)
		if err != nil {
			return err
		}

		// Find the widest mnemonic and arg string.
		var w1, w2 int
		for _, inst := range fn.Code {
			w1 = max(w1, len(inst.Mnemonic))
			w2 = max(w2, len(strings.Join(inst.Args, ", ")))
		}
		// Round both to a multiple of 2.
		w1 = (w1 + 1) &^ 1
		w2 = (w2 + 1) &^ 1

		// Clamp both to a reasonable maximum.
		w1 = min(w1, 16)
		w2 = min(w2, 40)

		prev := ""
		for _, inst := range fn.Code {
			line.Reset()
			if inst.Label != "" {
				fmt.Fprintf(line, "%s:\n", inst.Label)
			}

			fmt.Fprintf(line, "  %-*s  %-*s", w1, inst.Mnemonic, w2, strings.Join(inst.Args, ", "))

			switch {
			case fileLine:
				// Don't bother with line numbers for assembly functions.
				if fileLine && !strings.HasSuffix(fn.File, ".s") {
					if inst.Loc != prev {
						fmt.Fprintf(line, "  ; %v", inst.Loc)
						prev = inst.Loc
					}
				}
			case gnu:
				fmt.Fprintf(line, "  ; %v", inst.GCC)
			}

			_, err = fmt.Fprintf(out, "%s\n", bytes.TrimRight(line.Bytes(), " "))
			if err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	return nil
}

func run(binary string) error {
	data, err := dumpObjectFile(binary)
	if err != nil {
		return err
	}

	fns, err := parseDump(data)
	if err != nil {
		return err
	}

	// Annotate each function with jump labels.
	wg := new(sync.WaitGroup)
	for i := range fns {
		wg.Go(func() {
			generateLabels(&fns[i])
		})
	}
	wg.Wait()

	out := os.Stdout
	if *output != "-" {
		out, err = os.Create(*output)
		if err != nil {
			return err
		}
		defer out.Close()
	}

	return dumpFuncs(fns, out)
}

func main() {
	flag.Parse()
	if err := run(flag.Arg(0)); err != nil {
		if exit, ok := xerrors.As[*exec.ExitError](err); ok {
			os.Exit(exit.ExitCode())
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
