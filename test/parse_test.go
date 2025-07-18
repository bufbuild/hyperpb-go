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

package test

import (
	"flag"
	"fmt"
	"runtime"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/xflag"
	"buf.build/go/hyperpb/test/testdata"
)

var verbose bool

func TestMain(m *testing.M) {
	flag.Parse()
	verbose = xflag.Lookup[bool]("test.v")

	if xflag.Lookup[string]("test.bench") != "" {
		// Annoyingly, benchmarking won't print the compiler used...
		fmt.Printf("compiler: %v %v\n", runtime.Compiler, runtime.Version())
	}

	m.Run()
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()
	testdata.RunAll(t, func(t *testing.T, test *testdata.TestCase) {
		t.Helper()
		test.Run(t, nil, verbose)
	})
}

func BenchmarkUnmarshal(b *testing.B) {
	testdata.RunAll(b, func(b *testing.B, test *testdata.TestCase) {
		b.Helper()

		run := func(b *testing.B, specimen []byte) {
			b.Helper()
			b.Run("hyperpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := hyperpb.NewMessage(test.Type.Fast)
					_ = proto.Unmarshal(specimen, m)
				}
			})
			b.Run("zerocopy", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := hyperpb.NewMessage(test.Type.Fast)
					_ = m.Unmarshal(specimen, hyperpb.WithAllowAlias(true))
				}
			})
			b.Run("arena", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				ctx := new(hyperpb.Shared)
				for range b.N {
					m := ctx.NewMessage(test.Type.Fast)
					_ = m.Unmarshal(specimen, hyperpb.WithAllowAlias(true))
					ctx.Free()
				}
			})
			b.Run("pgo", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				ctx := new(hyperpb.Shared)

				// Warmup.
				profile := test.Type.Fast.NewProfile()
				for range 16 {
					m := ctx.NewMessage(test.Type.Fast)
					_ = m.Unmarshal(specimen,
						hyperpb.WithAllowAlias(true),
						hyperpb.WithRecordProfile(profile, 1.0),
					)
					ctx.Free()
				}
				ty := test.Type.Fast.Recompile(profile)

				o := hyperpb.WithAllowAlias(true)
				b.ResetTimer()
				for range b.N {
					m := ctx.NewMessage(ty)
					_ = m.Unmarshal(specimen, o)
					ctx.Free()
				}
			})
			b.Run("gencode", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := test.Type.Gencode.New().Interface()
					_ = proto.Unmarshal(specimen, m)
				}
			})
			b.Run("vtproto", func(b *testing.B) {
				type vtMessage interface{ UnmarshalVTUnsafe([]byte) error }
				if _, ok := test.Type.Gencode.New().Interface().(vtMessage); !ok {
					b.SkipNow()
				}

				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := test.Type.Gencode.New().Interface().(vtMessage) //nolint:errcheck
					_ = m.UnmarshalVTUnsafe(specimen)
				}
			})
			b.Run("dynamicpb", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(specimen)))
				for range b.N {
					m := dynamicpb.NewMessage(test.Type.Gencode.Descriptor())
					_ = proto.Unmarshal(specimen, m)
				}
			})
		}

		if len(test.Specimens) == 1 {
			run(b, test.Specimens[0])
			return
		}

		for _, specimen := range test.Specimens {
			b.Run("", func(b *testing.B) { run(b, specimen) })
		}
	})
}
