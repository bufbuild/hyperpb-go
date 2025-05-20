// Copyright 2020-2025 Buf Technologies, Inc.
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

// Stencil is a code generator for generating full specializations of generic
// functions.
//
// This generator looks for directives of the form
//
//	//stencil:generate Name Func[...] A -> B...
//
// Name is the name of the generated function, Func is the generic function that
// is being copied, and [...] are the explicit generic arguments. The A -> B
// allow for renaming symbols within the function to specialized versions.
//
// Generated functions are placed in a file called _stencils.go. All files in
// a package are processed in one go.
//
//nolint:errcheck // Internal tool; Panicking on error is fine.
package main

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tiendc/go-deepcopy"
	"golang.org/x/tools/go/packages"
)

var (
	directive = regexp.MustCompile(`^//fastpb:stencil\s+(\w+)\s+([\w.]+)\s*\[(.+)\]\s*(:?(\w+\s*->\s*[\w.]+\s*)*)`)
	rename    = regexp.MustCompile(`(\w+)\s*->\s*([\w.]+)`)
)

type Directive struct {
	Target, Source string
	Args           []string
	Renames        map[string]string
}

func parseDirective(comment *ast.Comment) (dir Directive, ok bool) {
	match := directive.FindStringSubmatch(comment.Text)
	if match == nil {
		return dir, false
	}

	dir.Target, dir.Source = match[1], match[2]
	dir.Args = strings.Split(match[3], ",")

	for i := range dir.Args {
		dir.Args[i] = strings.TrimSpace(dir.Args[i])
	}

	dir.Renames = make(map[string]string)
	for _, rename := range rename.FindAllStringSubmatch(match[4], -1) {
		dir.Renames[rename[1]] = rename[2]
	}

	return dir, true
}

