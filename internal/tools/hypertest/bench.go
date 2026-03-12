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

package main

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// prefixes is a list of SI prefixes.
var prefixes = []struct {
	prefix string
	mult   float64
}{
	{"E", 1e18},
	{"P", 1e15},
	{"T", 1e12},
	{"G", 1e9},
	{"M", 1e6},
	{"k", 1e3},
	{" ", 1e0},
	{"m", 1e-3},
	{"μ", 1e-6},
	{"n", 1e-9},
	{"p", 1e-12},
}

type (
	benchReport struct {
		// Grouped by subtest (i.e., the last component of the path).
		benches  [][]bench
		columns  []column
		subtests []string
	}

	bench struct {
		name, subtest string
		fields        []string
		metrics       []metric // Indexed by column order.
	}

	column struct {
		name  string
		order int
		units string
	}

	metric struct {
		formatted string
		value     float64
	}
)

func parseBenchmarkOutput(stdout string) *benchReport {
	r := new(benchReport)

	var prev string
	subtests := map[string]int{}
	for line := range strings.SplitSeq(stdout, "\n") {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		// Split each benchmark into fields. Each field is separated by tabs.
		fields := strings.Split(line, "\t")

		name := fields[0]
		// Trim off a trailing -n, since it's not especially
		// interesting.
		name = name[:strings.LastIndex(name, "-")]
		name = strings.TrimPrefix(name, "Benchmark")

		// Delete all occurrences of .yaml.
		name = strings.ReplaceAll(name, ".yaml", "")

		b := bench{
			name:   name,
			fields: fields[2:], // Skip the trial count at index 1.
		}

		slash := strings.LastIndex(b.name, "/")
		if slash != -1 {
			b.subtest = b.name[slash+1:]
			b.name = b.name[:slash]

			if len(r.benches) > 0 {
				subtests[b.subtest] = len(r.benches[len(r.benches)-1])
			}

			if prev == b.name {
				s := &r.benches[len(r.benches)-1]
				*s = append(*s, b)
				continue
			} else {
				prev = b.name
			}
		}
		r.benches = append(r.benches, []bench{b})
	}

	for subtest := range subtests {
		r.subtests = append(r.subtests, subtest)
	}
	slices.SortFunc(r.subtests, func(a, b string) int {
		if d := cmp.Compare(subtests[a], subtests[b]); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})

	type key struct {
		column string
		row    int
	}

	values := map[key]metric{}
	columns := map[string]column{}
	var k int
	for _, bs := range r.benches {
		for j := range bs {
			b := &bs[j]

			// Now, convert the fields into metric cells.
			for j := range b.fields {
				b.fields[j] = strings.TrimSpace(b.fields[j])
				if b.fields[j] == "" {
					continue
				}

				if b.fields[j][0] < '0' || b.fields[j][1] > '9' {
					continue
				}

				num, unit, ok := strings.Cut(b.fields[j], " ")
				if !ok {
					continue
				}

				v, err := strconv.ParseFloat(num, 64)
				if err != nil {
					panic(err)
				}

				unit = strings.TrimSuffix(unit, "/op")
				what := unit
				switch unit {
				// Normalize some units.
				case "ns":
					what = "time"
					unit = "s"
					v *= 1e-9
				case "MB/s":
					what = "throughput"
					unit = "B/s"
					v *= 1e6
				case "B":
					what = "memory"
				case "allocs":
					what = "allocations"
				default:
					idx := strings.LastIndex(unit, "/")
					if idx > 0 {
						unit = unit[:idx]
					}
				}

				// Pick the largest unit prefix smaller than field.units.
				exact := v
				if v == 0 {
					unit = " " + unit
				} else {
					for _, prefix := range prefixes {
						if prefix.mult <= v {
							v /= prefix.mult
							unit = prefix.prefix + unit
							break
						}
					}
				}

				values[key{what, k}] = metric{
					formatted: fmt.Sprintf("%.03f %v", v, unit),
					value:     exact,
				}
				columns[what] = column{
					name:  what,
					order: max(j, columns[what].order),
				}
			}
			k++
		}
	}

	// Realize an order for the columns.
	r.columns = map2slice(nil, columns, func(c1, c2 column) int {
		if d := cmp.Compare(c1.order, c2.order); d != 0 {
			return d
		}
		return cmp.Compare(c1.name, c2.name)
	})

	// Generate the metrics matrix.
	k = 0
	for _, bs := range r.benches {
		for j := range bs {
			b := &bs[j]

			b.metrics = make([]metric, len(r.columns))
			for j, c := range r.columns {
				b.metrics[j] = values[key{c.name, k}]
			}
			k++
		}
	}

	return r
}

