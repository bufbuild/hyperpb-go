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

// Package options defines the options for the VM.
package options

import "buf.build/go/hyperpb/internal/tdp/profile"

// Options is options for [Run].
type Options struct {
	// Max tries before hitting the tag table.
	MaxMisses int

	// Maximum recursion depth.
	MaxDepth int

	// If set, unknown fields are discarded.
	DiscardUnknown bool

	// If set, all string fields behave as if they are defined in proto2.
	AllowInvalidUTF8 bool

	// If set, the input data will not be copied before the parse begins.
	AllowAlias bool

	// Profiler fields.
	Recorder    *profile.Recorder
	ProfileRate float64
}

// Defaults returns the default settings for [Options].
func Defaults() Options {
	return Options{
		MaxMisses: 4,
		MaxDepth:  1000,
	}
}
