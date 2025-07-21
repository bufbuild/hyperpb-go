![The Buf logo](https://raw.githubusercontent.com/bufbuild/hyperpb-go/main/.github/buf-logo.svg)

# hyperpb

`hyperpb` is a highly optimized dynamic message library for Protobuf or read-only
workloads. It is designed to be a drop-in replacement for
[`dynamicpb`][dynamicpb],
`protobuf-go`'s canonical solution for working with completely dynamic messages.

`hyperpb`'s parser is an efficient VM for a special instruction set, a variant of
table-driven parsing (TDP), pioneered by [the UPB project][upb].

Our parser is very fast, beating `dynamicpb` by 10x, and often beating
`protobuf-go`'s generated code by a factor of 2-3x, especially for workloads with
many nested messages.

![Barchart of benchmarks for hyperpb](https://raw.githubusercontent.com/bufbuild/hyperpb-go/main/.github/benchmarks.png)

Here, we show two benchmark variants for `hyperpb`: out-of-the-box performance with no optimizations turned on, and real-time profile-guided optimization (PGO) with all optimizations we currently offer enabled.

## Usage

The core conceit of `hyperpb` is that you must pre-compile a parser using
`hyperpb.Compile` at runtime, much like regular expressions require that you
`regexp.Compile` them. Doing this allows `hyperpb` to run optimization passes on
your message, and delaying it to runtime allows us to continuously improve
layout optimizations, without making any source-breaking changes.

For example, let's say that we want to compile a parser for some type baked into
our binary, and parse some data with it.

<!-- weatherDataBytes values match data used in example_test.go and should be kept in sync -->

```go
package main

import (
    "fmt"
    "log"

    "buf.build/go/hyperpb"
    "google.golang.org/protobuf/proto"

    weatherv1 "buf.build/gen/go/bufbuild/hyperpb-examples/protocolbuffers/go/example/weather/v1"
)

// Byte slice representation of a valid *weatherv1.WeatherReport.
var weatherDataBytes = []byte{
    0x0a, 0x07, 0x53, 0x65, 0x61, 0x74, 0x74, 0x6c,
    0x65, 0x12, 0x1d, 0x0a, 0x05, 0x4b, 0x41, 0x44,
    0x39, 0x33, 0x15, 0x66, 0x86, 0x22, 0x43, 0x1d,
    0xcd, 0xcc, 0x34, 0x41, 0x25, 0xd7, 0xa3, 0xf0,
    0x41, 0x2d, 0x33, 0x33, 0x13, 0x40, 0x30, 0x03,
    0x12, 0x1d, 0x0a, 0x05, 0x4b, 0x48, 0x42, 0x36,
    0x30, 0x15, 0xcd, 0x8c, 0x22, 0x43, 0x1d, 0x33,
    0x33, 0x5b, 0x41, 0x25, 0x52, 0xb8, 0xe0, 0x41,
    0x2d, 0x33, 0x33, 0xf3, 0x3f, 0x30, 0x03,
}

func main() {
    // Compile a type for your message. Make sure to cache this!
    // Here, we're using a compiled-in descriptor.
    msgType := hyperpb.CompileMessageDescriptor(
        (*weatherv1.WeatherReport)(nil).ProtoReflect().Descriptor(),
    )

    // Allocate a fresh message using that type.
    msg := hyperpb.NewMessage(msgType)

    // Parse the message, using proto.Unmarshal like any other message type.
    if err := proto.Unmarshal(weatherDataBytes, msg); err != nil {
        // Handle parse failure.
        log.Fatalf("failed to parse weather data: %v", err)
    }

    // Use reflection to read some fields. hyperpb currently only supports access
    // by reflection. You can also look up fields by index using fields.Get(), which
    // is less legible but doesn't hit a hashmap.
    fields := msgType.Descriptor().Fields()

    // Get returns a protoreflect.Value, which can be printed directly...
    fmt.Println(msg.Get(fields.ByName("region")))

    // ... or converted to an explicit type to operate on, such as with List(),
    // which converts a repeated field into something with indexing operations.
    stations := msg.Get(fields.ByName("weather_stations")).List()
    for i := range stations.Len() {
        // Get returns a protoreflect.Value too, so we need to convert it into
        // a message to keep extracting fields.
        station := stations.Get(i).Message()
        fields := station.Descriptor().Fields()

        // Here we extract each of the fields we care about from the message.
        // Again, we could use fields.Get if we know the indices.
        fmt.Println("station:", station.Get(fields.ByName("station")))
        fmt.Println("frequency:", station.Get(fields.ByName("frequency")))
        fmt.Println("temperature:", station.Get(fields.ByName("temperature")))
        fmt.Println("pressure:", station.Get(fields.ByName("pressure")))
        fmt.Println("wind_speed:", station.Get(fields.ByName("wind_speed")))
        fmt.Println("conditions:", station.Get(fields.ByName("conditions")))
    }
}
```

Currently, `hyperpb` only supports manipulating messages through the reflection
API; it shines best when you need write a very generic service that
downloads types off the network and parses messages using those types, which
forces you to use reflection.

Mutation is currently not supported; any operation which would mutate an
already-parsed message will panic. Which methods of `*hyperpb.Message` panic
is included in the documentation.

### Using types from a registry

We can use the `hyperpb.CompileFileDescriptorSet` function to parse a dynamic type and
use it to walk the fields of a message:

```go
func processDynamicMessage(
    schema *descriptorpb.FileDescriptorSet,
    messageName protoreflect.FullName,
    data []byte,
) error {
    msgType, err := hyperpb.CompileFileDescriptorSet(schema, messageName) // Remember to cache this!
    if err != nil {
        return err
    }

    msg := hyperpb.NewMessage(msgType)
    if err := proto.Unmarshal(data, msg); err != nil {
        return err
    }

    // Range will iterate over all of the populated fields in msg. Here we
    // use Range with go1.24 iterator syntax.
    for field, value := range msg.Range {
        // Do something with each populated field.
    }
    return nil
}
```

Since any generic, non-mutating operation will work with `hyperpb` messages,
we can use them as an efficient transcoding medium from the wire format, for
runtime-loaded messages.

```go
func dynamicMessageToJSON(
    schema *descriptorpb.FileDescriptorSet,
    messageName protoreflect.FullName,
    data []byte,
) ([]byte, error) {
    msgType, err := hyperpb.CompileFileDescriptorSet(schema, messageName)
    if err != nil {
        return nil, err
    }

    msg := hyperpb.NewMessage(msgType)
    if err := proto.Unmarshal(data, msg); err != nil {
        return nil, err
    }

    // Dump the message to JSON. This just works!
    return protojson.Marshal(msg)
}
```

`protovalidate` also works directly on reflection, so it works out-of-the-box:

```go
func validateDynamicMessage(
    schema *descriptorpb.FileDescriptorSet,
    messageName protoreflect.FullName,
    data []byte,
) error {
    // Unmarshal like before.
    msgType, err := hyperpb.CompileFileDescriptorSet(schema, messageName)
    if err != nil {
        return err
    }

    msg := hyperpb.NewMessage(msgType)
    if err := proto.Unmarshal(data, msg); err != nil {
        return err
    }

    // Run custom validation. This just works!
    return protovalidate.Validate(msg)
}
```

## Advanced Usage

`hyperpb` is all about parsing as fast as possible, so there are a number of
optimization knobs available. Calling `Message.Unmarshal` directly instead
of `proto.Unmarshal` allows setting custom `UnmarshalOption`s:

```go
func unmarshalWithCustomOptions(
    schema *descriptorpb.FileDescriptorSet,
    messageName protoreflect.FullName,
    data []byte,
) error {
    msgType, err := hyperpb.CompileFileDescriptorSet(schema, messageName)
    if err != nil {
        return err
    }

    msg := hyperpb.NewMessage(msgType)
    return msg.Unmarshal(
        data,
        hyperpb.WithMaxDecodeMisses(16),
        // Additional options...
    )
}
```

The compiler also takes `CompileOptions`, such as for configuring how extensions
are resolved:

```go
msgType, err := hyperpb.CompileFileDescriptor(
    schema,
    messageName,
    hyperpb.WithExtensionsFromTypes(typeRegistry),
    // Additional options...
)
```

### Memory Reuse

`hyperpb` also has a memory-reuse mechanism that side-steps the Go garbage
collector for improved allocation latency. `hyperpb.Shared` is book-keeping
state and resources shared by all messages resulting from the same parse.
After the message goes out of scope, these resources are ordinarily reclaimed
by the garbage collector.

However, a `hyperpb.Shared` can be retained after its associated message goes
away, allowing for re-use. Consider the following example of a request handler:

```go
type requestContext struct {
    shared *hyperpb.Shared
    types map[string]*hyperpb.MessageType
    // Additional context fields...
}

func (c *requestContext) Handle(req Request) {
    msgType := c.types[req.Type]
    msg := c.shared.NewMessage(msgType)
    defer c.shared.Free()

    c.process(msg, req, ...)
}
```

Beware that `msg` must not outlive the call to `Shared.Free`; failure to do so
will result in memory errors that Go cannot protect you from.

### Profile-Guided Optimization (PGO)

`hyperpb` supports online PGO for squeezing extra performance out of the parser
by optimizing the parser with knowledge of what the average message actually
looks like. For example, using PGO, the parser can predict the expected size of
repeated fields and allocate more intelligently.

For example, suppose you have a corpus of messages for a particular type. You
can build an optimized type, using that corpus as the profile, using
`Type.Recompile`:

```go
func compilePGO(
    md protoreflect.MessageDescriptor,
    corpus [][]byte,
) (*hyperpb.MessageType, error) {
    // Compile the type without any profiling information.
    msgType := hyperpb.CompileMessageDescriptor(md)

    // Construct a new profile recorder.
    profile := msgType.NewProfile()

    // Parse all of the specimens in the corpus, making sure to record a profile
    // for all of them.
    s := new(hyperpb.Shared)
    for _, specimen := range corpus {
        if err := s.NewMessage(msgType).Unmarshal(
            specimen,
            hyperpb.WithRecordProfile(profile, 1.0),
        ); err != nil {
            return nil, err
        }
        s.Free()
    }

    // Recompile with the profile.
    return msgType.Recompile(profile), nil
}
```

Using a custom sampling rate in `hyperpb.WithRecordProfile`, it's possible to
sample data on-line as part of a request flow, and recompile dynamically:

```go
type requestContext struct {
    shared *hyperpb.Shared
    types map[string]*typeInfo
    // Additional context fields...
}

type typeInfo struct {
    msgType atomic.Pointer[hyperpb.MessageType]
    prof atomic.Pointer[hyperpb.Profile]
    seen atomic.Int64
}

func (c *requestContext) Handle(req Request) {
    // Look up the type in the context's type map.
    typeInfo := c.types[req.Type]

    // Parse the type as usual.
    msg := c.shared.NewMessage(typeInfo.msgType.Load())
    defer c.shared.Free()

    if err := msg.Unmarshal(
        data,
        // Only profile 1% of messages.
        hyperpb.WithRecordProfile(typeInfo.prof.Load(), 0.01),
    ); err != nil {
        // Process error...
    }
    typeInfo.seen.Add(1)

    // Every 100,000 messages, spawn a goroutine to asynchronously recompile the type.
    if typeInfo.seen.Load() % 100000 == 0 {
        go func() {
            prof := typeInfo.prof.Load()
            if !typeInfo.prof.CompareAndSwap(prof, nil) {
                // Avoid a race condition.
                return
            }

            // Recompile the type. This is gonna be really slow, because
            // the compiler is slow, which is why we're doing it asynchronously.
            typeInfo.msgType.Store(typeInfo.msgType.Load().Recompile(typeInfo.prof.Load()))
            typeInfo.prof.Store(typeInfo.msgType.Load().NewProfile())
        }
    }

    // Do something with msg.
}
```

## Compatibility

`hyperpb` is experimental software, and the API may change drastically before
`v1`. It currently implements all Protobuf language constructs. It does not
implement mutation of parsed messages, however.

## Contributing

For a detailed explanation of the implementation details of `hyperpb`, see
the [`DESIGN.md`](DESIGN.md) file. Contributions that significantly change the
parser will require benchmarks; you can run them with `make bench`.

## Legal

Offered under the [Apache 2 license][license].

[dynamicpb]: https://pkg.go.dev/google.golang.org/protobuf/types/dynamicpb
[upb]: https://github.com/protocolbuffers/protobuf/tree/main/upb
[license]: https://github.com/bufbuild/hyperpb-go/blob/main/LICENSE