//nolint:prealloc
func (r *benchReport) toCSV(w io.Writer) error {
	subtests := map[string]int{}
	for i, s := range r.subtests {
		subtests[s] = i
	}

	indices := map[[2]int]int{}
	header := []string{"benchmark"}
	for i, c := range r.columns {
		for j, subtest := range r.subtests {
			indices[[2]int{i, j}] = len(header)
			header = append(header, c.name+"/"+subtest)
		}
	}

	cells := [][]string{header}
	for _, bs := range r.benches {
		row := make([]string, len(header))
		cells = append(cells, row)
		for _, b := range bs {
			row[0] = b.name
			j := subtests[b.subtest]
			for i, m := range b.metrics {
				row[indices[[2]int{i, j}]] = strconv.FormatFloat(m.value, 'f', -1, 64)
			}
		}
	}

	return csv.NewWriter(w).WriteAll(cells)
}

func (r *benchReport) toMarkdown(w io.Writer) error {
	// Lay out the header.
	table := [][]string{make([]string, len(r.columns)+2)}

	header := table[0]
	header[0], header[1], header = "benchmark", "sub", header[2:]
	for i, col := range r.columns {
		header[i] = col.name
	}

	// Lay out the pretty table.
	for _, bs := range r.benches {
		for i, b := range bs {
			row := make([]string, len(r.columns)+2)
			if i == 0 {
				row[0] = b.name
			}
			row[1] = b.subtest
			table = append(table, row)

			row = row[2:]
			for i, m := range b.metrics {
				row[i] = m.formatted
				if m.formatted == "" {
					row[i] = "n/a  " + r.columns[i].units
				}
			}
		}
	}

	widths := make([]int, len(table[0]))
	for _, fields := range table {
		for i, field := range fields {
			widths[i] = max(widths[i], utf8.RuneCountInString(field))
		}
	}

	// Round all the widths up to a multiple of 2.
	for i := range widths {
		widths[i]++
		widths[i] &^= 1
	}

	// Print the table.
	for i, fields := range table {
		var err error
		if i > 0 && fields[0] != "" {
			for i := range fields {
				if i > 0 {
					if _, err = fmt.Fprint(w, " "); err != nil {
						return err
					}
				}

				if i < 2 {
					_, err = fmt.Fprintf(w, "| :%s", strings.Repeat("-", widths[i]-1))
				} else {
					_, err = fmt.Fprintf(w, "| %s:", strings.Repeat("-", widths[i]-1))
				}
				if err != nil {
					return err
				}
			}

			if _, err = fmt.Fprint(w, " |\n"); err != nil {
				return err
			}
		}

		for i, field := range fields {
			if i > 0 {
				if _, err = fmt.Fprint(w, " "); err != nil {
					return err
				}
			}

			if i < 2 {
				_, err = fmt.Fprintf(w, "| %s%*s",
					field, widths[i]-utf8.RuneCountInString(field), "")
			} else {
				_, err = fmt.Fprintf(w, "| %+*s", widths[i], field)
			}
			if err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintln(w, " |"); err != nil {
			return err
		}
	}

	return nil
}

func map2slice[K cmp.Ordered, V any](s []V, m map[K]V, sort func(V, V) int) []V {
	s = slices.Grow(s, len(m))
	n := len(s)

	for _, v := range m {
		s = append(s, v)
	}

	slices.SortStableFunc(s[n:], sort)
	return s
}
