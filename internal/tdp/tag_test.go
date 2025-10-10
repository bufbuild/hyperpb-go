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

package tdp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protowire"

	"buf.build/go/hyperpb/internal/tdp"
)

func TestTagOverflows(t *testing.T) {
	t.Parallel()
	tag := tdp.EncodeTag(protowire.MaxValidNumber, protowire.BytesType)
	assert.False(t, tag.Overflows(), "protowire.MaxValidNumber should not overflow")

	tag = tdp.EncodeTag(protowire.MaxValidNumber+1, protowire.BytesType)
	assert.True(t, tag.Overflows(), "protowire.MaxValidNumber+1 should overflow")
}
