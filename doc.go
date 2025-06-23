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

// Package hyperpb is a highly optimized dynamic message library for Protobuf or read-only
// workloads. It is designed to be a drop-in replacement for [dynamicpb],
// protobuf-go's canonical solution for working with completely dynamic messages.
//
// hyperpb's parser is an efficient VM for a special instruction set, a variant of
// table-driven parsing (TDP), pioneered by [the UPB project].
//
// Our parser is very fast, beating dynamicpb by 10x, and often beating
// protobuf-go's generated code by a factor of 2-3x, especially for workloads with
// many nested messages.
//
// # Usage
//
// The core conceit of hyperpb is that you must pre-compile a parser using
// hyperpb.[Compile] at runtime, much like regular expressions require that you use
// [regexp.Compile] them. Doing this allows hyperpb to run optimization passes on
// your message, and delaying it to runtime allows us to continuously improve
// layout optimizations, without making any source-breaking changes.
//
// For example, let's say that we want to compile a parser for some type baked into
// our binary, and parse some data with it.
//
// For example, let's say that we want to compile a parser for some type baked into
// our binary, and parse some data with it.
//
//	//Compile a type for your message. Make sure to cache this!
//	ty := hyperpb.CompileFor[*weatherv1.WeatherReport]()
//
//	data := /* ... */
//
//	// Allocate a fresh message using that type.
//	msg := hyperpb.New(ty)
//
//	// Parse the message, using proto.Unmarshal like any other message type.
//	if err := proto.Unmarshal(data, msg); err != nil {
//		// Handle parse failure.
//	}
//
//	// Use reflection to read some fields. hyperpb currently only supports access
//	// by reflection. You can also look up fields by index using fields.Get(), which
//	// is less legible but doesn't hit a hashmap.
//	fields := ty.Descriptor().Fields()
//
//	// Get returns a protoreflect.Value, which can be printed directly...
//	fmt.Println(msg.Get(fields.ByName("region")))
//
//	// ... or converted to an explicit type to operate on, such as with List(),
//	// which converts a repeated field into something with indexing operations.
//	stations := msg.Get(fields.ByName("weather_stations")).List()
//	for i := range stations.Len() {
//		// Get returns a protoreflect.Value too, so we need to convert it into
//		// a message to keep extracting fields.
//		station := stations.Get(i).Message()
//		fields := station.Descriptor().Fields()
//
//		// Here we extract each of the fields we care about from the message.
//		// Again, we could use fields.Get if we know the indices.
//		fmt.Println("station:", station.Get(fields.ByName("station")))
//		fmt.Println("frequency:", station.Get(fields.ByName("frequency")))
//		fmt.Println("temperature:", station.Get(fields.ByName("temperature")))
//		fmt.Println("pressure:", station.Get(fields.ByName("pressure")))
//		fmt.Println("wind_speed:", station.Get(fields.ByName("wind_speed")))
//		fmt.Println("conditions:", station.Get(fields.ByName("conditions")))
//	}
//
// Currently, hyperpb only supports manipulating messages through the reflection
// API; it shines best when you need write a very generic service that
// downloads types off the network and parses messages using those types, which
// forces you to use reflection.
//
// Mutation is currently not supported; any operation which would mutate an
// already-parsed message will panic. Which methods of [Message] panic
// is included in the documentation.
//
// # Memory Reuse
//
// hyperpb has a memory-reuse mechanism that side-steps the Go garbage
// collector for improved allocation latency. [Shared] is book-keeping
// state and resources shared by all messages resulting from the same parse.
// After the message goes out of scope, these resources are ordinarily reclaimed
// by the garbage collector.
//
// However, a [Shared] can be retained after its associated message goes
// away, allowing for re-use. Consider the following example of a request
// handler:
//
//	type requestContext struct {
//	    shared *hyperpb.Shared
//	    types map[string]*hyperpb.Type
//	    // ...
//	}
//
//	func (c *requestContext) Handle(req Request) {
//	    // ...
//	    ty := types[req.Type]
//	    msg := c.shared.New(ty)
//	    defer c.shared.Free()
//
//	    c.process(msg, req, ...)
//	}
//
// Beware that msg must not outlive the call to [Shared.Free]; failure to do so
// will result in memory errors that Go cannot protect you from.
//
// # Compatibility
//
// hyperpb is experimental software, and the API may change drastically before
// v1. It currently implements all Protobuf language constructs. It does not
// implement mutation of parsed messages, however.
//
// [the UPB project]: https://github.com/protocolbuffers/protobuf/tree/main/upb
package hyperpb

import (
	"google.golang.org/protobuf/types/dynamicpb" // For doc links.

	_ "github.com/bufbuild/hyperpb/internal/unsafe2/support"
)

var _ = dynamicpb.Message{} // Force the dynamicpb import to be kept.
