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

// Package gencode provides helpers for printing generated code.
package printer

import (
	"bytes"
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// Printer is a gencode pretty-printer.
//
// The zero value is ready to use.
type Printer struct {
	*protogen.GeneratedFile
	buf indented

	// If true, printing will show the site at which a print function was called.
	trace bool
}

// NewPrinter wraps a [protogen.GenreatedFile].
func NewPrinter(g *protogen.GeneratedFile) *Printer {
	return &Printer{GeneratedFile: g}
}

// SetTrace sets whether this printer should print trace comments.
func (p *Printer) SetTrace(trace bool) {
	p.trace = trace
}

// P prints some text directly to this printer.
func (p *Printer) P(format string, args ...any) {
	var comment string
	if p.trace {
		file, line := Caller()
		if file != "" {
			file = strings.TrimPrefix(file, "buf.build/go/hyperpb/")
			comment = fmt.Sprintf("// %s:%d", file, line)
		}
	}

	for i, arg := range args {
		if id, ok := arg.(protogen.GoIdent); ok {
			args[i] = p.QualifiedGoIdent(id)
		}
	}

	comment, p.buf.comment = p.buf.comment, comment
	fmt.Fprintf(&p.buf, format, args...)
	p.buf.comment = comment
}

// Block helps with printing a delimited multi-line block. open/close are
// the delimiters, such as "{" and "}". All printing of the contents of the
// block should happen in body.
func (p *Printer) Block(open, close string, body func()) {
	var comment string
	if p.trace {
		file, line := Caller()
		if file != "" {
			file = strings.TrimPrefix(file, "buf.build/go/hyperpb/")
			comment = fmt.Sprintf("// %s:%d", file, line)
		}
	}

	comment, p.buf.comment = p.buf.comment, comment
	p.buf.buf.WriteString(open)
	backtrack := p.buf.buf.Len()

	fmt.Fprintln(&p.buf)
	mark := p.buf.buf.Len()

	p.buf.indent++
	body()
	p.buf.indent--

	if p.buf.buf.Len() == mark {
		// Nothing got printed. Get rid of the newline.
		p.buf.buf.Truncate(backtrack)
	} else if p.buf.indented {
		// Need a newline before the close.
		fmt.Fprintln(&p.buf)
	}

	p.buf.buf.WriteString(close)
	p.buf.comment = comment
}

// Write implements [io.Writer].
func (p *Printer) Write(buf []byte) (int, error) {
	return p.buf.Write(buf)
}

// Finish dumps this printer's buffer into the given writer.
func (p *Printer) Finish() error {
	_, err := p.GeneratedFile.Write(p.buf.buf.Bytes())
	return err
}

type indented struct {
	buf      bytes.Buffer
	indent   int  // Number of tabs to indent with.
	indented bool // Whether or not we have already indented the current line.

	comment string // Used to print debugging information.
}

func (i *indented) Write(buf []byte) (int, error) {
	n := len(buf)
	for len(buf) > 0 {
		line, rest, nl := bytes.Cut(buf, []byte("\n"))
		buf = rest
		if len(line) > 0 && !i.indented {
			for range i.indent {
				i.buf.WriteByte('\t')
			}
			i.indented = true
		}
		i.buf.Write(line)
		if nl {
			i.buf.WriteString(i.comment)
			i.buf.WriteByte('\n')
			i.indented = false
		}
	}

	return n, nil
}
