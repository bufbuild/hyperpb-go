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

package compiler

import (
	"cmp"
	"fmt"
	"iter"
	"runtime"
	"slices"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/scc"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/compiler/linker"
	"buf.build/go/hyperpb/internal/tdp/profile"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Options is the configuration for [Compile].
type Options struct {
	Profile    profile.Profile
	Extensions ExtensionResolver

	// Backend connects a [compiler] with backend configuration defined in another
	// package.
	//
	// This type mostly exists to break a circular dependency.
	Backend interface {
		// SelectArchetype classifies a field into a particular archetype.
		//
		// prof is a profile that inquires for the profile of a field within the context
		// of parsing fd. It takes a FieldDescriptor rather than a FieldSite because
		// the caller is responsible for constructing the FieldSite.
		//
		// Returns nil if the field is not supported yet.
		SelectArchetype(protoreflect.FieldDescriptor, profile.Field) *Archetype

		// PopulateMethods gives the backend an opportunity to populate the
		// fast-path methods of the generated type.
		PopulateMethods(*protoiface.Methods)
	}
}

// Compile compiles a descriptor into a [Type], for optimized parsing.
//
// Panics if md is too complicated (i.e. it exceeds internal limitations for the compiler).
func Compile(md protoreflect.MessageDescriptor, options Options) *tdp.Type {
	c := &compiler{
		Options: options,
		root:    md,

		types:   make(map[protoreflect.MessageDescriptor]*ir),
		sccInfo: make(map[*scc.Component[*ir]]*sccInfo),

		fdCache: make(map[protoreflect.MessageDescriptor][]protoreflect.ExtensionDescriptor),
	}

	return c.compile(md)
}

// compiler converts descriptors into [tdp.Type]s.
type compiler struct {
	Options
	root  protoreflect.MessageDescriptor
	types map[protoreflect.MessageDescriptor]*ir

	linker.Linker

	dag     *scc.DAG[*ir]
	sccInfo map[*scc.Component[*ir]]*sccInfo

	fdCache map[protoreflect.MessageDescriptor][]protoreflect.FieldDescriptor
}

func (c *compiler) compile(md protoreflect.MessageDescriptor) *tdp.Type {
	if debug.Enabled {
		if profile, ok := c.Profile.(*profile.Recorder); ok {
			c.log("pgo", "\n%s", profile.Dump())
		}
	}

	c.recurse(md)
	c.dag = scc.Sort(c.types[md], func(ty *ir) iter.Seq[*ir] {
		return func(yield func(*ir) bool) {
			for _, t := range ty.t {
				md := fieldMessage(t.d)
				if md != nil && !yield(c.types[md]) {
					return
				}
			}
		}
	})

	for cycle := range c.dag.Topological() {
		c.sccInfo[cycle] = newSCCInfo(c, cycle)

		for _, ir := range cycle.Members() {
			ir.doLayout(c)
			ir.doSchedule(c)
			c.codegen(ir)
		}
	}

	auxes := make([]tdp.Aux, len(c.types))
	buf, err := c.Link(func(size, align int) []byte {
		// Copy buf onto some memory that the GC can trace through md to keep all of
		// the descriptors alive.
		ptr := arena.AllocTraceable(size, unsafe.Pointer(unsafe.SliceData(auxes)))
		return unsafe.Slice(ptr, size)
	})
	if err != nil {
		// This only panics if the compiler hits a hard limit somewhere; this is
		// not really an error that can be meaningfully handled.
		panic(fmt.Errorf("hyperpb: failed to link parser for %s: %w", c.root.FullName(), err))
	}

	c.log("bytes", "%d", len(buf))

	// Resolve all message type references. This needs to be done as a separate
	// step due to potential cycles.
	lib := &tdp.Library{
		Base:  xunsafe.Cast[tdp.Type](unsafe.SliceData(buf)),
		Types: make(map[protoreflect.MessageDescriptor]*tdp.Type),
	}
	requiredSet := make(map[int32]struct{})
	var i int
	for sym, offset := range linker.Symbols[typeSymbol](&c.Linker) {
		ty := lib.AtOffset(uint32(offset))
		ty.Aux = &auxes[i]
		i++

		ty.Library = lib
		ty.Descriptor = sym.ty
		ty.FieldDescriptors = c.fdCache[sym.ty]

		c.Backend.PopulateMethods(&ty.Methods)

		// Find which fields are required or contain required fields.
		for _, fd := range ty.FieldDescriptors {
			if fd.IsExtension() {
				// Extensions cannot be required. Once we see one extension
				// we're all done.
				break
			}

			if fd.Cardinality() == protoreflect.Required {
				requiredSet[int32(fd.Index())] = struct{}{}
			}

			m := fieldMessage(fd)
			if m != nil && c.sccInfo[c.dag.ForNode(c.types[m])].hasRequired {
				requiredSet[^int32(fd.Index())] = struct{}{}
			}
		}
		for i := range requiredSet {
			ty.Required = append(ty.Required, i)
		}
		slices.Sort(ty.Required)
		slices.Reverse(ty.Required)
		clear(requiredSet)

		lib.Types[sym.ty] = ty

		if debug.Enabled {
			*ty.Layout.Get() = c.types[sym.ty].layout
		}
	}

	if debug.Enabled {
		runtime.SetFinalizer(lib.Base, func(t *tdp.Type) {
			c.log("finalizer", "%p:%s", t, t.Descriptor.FullName())
		})
	}

	entry := lib.Types[md]
	c.log("done", "%v", entry)
	return entry
}

// profile returns profiling information for fd in the compiler's current
// context.
func (c *compiler) profile(fd protoreflect.FieldDescriptor) profile.Field {
	site := profile.Site{Field: fd}
	if c.Profile == nil {
		return site.DefaultProfile()
	}
	return c.Profile.ForField(site)
}

func (c *compiler) fields(md protoreflect.MessageDescriptor) []protoreflect.FieldDescriptor {
	fields, ok := c.fdCache[md]
	if ok {
		return fields
	}

	fds := md.Fields()
	fields = make([]protoreflect.FieldDescriptor, fds.Len())
	for i := range fields {
		fields[i] = fds.Get(i)
	}

	if c.Extensions != nil {
		fields = append(fields, c.Extensions.FindExtensionsByMessage(md.FullName())...)
		// Ensure determinism by sorting the extension fields by number. The
		// implementation of GlobalTypes.RangeExtensions uses a map and so likes
		// to have a per-process random order.
		//
		// We don't actually lose anything from not sorting, but it makes tests
		// deterministic.
		slices.SortFunc(fields[fds.Len():], func(a, b protoreflect.FieldDescriptor) int {
			return cmp.Compare(a.Number(), b.Number())
		})
	}

	c.fdCache[md] = fields
	return fields
}

// recurse calls analyze recursively.
func (c *compiler) recurse(md protoreflect.MessageDescriptor) {
	if c.types[md] != nil {
		return
	}

	c.log("message", "%s", md.FullName())
	ir := newIR(c, md)
	c.types[md] = ir
	for _, t := range ir.t {
		if m := fieldMessage(t.d); m != nil {
			c.recurse(m)
		}
	}
}

// codegen code-generates the analyzed contents of an intermediate
// representation.
func (c *compiler) codegen(ir *ir) {
	tSym := typeSymbol{ty: ir.d}
	pSym := parserSymbol{ty: ir.d}
	mSym := parserSymbol{ty: ir.d, mapEntry: true}

	ty := c.NewSymbol(tSym)
	ty.Rel(
		linker.Rel{
			Symbol: pSym,
			Offset: unsafe.Offsetof(tdp.Type{}.Parser),
			Kind:   linker.Address,
		},
		linker.Rel{
			Symbol: tableSymbol{tSym},
			Offset: unsafe.Offsetof(tdp.Type{}.Numbers),
			Kind:   linker.Address,
		},
	)
	ty.Push(tdp.Type{
		Size:     uint32(ir.hot),
		ColdSize: uint32(ir.cold),
		Count:    uint32(len(ir.t)),
	})

	numbers := make([]swiss.Entry[int32, uint32], 0, len(ir.t))
	for i, tf := range ir.t {
		if md := fieldMessage(tf.d); md != nil {
			ty.Rel(linker.Rel{
				Symbol: typeSymbol{md},
				Offset: unsafe.Offsetof(tdp.Field{}.Message),
				Kind:   linker.Address,
			})
		}

		// Append whatever field data we can before doing layout.
		ty.Push(tdp.Field{
			Accessor: tdp.Accessor{
				Offset: tf.offset,
				Getter: tf.arch.Getter.adapt(),
			},
		})

		numbers = append(numbers, swiss.KV(int32(tf.d.Number()), uint32(i)))
	}
	// Append the dummy end field.
	ty.Push(tdp.Field{})

	// Append the field number table.
	linker.PushTable(c.NewSymbol(tableSymbol{tSym}), numbers...)

	tp := c.NewSymbol(pSym)
	tp.Rel(
		linker.Rel{
			Symbol: tSym,
			Offset: unsafe.Offsetof(tdp.TypeParser{}.TypeOffset),
			Kind:   linker.Abs32,
		},
		linker.Rel{
			Symbol: tableSymbol{pSym},
			Offset: unsafe.Offsetof(tdp.TypeParser{}.Tags),
			Kind:   linker.Address,
		},
		linker.Rel{
			Symbol: mSym,
			Offset: unsafe.Offsetof(tdp.TypeParser{}.MapEntry),
			Kind:   linker.Address,
		},
		linker.Rel{
			Symbol: fieldParserSymbol{parser: pSym, index: 0},
			Offset: unsafe.Offsetof(tdp.TypeParser{}.Entrypoint) +
				unsafe.Offsetof(tdp.FieldParser{}.NextOk),
			Kind: linker.Address,
		},
	)
	tpOffset := tp.Push(tdp.TypeParser{})

	numbers = numbers[:0]
	// Lay out the parser table.
	for i, pf := range ir.p {
		tf := ir.t[pf.tIdx]
		p := tf.arch.Parsers[pf.aIdx]

		tag := tdp.EncodeTag(tf.d.Number(), p.Kind)

		numbers = append(numbers, swiss.KV(
			int32(protowire.EncodeTag(tf.d.Number(), p.Kind)),
			uint32(i),
		))

		nextOk := pf.next
		nextErr := i + 1
		if nextErr == len(ir.p) {
			nextErr = 0
		}

		fp := c.NewSymbol(fieldParserSymbol{parser: pSym, index: i})
		fp.Rel(
			linker.Rel{
				Symbol: fieldParserSymbol{parser: pSym, index: nextOk},
				Offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
				Kind:   linker.Address,
			},
			linker.Rel{
				Symbol: fieldParserSymbol{parser: pSym, index: nextErr},
				Offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
				Kind:   linker.Address,
			},
		)

		if md := fieldMessage(tf.d); md != nil {
			fp.Rel(linker.Rel{
				Symbol: parserSymbol{ty: md},
				Offset: unsafe.Offsetof(tdp.FieldParser{}.Message),
				Kind:   linker.Address,
			})
		}

		fp.Push(tdp.FieldParser{
			Tag:     tag,
			Offset:  tf.offset,
			Preload: uint32(ir.t[pf.tIdx].prof.ExpectedCount),
			Parse:   uintptr(xunsafe.NewPC(p.Thunk)),
		})
	}

	// Ensure that there is at least one parser to be the entry-point.
	if len(ir.p) == 0 {
		fp := c.NewSymbol(fieldParserSymbol{parser: pSym, index: 0})
		fp.Rel(
			linker.Rel{
				Symbol: fieldParserSymbol{parser: pSym, index: 0},
				Offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
				Kind:   linker.Address,
			},
			linker.Rel{
				Symbol: fieldParserSymbol{parser: pSym, index: 0},
				Offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
				Kind:   linker.Address,
			},
		)
		fp.Push(tdp.FieldParser{
			Tag: ^tdp.Tag(0), // This will never be matched.
		})
	}

	// Write the fast-lookup lut.
	writeLUT(c, tp, tpOffset, numbers)

	// Append the parser's field number table.
	linker.PushTable(c.NewSymbol(tableSymbol{pSym}), numbers...)

	mp := c.NewSymbol(mSym)
	mp.Rel(
		linker.Rel{
			Symbol: tSym,
			Offset: unsafe.Offsetof(tdp.TypeParser{}.TypeOffset),
			Kind:   linker.Abs32,
		},
		linker.Rel{
			Symbol: tableSymbol{mSym},
			Offset: unsafe.Offsetof(tdp.TypeParser{}.Tags),
			Kind:   linker.Address,
		},
		linker.Rel{
			Symbol: fieldParserSymbol{parser: mSym, index: 0},
			Offset: unsafe.Offsetof(tdp.TypeParser{}.Entrypoint) +
				unsafe.Offsetof(tdp.FieldParser{}.NextOk),
			Kind: linker.Address,
		},
	)
	mpOffset := mp.Push(tdp.TypeParser{
		DiscardUnknown: true,
	})

	// Write the map entry parser.
	const mapValue = 0x2<<3 | tdp.Tag(protowire.BytesType) // Field number 2 with bytes type (so, 0b10010).
	numbers = []swiss.Entry[int32, uint32]{{Key: int32(mapValue), Value: 0}}
	mpf := c.NewSymbol(fieldParserSymbol{parser: mSym, index: 0})
	mpf.Rel(
		linker.Rel{
			Symbol: fieldParserSymbol{parser: mSym, index: 0},
			Offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
			Kind:   linker.Abs32,
		},
		linker.Rel{
			Symbol: fieldParserSymbol{parser: mSym, index: 0},
			Offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
			Kind:   linker.Address,
		},
		linker.Rel{
			Symbol: pSym,
			Offset: unsafe.Offsetof(tdp.FieldParser{}.Message),
			Kind:   linker.Address,
		},
	)
	mpf.Push(tdp.FieldParser{
		Tag:   mapValue,
		Parse: uintptr(xunsafe.NewPC(vm.Thunk(vm.P1.ParseMapEntry))),
	})

	// Write the fast-lookup lut.
	writeLUT(c, mp, mpOffset, numbers)

	// Append the parser's field number table.
	linker.PushTable(c.NewSymbol(tableSymbol{mSym}), numbers...)
}

func fieldMessage(fd protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if fd.IsMap() {
		return fd.MapValue().Message()
	}
	return fd.Message()
}

func writeLUT(c *compiler, sym *linker.Sym, offset int, entries []swiss.Entry[int32, uint32]) {
	offset += int(unsafe.Offsetof(tdp.TypeParser{}.TagLUT))
	lut := sym.At(offset, offset+128)

	for i := range lut {
		lut[i] = 0xff
	}

	for _, e := range entries {
		if e.Key >= 0 && e.Key < 128 && e.Value <= 0xff {
			c.log("lut write", "%#x->%#x", e.Key, e.Value)
			lut[byte(e.Key)] = uint8(e.Value)
		}
	}

	c.log("lut", "%x", lut)
}

func (c *compiler) log(op, format string, args ...any) {
	debug.Log([]any{"%p", c}, op, format, args...)
}
