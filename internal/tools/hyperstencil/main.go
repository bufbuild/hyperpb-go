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

// hyperstencil is a code generator for generating full specializations of
// generic functions.
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
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tiendc/go-deepcopy"
	"golang.org/x/tools/go/packages"

	"buf.build/go/hyperpb/internal/xsync"
)

var (
	directive = regexp.MustCompile(`^//hyperpb:stencil\s+(\w+)\s+([\w.]+)\s*\[(.+)\]\s*(:?(\w+\s*->\s*[\w.]+\s*)*)`)
	rename    = regexp.MustCompile(`(\w+)\s*->\s*([\w.]+)`)

	toolPkg = func() string {
		info, _ := debug.ReadBuildInfo()
		return info.Path
	}()
)

// Directive is a //hyperpb:stencil directive, as described in the package comments.
type Directive struct {
	// The target and source function names.
	Target, Source string
	// The explicit generic arguments taken from the source function.
	Args []string
	// Any renames for this stencil of the form A -> B.
	Renames map[string]string
}

// parseDirective parses a [Directive] out of a comment, if it's in the right
// format.
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

// parseDirectives finds and parses all directives in the file f.
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

// makeStencil accepts a generic function, and a directive which attempts to
// stencil it, and constructs a new function AST with the stencil operations
// applied.
//
// bases and attrs are used to track information used for postprocessing:
// namely, imports and //go: directives, respectively.
func makeStencil(
	dir Directive,
	generic *ast.FuncDecl,
	bases *xsync.Set[string],
	attrs *xsync.Map[string, []string],
) (*ast.FuncDecl, error) {
	if generic == nil {
		return nil, fmt.Errorf("no function with name %s", dir.Source)
	}
	generic.Name.Obj = nil

	var attrComments []string
	for _, c := range generic.Doc.List {
		if strings.HasPrefix(c.Text, "//go:") {
			attrComments = append(attrComments, c.Text)
		}
	}
	if attrComments != nil {
		attrs.Store(dir.Target, attrComments)
	}

	// Make a deep copy of the function so that we can edit it.
	var stencil *ast.FuncDecl
	err := deepcopy.Copy(&stencil, &generic)
	if err != nil {
		return nil, err
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
				return nil, fmt.Errorf("too few arguments for %s", dir.Source)
			}
			dir.Renames[ty.(*ast.Ident).Name] = args[0] //nolint:errcheck
			args = args[1:]
		}

		stencil.Recv = nil
	} else {
		// Add generic parameters to the renames.
		for _, field := range stencil.Type.TypeParams.List {
			for _, name := range field.Names {
				if len(args) == 0 {
					return nil, fmt.Errorf("too few arguments for %s", dir.Source)
				}
				dir.Renames[name.Name] = args[0]
				args = args[1:]
			}
		}

		stencil.Type.TypeParams = nil
	}

	if len(args) > 0 {
		return nil, fmt.Errorf("too many arguments for %s", dir.Source)
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
				bases.Store(id.Name)
			}

		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				// Special case for calling a method that is in the renames
				// array.
				if arg, ok := dir.Renames[sel.Sel.Name]; ok {
					// Rewrite the function expression to an identifier.
					n.Fun = &ast.Ident{Name: arg}

					// Append the selectee as the first argument of the call.
					n.Args = slices.Insert(n.Args, 0, sel.X)
				}
			} else if idx, ok := n.Fun.(*ast.IndexExpr); ok {
				// Special case for calling a generic function.
				if sel, ok := idx.X.(*ast.SelectorExpr); ok {
					if arg, ok := dir.Renames[sel.Sel.Name]; ok {
						// Rewrite the call to be non-generic.
						sel.Sel.Name = arg
						n.Fun = sel
					}
				}
			} else if idx, ok := n.Fun.(*ast.IndexExpr); ok {
				// Special case for calling a generic function.
				if sel, ok := idx.X.(*ast.SelectorExpr); ok {
					if arg, ok := dir.Renames[sel.Sel.Name]; ok {
						// Rewrite the call to be non-generic.
						sel.Sel.Name = arg
						n.Fun = sel
					}
				}
			}
		}

		return v
	}), stencil)

	// Include any replacement targets in the import requirements.
	for _, tgt := range dir.Renames {
		base, _, ok := strings.Cut(tgt, ".")
		if ok {
			bases.Store(base)
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

	return stencil, nil
}

func run() error {
	profile := os.Getenv("STENCIL_PROFILE")
	if profile != "" {
		f, err := os.Create(profile)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	path := os.Getenv("GOFILE")
	pkg := os.Getenv("GOPACKAGE")

	dirname := filepath.Dir(path)
	dir, err := os.ReadDir(dirname)
	if err != nil {
		return err
	}

	isTest := strings.HasSuffix(path, "_test.go")
	outPath := "stencils.go"
	if isTest {
		outPath = "stencils_test.go"
	}
	outPath = filepath.Join(dirname, outPath)

	// Check to see if this file is newer than the files it depends on to avoid
	// needing to regenerate.
	var mtime time.Time
	if info, err := os.Stat(outPath); err == nil {
		mtime = info.ModTime()
	}

	var files []string //nolint:prealloc
	var newer bool
	for _, dirent := range dir {
		if dirent.Type().IsDir() ||
			!strings.HasSuffix(dirent.Name(), ".go") {
			continue
		}
		files = append(files, filepath.Join(dirname, dirent.Name()))

		info, _ := dirent.Info()
		newer = newer || info.ModTime().After(mtime)
	}
	if info, err := os.Stat(os.Args[0]); err == nil {
		// Also include the age of this executable.
		newer = newer || info.ModTime().After(mtime)
	}
	if !newer {
		return nil
	}

	slices.Sort(files)

	var (
		out  = ast.File{Name: ast.NewIdent("x")}
		fset = token.NewFileSet()

		imports  xsync.Map[string, *ast.ImportSpec]
		bases    xsync.Set[string]
		attrs    xsync.Map[string, []string]
		pkgCache xsync.Map[string, []*packages.Package]
	)

	wg := new(sync.WaitGroup)
	ch := make(chan error)
	errs := []error{}
	go func() {
		for e := range ch {
			errs = append(errs, e)
		}
	}()

	// Stenciling isn't super fast; parallelizing it helps a lot.
	decls := make([][]ast.Decl, len(files))
	for i, path := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				ch <- fmt.Errorf("%s:%w", path, err)
				return
			}
			if file.Name.Name != pkg {
				return
			}

			// Build a map of import names to imports.
			for _, imp := range file.Imports {
				if imp.Name != nil {
					imports.Store(imp.Name.Name, imp)
					continue
				}

				path, _ := strconv.Unquote(imp.Path.Value)
				pkgs, ok := pkgCache.Load(path)
				if !ok {
					pkgs, err = packages.Load(nil, path)
					if err != nil {
						ch <- err
						return
					}
					pkgCache.Store(path, pkgs)
				}
				imports.Store(pkgs[0].Name, imp)
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

			directives := parseDirectives(file)

			decls := &decls[i]
			*decls = make([]ast.Decl, len(directives))

			for i, dir := range directives {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Start by finding a func in file with this name.
					generic := funcs[dir.Source]
					stencil, err := makeStencil(dir, generic, &bases, &attrs)
					if err != nil {
						ch <- err
						return
					}

					// Finally, append stencil to the output file.
					(*decls)[i] = stencil
				}()
			}
		}()
	}

	wg.Wait()
	close(ch)
	if len(errs) > 0 {
		return errs[0]
	}

	out.Decls = slices.Concat(decls...)

	var imported []string
	for base := range bases.All() {
		imp, ok := imports.Load(base)
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
	header := fmt.Sprintf(`// Code generated by %s. DO NOT EDIT.

package %s

import (%s)

`, toolPkg, pkg, strings.Join(imported, ";"))

	// Print to a string, so that we can add nosplit comments the "easy" way.
	buf := new(strings.Builder)
	if err := printer.Fprint(buf, fset, &out); err != nil {
		return err
	}
	source := buf.String()

	oldnew := []string{"package x\n", header}
	for name, attrs := range attrs.All() {
		oldnew = append(oldnew, "func "+name, strings.Join(attrs, "\n")+"\nfunc "+name)
	}
	source = strings.NewReplacer(oldnew...).Replace(source)
	bytes, err := format.Source([]byte(source))
	if err != nil {
		return err
	}

	return os.WriteFile(outPath, bytes, 0o666)
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
