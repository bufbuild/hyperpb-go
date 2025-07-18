// gen is used for randomly generating benchmark data using the schemas from
// the Rust Serialization Benchmark.
//
// See https://github.com/djkoloski/rust_serialization_benchmark/tree/master
package main

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/test/internal/gen/rsb"
	logpb "buf.build/go/hyperpb/test/internal/gen/rsb/log"
	meshpb "buf.build/go/hyperpb/test/internal/gen/rsb/mesh"
	minecraftpb "buf.build/go/hyperpb/test/internal/gen/rsb/minecraft"
	mk48pb "buf.build/go/hyperpb/test/internal/gen/rsb/mk48"
)

func run() error {
	dir := getDir()

	err := GenerateYAML[*logpb.Logs](1, filepath.Join(dir, "log.yaml"))
	if err != nil {
		return err
	}
	err = GenerateYAML[*meshpb.Mesh](1, filepath.Join(dir, "mesh.yaml"))
	if err != nil {
		return err
	}
	err = GenerateYAML[*minecraftpb.Players](1, filepath.Join(dir, "minecraft.yaml"))
	if err != nil {
		return err
	}
	err = GenerateYAML[*mk48pb.Updates](1, filepath.Join(dir, "mk48.yaml"))
	if err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func GenerateYAML[M proto.Message](count int, path string) error {
	specimens := make([][]byte, count)
	var m M
	m = m.ProtoReflect().Type().New().Interface().(M)
	for i := range specimens {
		proto.Reset(m)
		Generate(m)
		specimens[i], _ = proto.Marshal(m)
	}

	out := new(bytes.Buffer)
	fmt.Fprintln(out, "type:", m.ProtoReflect().Descriptor().FullName())
	fmt.Fprintln(out, "benchmark: true")
	fmt.Fprintln(out, "large: true")
	fmt.Fprintln(out, "hex:")

	for _, specimen := range specimens {
		fmt.Fprintln(out, "- |")
		// Break it up into rows of 64, so 32 bytes to a row.
		for i := 0; i < len(specimen); i += 32 {
			chunk := specimen[i:min(len(specimen), i+32)]
			fmt.Fprintf(out, "  %x\n", chunk)
		}
	}

	fmt.Printf("writing %v: %.3f MB\n", path, float64(out.Len())/(1<<20))
	return os.WriteFile(path, out.Bytes(), 0666)
}

func Generate(m proto.Message) {
	r := m.ProtoReflect()
	options := proto.GetExtension(r.Descriptor().Options(), rsb.E_M).(*rsb.MessageOptions)
	options = orDefaultMessage(options)
	depth := orDefault(options.MaxDepth, 8)
	GenerateMessage(int(depth), r)
}

func GenerateMessage(depth int, m protoreflect.Message) {
	if depth == 0 {
		return
	}

	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		options := proto.GetExtension(fd.Options(), rsb.E_F).(*rsb.FieldOptions)
		p := 0.75
		if options != nil {
			p = options.P
		}

		if rand.Float64() > p {
			continue
		}

		switch {
		case fd.IsList():
			len := orDefaultMessage(options.GetLen())
			min := int(orDefault(len.Min, 2))
			max := int(orDefault(len.Max, 32))
			x := m.Mutable(fd).List()
			for range rand.IntN(max-min+1) + min {
				if fd.Message() != nil {
					GenerateMessage(depth-1, x.AppendMutable().Message())
				} else {
					x.Append(GenerateSingular(fd))
				}
			}

		case fd.IsMap():
			panic("unimplemented")

		case fd.Message() != nil:
			GenerateMessage(depth-1, m.Mutable(fd).Message())

		default:
			m.Set(fd, GenerateSingular(fd))
		}
	}
}

func GenerateSingular(fd protoreflect.FieldDescriptor) protoreflect.Value {
	options := proto.GetExtension(fd.Options(), rsb.E_F).(*rsb.FieldOptions)

	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		m := orDefaultMessage(options.GetInt())
		min := orDefault(m.Min, math.MinInt32)
		max := orDefault(m.Max, math.MaxInt32)
		n := int32(rand.Uint32N(uint32(max-min))) + int32(min)
		return protoreflect.ValueOf(n)

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		m := orDefaultMessage(options.GetInt())
		min := orDefault(m.Min, math.MinInt64)
		max := orDefault(m.Max, math.MaxInt64)
		n := int64(rand.Uint64N(uint64(max-min))) + min
		return protoreflect.ValueOf(n)

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		m := orDefaultMessage(options.GetUint())
		min := orDefault(m.Min, 0)
		max := orDefault(m.Max, math.MaxUint32)
		n := rand.Uint32N(uint32(max-min)) + uint32(min)
		return protoreflect.ValueOf(n)

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		m := orDefaultMessage(options.GetUint())
		min := orDefault(m.Min, 0)
		max := orDefault(m.Max, math.MaxUint64)
		n := rand.Uint64N(max-min) + min
		return protoreflect.ValueOf(n)

	case protoreflect.EnumKind:
		values := fd.Enum().Values()
		value := values.Get(rand.IntN(values.Len()))
		return protoreflect.ValueOf(value.Number())

	case protoreflect.BoolKind:
		return protoreflect.ValueOf(rand.IntN(2) == 0)

	case protoreflect.FloatKind:
		return protoreflect.ValueOf(rand.Float32())

	case protoreflect.DoubleKind:
		return protoreflect.ValueOf(rand.Float64())

	case protoreflect.StringKind:
		m := orDefaultMessage(options.GetLen())
		min := int(orDefault(m.Min, 2))
		max := int(orDefault(m.Max, 32))
		n := rand.IntN(max-min+1) + min
		data := make([]byte, n)
		for i := range n {
			data[i] = byte(rand.IntN(128))
		}
		return protoreflect.ValueOf(string(data))

	case protoreflect.BytesKind:
		m := orDefaultMessage(options.GetLen())
		min := int(orDefault(m.Min, 2))
		max := int(orDefault(m.Max, 32))
		n := rand.IntN(max-min+1) + min
		data := make([]byte, n)
		for i := range n {
			data[i] = byte(rand.IntN(256))
		}
		return protoreflect.ValueOf(data)

	default:
		panic("unexpected protoreflect.Kind")
	}
}

func getDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func orDefault[T any](p *T, d T) T {
	if p == nil {
		return d
	}
	return *p
}

func orDefaultMessage[M proto.Message](m M) M {
	if !m.ProtoReflect().IsValid() {
		m = m.ProtoReflect().Type().New().Interface().(M)
	}
	return m
}