func parseDirectives(f *ast.File) (dirs []Directive) {
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if dir, ok := parseDirective(c); ok {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

func run() error {
	path := os.Getenv("GOFILE")
	dirname := filepath.Dir(path)
	dir, err := os.ReadDir(dirname)
	if err != nil {
		return err
	}

	var files []string //nolint:prealloc
	for _, dirent := range dir {
		if dirent.Type().IsDir() ||
			!strings.HasSuffix(dirent.Name(), ".go") ||
			strings.HasSuffix(dirent.Name(), "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dirname, dirent.Name()))
	}
	slices.Sort(files)

	out := new(ast.File)
	out.Name = ast.NewIdent("x")

	fset := token.NewFileSet()
	var nosplits []string
	imports := make(map[string]*ast.ImportSpec)
	bases := make(map[string]struct{})

	var pkg string
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkg = file.Name.Name

		// Build a map of import names to imports.
		for _, imp := range file.Imports {
			if imp.Name != nil {
				imports[imp.Name.Name] = imp
				continue
			}

			path, _ := strconv.Unquote(imp.Path.Value)
			pkgs, err := packages.Load(nil, path)
			if err != nil {
				return err
			}

			imports[pkgs[0].Name] = imp
		}

		// Build a map of names to funcs.
		funcs := make(map[string]*ast.FuncDecl)
		for _, decl := range file.Decls {
			fnc, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if fnc.Recv == nil {
				funcs[fnc.Name.Name] = fnc
				continue
			}

			var recv string
			expr := fnc.Recv.List[0].Type
		loop:
			for {
				switch e := expr.(type) {
				case *ast.Ident:
					recv = e.Name
					break loop
				case *ast.StarExpr:
					expr = e.X
				case *ast.IndexExpr:
					expr = e.X
				case *ast.IndexListExpr:
					expr = e.X
				}
			}

			funcs[recv+"."+fnc.Name.Name] = fnc
		}

		for _, dir := range parseDirectives(file) {
			// Start by finding a func in file with this name.
			generic := funcs[dir.Source]
			if generic == nil {
				return fmt.Errorf("no function with name %s", dir.Source)
			}
			generic.Name.Obj = nil

			for _, c := range generic.Doc.List {
				if c.Text == "//go:nosplit" {
					nosplits = append(nosplits, dir.Target)
					break
				}
			}

			// Make a deep copy of the function so that we can edit it.
			var stencil *ast.FuncDecl
			err := deepcopy.Copy(&stencil, &generic)
			if err != nil {
				panic(err)
			}

			stencil.Doc = nil

			args := dir.Args
			if stencil.Recv != nil {
				// Append the receiver as the first parameter of the function.
				stencil.Type.Params.List = append(stencil.Recv.List, stencil.Type.Params.List...)

				// Add the receiver's type parameters to the renames.
				var params []ast.Expr
				expr := stencil.Recv.List[0].Type
			again:
				switch e := expr.(type) {
				case *ast.Ident:
					break
				case *ast.StarExpr:
					expr = e.X
					goto again
				case *ast.IndexExpr:
					params = []ast.Expr{e.Index}
				case *ast.IndexListExpr:
					params = e.Indices
				}

				for _, ty := range params {
					if len(args) == 0 {
						return fmt.Errorf("too few arguments for %s", dir.Source)
					}
					dir.Renames[ty.(*ast.Ident).Name] = args[0]
					args = args[1:]
				}

				stencil.Recv = nil
			} else {
				// Add generic parameters to the renames.
				for _, field := range stencil.Type.TypeParams.List {
					for _, name := range field.Names {
						if len(args) == 0 {
							return fmt.Errorf("too few arguments for %s", dir.Source)
						}
						dir.Renames[name.Name] = args[0]
						args = args[1:]
					}
				}

				stencil.Type.TypeParams = nil
			}

			if len(args) > 0 {
				return fmt.Errorf("too many arguments for %s", dir.Source)
			}

			stencil.Name.Name = dir.Target

			// Now walk the AST of stencil and overwrite any identifiers with the
			// same name as a generic parameter. THis isn't perfect, but it's
			// essentially all we need.
			//
			// On the way down, also record all of the bases for selectors. This
			// is necessary to determine the requisite import set.
			ast.Walk(visitor(func(v visitor, n ast.Node) ast.Visitor {
				switch n := n.(type) {
				case *ast.Ident:
					if arg, ok := dir.Renames[n.Name]; ok {
						n.Name = arg
					}

				case *ast.SelectorExpr:
					if id, ok := n.X.(*ast.Ident); ok {
						bases[id.Name] = struct{}{}
					}

				case *ast.CallExpr:
					// Special case for calling a method that is in the renames
					// array.
					sel, ok := n.Fun.(*ast.SelectorExpr)
					if !ok {
						break
					}
					if arg, ok := dir.Renames[sel.Sel.Name]; ok {
						// Rewrite the function expression to an identifier.
						n.Fun = &ast.Ident{Name: arg}

						// Append the selectee as the first argument of the call.
						n.Args = slices.Insert(n.Args, 0, sel.X)
					}
				}

				return v
			}), stencil)

			// Include any replacement targets in the import requirements.
			for _, tgt := range dir.Renames {
				base, _, ok := strings.Cut(tgt, ".")
				if ok {
					bases[base] = struct{}{}
				}
			}

			// Append a statement to the function to force the generic function to
			// be used. This helps silence lints that don't understand that the
			// seemingly unused generic function is part of the build step.
			var orig ast.Expr
			if generic.Recv != nil {
				orig = &ast.SelectorExpr{
					X:   &ast.ParenExpr{X: stencil.Type.Params.List[0].Type},
					Sel: generic.Name,
				}
			} else {
				var indices []ast.Expr
				for _, arg := range dir.Args {
					indices = append(indices, &ast.Ident{Name: arg})
				}
				orig = &ast.IndexListExpr{
					X:       generic.Name,
					Indices: indices,
				}
			}

			stencil.Body.List = slices.Insert(
				stencil.Body.List, 0, ast.Stmt(&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "_"}},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{orig},
				}),
			)

			// Finally, append stencil to the output file.
			out.Decls = append(out.Decls, stencil)
		}
	}

	var imported []string
	for base := range bases {
		imp, ok := imports[base]
		if ok {
			imported = append(imported, imp.Path.Value)
		}
	}
	slices.SortFunc(imported, func(a, b string) int {
		stdA, stdB := !strings.Contains(a, "."), !strings.Contains(b, ".")
		if stdA && !stdB {
			return -1
		}
		if stdB && !stdA {
			return 1
		}

		return cmp.Compare(a, b)
	})

	// Generating this in the AST is far too painful.
	header := fmt.Sprintf(`package %s

import (%s)

// Code generated by internal/stencil. DO NOT EDIT

`, pkg, strings.Join(imported, ";"))

	// Print to a string, so that we can add nosplit comments the "easy" way.
	buf := new(strings.Builder)
	if err := printer.Fprint(buf, fset, out); err != nil {
		return err
	}
	source := buf.String()

	oldnew := []string{"package x\n", header}
	for _, name := range nosplits {
		oldnew = append(oldnew, "func "+name, "//go:nosplit\nfunc "+name)
	}
	source = strings.NewReplacer(oldnew...).Replace(source)
	bytes, err := format.Source([]byte(source))
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dirname, "stencils.go"), bytes, 0o666)
}

type visitor func(visitor, ast.Node) ast.Visitor

func (v visitor) Visit(node ast.Node) ast.Visitor {
	return v(v, node)
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
