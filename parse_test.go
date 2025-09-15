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

package hyperpb_test

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"

	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/testdata"
	"buf.build/go/hyperpb/internal/xflag"
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

		run := func(b *testing.B, specimens [][]byte) {
			b.Helper()
			b.Run("hyperpb", func(b *testing.B) {
				b.ReportAllocs()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := hyperpb.NewMessage(test.Type.Fast)
					_ = proto.Unmarshal(specimen, m)
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("zerocopy", func(b *testing.B) {
				b.ReportAllocs()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := hyperpb.NewMessage(test.Type.Fast)
					_ = m.Unmarshal(specimen, hyperpb.WithAllowAlias(true))
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("arena", func(b *testing.B) {
				b.ReportAllocs()
				ctx := new(hyperpb.Shared)
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := ctx.NewMessage(test.Type.Fast)
					_ = m.Unmarshal(specimen, hyperpb.WithAllowAlias(true))
					ctx.Free()
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("arena-w-pool", func(b *testing.B) {
				b.ReportAllocs()
				pool := sync.Pool{New: func() interface{} { return new(hyperpb.Shared) }}
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					ctx := pool.Get().(*hyperpb.Shared)
					m := ctx.NewMessage(test.Type.Fast)
					runtime.AddCleanup(m, func(ctx *hyperpb.Shared) {
						ctx.Free()
						pool.Put(ctx)
					}, ctx)
					_ = m.Unmarshal(specimen, hyperpb.WithAllowAlias(true))
					ctx.Free()
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("pgo", func(b *testing.B) {
				b.ReportAllocs()
				ctx := new(hyperpb.Shared)

				// Warmup.
				profile := test.Type.Fast.NewProfile()
				for _, specimen := range specimens {
					for range 16 {
						m := ctx.NewMessage(test.Type.Fast)
						_ = m.Unmarshal(specimen,
							hyperpb.WithAllowAlias(true),
							hyperpb.WithRecordProfile(profile, 1.0),
						)
						ctx.Free()
					}
				}
				ty := test.Type.Fast.Recompile(profile)

				o := hyperpb.WithAllowAlias(true)
				b.ResetTimer()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := ctx.NewMessage(ty)
					_ = m.Unmarshal(specimen, o)
					ctx.Free()
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("pgo-w-pool", func(b *testing.B) {
				b.ReportAllocs()
				pool := sync.Pool{New: func() interface{} { return new(hyperpb.Shared) }}

				// Warmup.
				profile := test.Type.Fast.NewProfile()
				for _, specimen := range specimens {
					for range 16 {
						ctx := pool.Get().(*hyperpb.Shared)
						m := ctx.NewMessage(test.Type.Fast)
						runtime.AddCleanup(m, func(ctx *hyperpb.Shared) {
							ctx.Free()
							pool.Put(ctx)
						}, ctx)
						_ = m.Unmarshal(specimen,
							hyperpb.WithAllowAlias(true),
							hyperpb.WithRecordProfile(profile, 1.0),
						)
					}
				}
				ty := test.Type.Fast.Recompile(profile)

				o := hyperpb.WithAllowAlias(true)
				b.ResetTimer()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					ctx := pool.Get().(*hyperpb.Shared)
					m := ctx.NewMessage(ty)
					runtime.AddCleanup(m, func(ctx *hyperpb.Shared) {
						ctx.Free()
						pool.Put(ctx)
					}, ctx)
					_ = m.Unmarshal(specimen, o)
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("gencode", func(b *testing.B) {
				b.ReportAllocs()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := test.Type.Gencode.New().Interface()
					_ = proto.Unmarshal(specimen, m)
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("vtproto", func(b *testing.B) {
				type vtMessage interface{ UnmarshalVTUnsafe([]byte) error }
				if _, ok := test.Type.Gencode.New().Interface().(vtMessage); !ok {
					b.SkipNow()
				}

				b.ReportAllocs()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := test.Type.Gencode.New().Interface().(vtMessage) //nolint:errcheck
					_ = m.UnmarshalVTUnsafe(specimen)
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
			b.Run("dynamicpb", func(b *testing.B) {
				b.ReportAllocs()
				var totalBytes int64
				for i := range b.N {
					specimen := specimens[i%len(specimens)]
					totalBytes += int64(len(specimen))
					m := dynamicpb.NewMessage(test.Type.Gencode.Descriptor())
					_ = proto.Unmarshal(specimen, m)
				}
				b.SetBytes(totalBytes / int64(b.N))
			})
		}

		run(b, test.Specimens)
	})
}
