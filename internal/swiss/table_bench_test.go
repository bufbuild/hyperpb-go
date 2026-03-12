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

package swiss_test

import (
	_ "embed"
	"flag"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"buf.build/go/hyperpb/internal/arena"
	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/xunsafe"
)

const (
	mapSize = 2048
	numKeys = 1024
)

var benchProbes = flag.Bool("hyperpb.benchprobe", false, "if true, benchmark probe sequence length")

//go:generate go tool hyperstencil

func BenchmarkTable(b *testing.B) {
	u32Benchmark(b, uint32s{}, mapSize)
	u64Benchmark(b, uint64s{}, mapSize)
	u64Benchmark(b, highBits{}, mapSize)
	u64Benchmark(b, lowBits{}, mapSize)

	u32Benchmark(b, new(uuids), mapSize)
	u32Benchmark(b, new(kilobytes), mapSize)
	u32Benchmark(b, new(english), mapSize)
	u32Benchmark(b, new(urls), mapSize)
}

//hyperpb:stencil u32Benchmark scalarBenchmark[uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32 Lookup -> swiss.LookupU32xU32
//hyperpb:stencil u64Benchmark scalarBenchmark[uint64] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32 Lookup -> swiss.LookupU64xU32
func scalarBenchmark[K swiss.Key](b *testing.B, c corpus[K], mapSize int) {
	b.Helper()

	r := rand.New(runtimeSource{})
	b.Run(strings.Trim(fmt.Sprintf("%Tx%d", c, mapSize), "*"), func(b *testing.B) {
		var extract func(K) []byte
		if e, ok := c.(extractor[K]); ok {
			extract = e.extract
		}

		theirsScalar := make(map[K]uint32)
		theirsString := make(map[string]uint32)

		entries := make([]swiss.Entry[K, uint32], mapSize)
		for i := range entries {
			k := c.sample(r, false)
			entries[i].Key = k
			entries[i].Value = uint32(i)
			if extract != nil {
				theirsString[xunsafe.SliceToString(extract(k))] = uint32(i)
			} else {
				theirsScalar[k] = uint32(i)
			}
		}
		_, ours := swiss.New(nil, extract, entries...)
		metrics := new(swiss.Metrics)
		if *benchProbes {
			ours.Record(metrics)
		}

		lookup := func(b *testing.B, miss bool) {
			b.Helper()

			lookup := make([]K, numKeys)
			for i := range lookup {
				lookup[i] = c.sample(r, miss)
			}

			b.Run("swiss", func(b *testing.B) {
				metrics.Reset()
				if extract != nil {
					for range b.N {
						for _, k := range lookup {
							ours.LookupFunc(extract(k), extract)
						}
					}
				} else {
					for range b.N {
						for _, k := range lookup {
							ours.Lookup(k)
						}
					}
				}

				if *benchProbes {
					metrics.Report(b)
				}
			})

			b.Run("gomap", func(b *testing.B) {
				if extract != nil {
					for range b.N {
						for _, k := range lookup {
							_ = theirsString[xunsafe.SliceToString(extract(k))]
						}
					}
				} else {
					for range b.N {
						for _, k := range lookup {
							_ = theirsScalar[k]
						}
					}
				}
			})
		}

		b.Run("hit", func(b *testing.B) { lookup(b, false) })
		b.Run("miss", func(b *testing.B) { lookup(b, true) })

		entries = slices.Repeat(entries, 10)
		r.Shuffle(len(entries), func(i, j int) {
			entries[i], entries[j] = entries[j], entries[i]
		})

		b.Run("build", func(b *testing.B) {
			b.Run("swiss", func(b *testing.B) {
				metrics := new(swiss.Metrics)
				arena := new(arena.Arena)
				arena.KeepAlive(metrics)

				b.ResetTimer()
				for range b.N {
					size, _ := swiss.Layout[K, uint32](0)
					m := xunsafe.Cast[swiss.Table[K, uint32]](arena.Alloc(size))
					m.Init(0, nil, extract)
					if *benchProbes {
						m.Record(metrics)
					}
					for _, kv := range entries {
						v := m.Insert(kv.Key, extract)
						if v == nil {
							size, _ := swiss.Layout[K, uint32](m.Len() + 1)
							m2 := xunsafe.Cast[swiss.Table[K, uint32]](arena.Alloc(size))
							m2.Init(m.Len()+1, m, extract)
							m = m2
							v = m.Insert(kv.Key, extract)
						}
						*v = kv.Value
					}

					arena.Free()
				}

				if *benchProbes {
					metrics.Report(b)
				}
			})

			b.Run("gomap", func(b *testing.B) {
				if extract != nil {
					for range b.N {
						gomap := make(map[string]uint32)
						for _, kv := range entries {
							k := xunsafe.SliceToString(extract(kv.Key))
							gomap[k] = kv.Value
						}
					}
				} else {
					for range b.N {
						gomap := make(map[K]uint32)
						for _, kv := range entries {
							gomap[kv.Key] = kv.Value
						}
					}
				}
			})
		})
	})
}

