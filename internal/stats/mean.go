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

package stats

import "github.com/bufbuild/hyperpb/internal/xsync"

// Mean tracks an average statistic.
//
// The zero value is ready to use. [Mean.Record] may be called concurrently,
// but not with [Mean.Get].
type Mean struct {
	total, samples xsync.AtomicFloat64
}

// Record records a sample.
func (m *Mean) Record(sample float64) {
	m.total.Add(sample)
	m.samples.Add(1)
}

// Get returns the mean value of this statistic.
func (m *Mean) Get() float64 {
	total, samples := m.total.Load(), m.samples.Load()
	if samples == 0 {
		return 0
	}
	return total / samples
}

// Merge adds all of the samples from that to m.
func (m *Mean) Merge(that *Mean) {
	m.total.Add(that.total.Load())
	m.samples.Add(that.samples.Load())
}
