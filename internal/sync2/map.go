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

package sync2

import (
	"iter"
	"sync"
)

// Map is a strongly-typed wrapper over sync.Map.
type Map[K comparable, V any] struct {
	impl sync.Map
}

// Load forwards to [sync.Map.Load].
func (m *Map[K, V]) Load(k K) (V, bool) {
	v, ok := m.impl.Load(k)
	if !ok {
		var z V
		return z, ok
	}

	return v.(V), ok //nolint:errcheck
}

// LoadOrStore loads a value if its present, or constructs it with make and
// inserts it.
//
// There is a possibility that make is called, but the return value is not
// inserted.
func (m *Map[K, V]) LoadOrStore(k K, make func() V) (actual V, loaded bool) {
	v, ok := m.Load(k)
	if ok {
		return v, true
	}
	w, ok := m.impl.LoadOrStore(k, make())
	return w.(V), ok //nolint:errcheck
}

// All returns an iterator over the values in this map, using [sync.Map.Range].
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.impl.Range(func(key, value any) bool {
			return yield(key.(K), value.(V)) //nolint:errcheck
		})
	}
}