// corpus is a corpus of values to draw from for hammering a table.
type corpus[K swiss.Key] interface {
	sample(r *rand.Rand, missing bool) K
}

// extractor is implemented by corpora that use one of the Table.*Func()
// functions.
type extractor[K swiss.Key] interface {
	corpus[K]
	extract(K) []byte
}

type uint32s struct{}

func (uint32s) sample(r *rand.Rand, missing bool) uint32 {
	n := r.Uint32()
	if missing {
		n |= 1
	} else {
		n &^= 1
	}
	return n
}

type uint64s struct{}

func (uint64s) sample(r *rand.Rand, missing bool) uint64 {
	n := r.Uint64()
	if missing {
		n |= 1
	} else {
		n &^= 1
	}
	return n
}

type highBits struct{}

func (highBits) sample(r *rand.Rand, missing bool) uint64 {
	n := uint64s{}.sample(r, missing)
	n <<= 48
	return n
}

type lowBits struct{}

func (lowBits) sample(r *rand.Rand, missing bool) uint64 {
	n := uint64s{}.sample(r, missing)
	n &= 0xffff
	return n
}

type uuids []string

func (u *uuids) sample(r *rand.Rand, missing bool) uint32 {
	if *u == nil {
		*u = make([]string, 20000)
		for i := range 10000 {
			uuid := uuid.New()

			uuid[0] |= 1
			(*u)[2*i] = uuid.String()
			uuid[0] &^= 1
			(*u)[2*i+1] = uuid.String()
		}
	}

	n := 2 * r.Uint32N(10000)
	if missing {
		n++
	}
	return n
}

func (u *uuids) extract(k uint32) []byte {
	return xunsafe.StringToSlice[[]byte]((*u)[k])
}

type kilobytes []string

func (d *kilobytes) sample(r *rand.Rand, missing bool) uint32 {
	if *d == nil {
		*d = make([]string, 20000)
		for i := range 20000 {
			buf := new(strings.Builder)
			for range 1024 {
				b := byte(r.Int())
				if i%2 == 0 {
					b |= 1
				} else {
					b &^= 1
				}
				buf.WriteByte(b)
			}
			(*d)[i] = buf.String()
		}
	}

	n := 2 * r.Uint32N(10000)
	if missing {
		n++
	}
	return n
}

func (d *kilobytes) extract(k uint32) []byte {
	return xunsafe.StringToSlice[[]byte]((*d)[k])
}

var (
	//go:embed benchdata/urls.txt
	urlTxt string
	//go:embed benchdata/english.txt
	englishTxt string

	urlSlice, englishSlice []string
)

func init() {
	urlSlice = strings.Split(strings.TrimSpace(urlTxt), "\n")
	englishSlice = strings.Split(strings.TrimSpace(englishTxt), "\n")
}

type urls struct{}

func (urls) sample(r *rand.Rand, missing bool) uint32 {
	n := 2 * r.Uint32N(uint32(len(urlSlice))/2)
	if missing {
		n++
	}
	return n
}

func (urls) extract(k uint32) []byte {
	return xunsafe.StringToSlice[[]byte](urlSlice[k])
}

type english struct{}

func (english) sample(r *rand.Rand, missing bool) uint32 {
	n := 2 * r.Uint32N(uint32(len(englishSlice))/2)
	if missing {
		n++
	}
	return n
}

func (english) extract(k uint32) []byte {
	return xunsafe.StringToSlice[[]byte](englishSlice[k])
}

type runtimeSource struct{}

func (runtimeSource) Uint64() uint64 { return rand.Uint64() }
