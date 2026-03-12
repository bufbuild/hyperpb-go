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

package hyperpb

import (
	"math"

	"google.golang.org/protobuf/reflect/protoregistry"

	"buf.build/go/hyperpb/internal/tdp/compiler"
	"buf.build/go/hyperpb/internal/tdp/vm"
)

// The below are not interfaces because of https://github.com/golang/go/issues/74356,
// and because the With*() functions for unmarshaling are unfortunately part of
// the critical path. CompileOption is the same for symmetry, because having it
// be an interface while UnmarshalOption isn't would be weird.

// CompileOption is a configuration setting for [CompileMessageDescriptor].
type CompileOption struct{ apply func(*compiler.Options) }

// WithExtensions provides an extension resolver for a compiler.
//
// Unlike ordinary Protobuf parsers, hyperpb does not perform extension
// resolution on the fly. Instead, any extensions that should be parsed must
// be provided up-front.
func WithExtensions(resolver compiler.ExtensionResolver) CompileOption {
	return CompileOption{func(c *compiler.Options) { c.Extensions = resolver }}
}

// WithExtensionsFromTypes uses a type registry to provide extension information
// about a message type.
func WithExtensionsFromTypes(types *protoregistry.Types) CompileOption {
	return CompileOption{func(c *compiler.Options) { c.Extensions = (*compiler.ExtensionsFromRegistry)(types) }}
}

// WithExtensionsFromFiles uses a file registry to provide extension information
// about a message type.
func WithExtensionsFromFiles(files *protoregistry.Files) CompileOption {
	return CompileOption{func(c *compiler.Options) { c.Extensions = compiler.ExtensionsFromFile(files) }}
}

// WithProfile provides a profile for profile-guided optimization.
//
// Typically, you'll prefer to use [MessageType.Recompile].
func WithProfile(profile *Profile) CompileOption {
	return CompileOption{func(c *compiler.Options) { c.Profile = &profile.impl }}
}

// UnmarshalOption is a configuration setting for [Message.Unmarshal].
type UnmarshalOption struct{ apply func(*vm.Options) }

// WithMaxDecodeMisses sets the number of decode misses allowed in the parser before
// switching to the slow path.
//
// Large values may improve performance for common protos, but introduce a
// potential DoS vector due to quadratic worst case performance. The default
// is 4.
func WithMaxDecodeMisses(maxMisses int) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) { opts.MaxMisses = maxMisses }}
}

// WithMaxDepth sets the maximum recursion depth for the parser.
//
// Setting a large value enables potential DoS vectors.
func WithMaxDepth(depth int) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) { opts.MaxDepth = min(depth, math.MaxUint32) }}
}

// WithDiscardUnknown sets whether unknown fields should be discarded while
// parsing. Analogous to [proto.UnmarshalOptions].
//
// Setting this option will break round-tripping, but will also improve parse
// speeds of messages with many unknown fields.
func WithDiscardUnknown(discard bool) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) { opts.DiscardUnknown = discard }}
}

// WithAllowInvalidUTF8 sets whether UTF-8 is validated when parsing string
// fields originating from non-proto2 files.
func WithAllowInvalidUTF8(allow bool) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) { opts.AllowInvalidUTF8 = allow }}
}

// WithAllowAlias sets whether aliasing the input buffer is allowed. This avoids
// an expensive copy at the start of parsing.
//
// Analogous to [protoimpl.UnmarshalAliasBuffer].
func WithAllowAlias(allow bool) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) { opts.AllowAlias = allow }}
}

// WithRecordProfile sets a profiler for an unmarshaling operation. Rate is a
// value from 0 to 1 that specifies the sampling rate. profile may be nil, in
// which case nothing will be recorded.
//
// Profiling should be done with many, many message types, all with the same
// rate. This will allow the profiler to collect statistically relevant data,
// which can be used to recompile this type to be more efficient using
// [MessageType.Recompile].
func WithRecordProfile(profile *Profile, rate float64) UnmarshalOption {
	return UnmarshalOption{func(opts *vm.Options) {
		if profile == nil {
			opts.Recorder = nil
		} else {
			opts.Recorder = &profile.impl
		}
		opts.ProfileRate = rate
	}}
}
