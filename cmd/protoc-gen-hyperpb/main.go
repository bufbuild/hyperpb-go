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

// Binary protoc-gen-hyperpb is a protoc plugin that generates Protobuf APIs for
// working with specific hyperpb-compiled messages without reflection.
//
// The generated API follows, but is not a drop-in replacement for, the opaque
// Go API, particularly returning non-slice values for accessing repeated
// fields.
//
// All message types emitted by this generator export the full interface of
// [hyperpb.Message], which can be used to access optimized operations, such
// as [hyperpb.Message.Unmarshal].
package main

import (
	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/gencode/generator"
)

func main() { generator.Main() }

var _ hyperpb.Message // For documentation links.
