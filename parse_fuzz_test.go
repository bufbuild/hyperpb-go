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
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/bufbuild/hyperpb"
	testpb "github.com/bufbuild/hyperpb/internal/gen/test"
	"github.com/bufbuild/hyperpb/internal/testdata"
	"github.com/bufbuild/hyperpb/internal/xsync"
)

var contexts = xsync.Pool[hyperpb.Shared]{Reset: (*hyperpb.Shared).Free}

func FuzzScalars(f *testing.F)    { fuzz[*testpb.Scalars](f) }
func FuzzRepeated(f *testing.F)   { fuzz[*testpb.Repeated](f) }
func FuzzGraph(f *testing.F)      { fuzz[*testpb.Graph](f) }
func FuzzOneof(f *testing.F)      { fuzz[*testpb.Oneof](f) }
func FuzzDescriptor(f *testing.F) { fuzz[*descriptorpb.FileDescriptorProto](f) }
func FuzzStruct(f *testing.F)     { fuzz[*structpb.Value](f) }
func FuzzEmpty(f *testing.F)      { fuzz[*emptypb.Empty](f) }

func fuzz[M proto.Message](f *testing.F) {
	f.Helper()

	var z M
	test := new(testdata.TestCase)
	test.Type.Gencode = z.ProtoReflect().Type()
	test.Type.Fast = hyperpb.CompileForDescriptor(test.Type.Gencode.Descriptor())

	f.Fuzz(func(t *testing.T, b []byte) {
		ctx := contexts.Get()
		defer contexts.Put(ctx)

		test := *test
		test.Specimens = [][]byte{b}
		test.Run(t, ctx, verbose)
	})
}
