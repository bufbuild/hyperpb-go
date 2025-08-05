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

package gencode_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	testpb "buf.build/go/hyperpb/internal/gen/test"
	testhy "buf.build/go/hyperpb/internal/gen/test/hyper-test"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	t.Run("ints", func(t *testing.T) {
		t.Parallel()

		want := &testpb.Scalars{
			A1: 42,
			B1: proto.Int32(42),
		}
		data, err := proto.Marshal(want)
		require.NoError(t, err)

		got := testhy.NewScalars(nil)
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		assert.Equal(t, want.GetA1(), got.GetA1())
		assert.Equal(t, want.GetB1(), got.GetB1())
		assert.True(t, got.HasB1())
	})

	t.Run("repeated", func(t *testing.T) {
		t.Parallel()

		want := &testpb.Repeated{
			R1: []int32{math.MinInt32, math.MaxInt32},
			R4: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			R7: []string{"foo", "bar", "baz"},
		}
		data, err := proto.Marshal(want)
		require.NoError(t, err)

		got := testhy.NewRepeated(nil)
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		assert.Equal(t, want.R1, got.GetR1().Copy(nil))
		assert.Equal(t, want.R4, got.GetR4().Copy(nil))
		assert.Equal(t, want.R7, got.GetR7().Copy(nil))
	})

	t.Run("messages", func(t *testing.T) {
		t.Parallel()

		want := &testpb.Graph{
			V: 1,
			S: &testpb.Graph{
				V: 2,
				R: []*testpb.Graph{
					{V: 3},
					{V: 4},
				},
			},
		}
		data, err := proto.Marshal(want)
		require.NoError(t, err)

		got := testhy.NewGraph(nil)
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		assert.Equal(t, want.V, got.GetV())
		assert.Equal(t, want.S.V, got.GetS().GetV())
		assert.Equal(t, len(want.R), got.GetR().Len())
		assert.Equal(t, len(want.S.R), got.GetS().GetR().Len())
		for i, g1 := range want.S.R {
			g2 := got.GetS().GetR().Get(i)
			assert.Equal(t, g1.V, g2.GetV())
			assert.Nil(t, g2.GetS())
			assert.Equal(t, int32(0), g2.GetS().GetV())
			assert.Equal(t, len(g1.R), g2.GetR().Len())
		}
	})
}
