# Hacking Tips

The implementation of the message and parser follows the design principles of
UPB, an even faster C runtime. All memory is allocated on arenas, and where
possible fields are zero-copy references of the original parse input.

Schematically, this package can be thought of as having two major components.

  1. A compiler that generates `Type`s at runtime. `Type`s can be thought of
     as highly optimized programs for parsing Protobuf messages. The compiler
     is not optimized for speed, because it is expected to be a one-time call
     per message type.

  2. An highly optimized interpreter for the programs that the compiler
     generates. No expense is spared in making this interpreter as fast as
     possible, beyond limitations incurred by the Go compiler's mediocre
     register allocation and other codegen bugs.

## Benchmarking and Profiling

To ensure that your changes don't make performance worse, you should use the
provided benchmarks. `make bench` will run all benchmarks; `make profile`
will run a CPU profile and display it in a local `pprof` instance.
`make asm` will dump the assembly of the benchmarks for manual inspection.

The benchmarks are also run against `protobuf-go`'s generated code, and
`dynamicpb`, which this package replaces.Benchmarks are defined by `.yaml`
files in the `internal/testdata` directory.

When filing a PR, it is recommended that it include a before/after comparison
of the benchmarks.

## File Structure

The main package, `hyperpb`, is mostly a facade over a collection of internal
packages. The bulk of the complicated stuff lives in `internal/tdp`:

* `tdp` itself contains the "tables", or "executable format", of the
  parser VM.
* `tdp/dynamic` contains the rudiments for dynamic message types based
  on the layout information from `tdp`. See also `tdp/empty`.
* `tdp/vm` is the parser VM itself.
* `tdp/compiler` is the compiler that generates the tables from `tdp`.
* `tdp/thunks` is the specialized code for parsing each of the hundreds
  of field type combinations.

The other important internal packages are:

* `arena` - All allocations go through here. See
  <https://mcyoung.xyz/2025/04/21/go-arenas/> for an introduction.
* `debug` - Debugging utilities. Enable with `-tags debug`.
* `swiss` - Full-fledged Swisstable implementation.
* `tools/stencil` - Code generator for manually specializing generic functions.
* `xunsafe` - Unsafe helpers.

# Implementation Details

## A Brief Primer on Go's Memory Semantics

Go does not specify the precise semantics of its garbage collector; however,
there are some GC behaviors that would be very difficult for Go to change
without breaking existing code. We assume that these behaviors will not
change to enable side-stepping the GC in several situations where it would
result in non-optimal code generation.

  1. The GC will never move data that has escaped to the heap. That is, it
     will not look for and update pointers whose address refers to a value
     that has escaped to the heap.
     Doing so requires both stopping the world and searching the whole heap
     for addresses within any ranges being moved, something that is many
     times more expensive than Go's current marking phase, and which cannot
     be done concurrently. Non-native GC'ed languages avoid this cost by,
     among other things, avoiding having to mark interior pointers.

  2. It is possible to reliably force data to escape to the heap. This can
     be done, for example, by writing to a global `unsafe.Pointer`.
     `runtime.KeepAlive` is the prototypal example.

  3. If the GC can find a pointer to anywhere within an object, even an
     offset that would be otherwise misaligned (such as pointing midway into
     an interface or an integer), the GC will mark the entire struct, not
     just a pointed-to subobject.
     This is because Go pointers are untyped from the perspective of the
     runtime, an assumption that is baked into the GC and the unsafe package.
     Loads and stores are typed, but pointers are not (similar to LLVM).

  4. Go tolerates unaligned loads and stores if the underlying machine does.
     This is only true for loads and stores of pointer-free data. If the
     load is a pointer load, it may emit a write barrier, which may trigger
     an alignment check. Because pointers are untyped, Go tolerates unaligned
     pointers merely existing (like Rust, but unlike C++).

  5. It is possible to allocate memory of arbitrary GC shape at runtime. This
     can be achieved using reflection to create a fresh struct type and then
     allocate a value thereof.

  6. Storing pointer data without a write barrier (by storing a `uintptr`
     instead) is fine as long as the location being stored to does not
     actually have pointer shape, so the GC would not be potentially
     concurrently tracing through it.

The most important corollary of this are that it is possible to store
pointers in heap memory that is not of GC shape, then load those addresses
and load/store through them, so long as:

  1. That pointer is forced to escape to the heap in one way or another.

  2. That pointer is transitively reachable by the GC from the allocation
     that contains the heap memory the pointer is stored to.

This enables many data structure optimizations in this package, and in the
`arena.Arena` type it depends on. In particular, so long as all pointers in
an arena were allocated by the arena itself, the GC will never invalidate
them, because the arena is carefully written such that holding a pointer
into any arena memory will cause the whole arena to be traced by the GC

This blogpost contains some of the same information:
<https://mcyoung.xyz/2025/04/21/go-arenas/>.

## The Compiler

The compiler consumes descriptors and generates a program. This program
specifies:

  1. How many bytes to allocate for a particular message type.

  2. What fields must be parsed for that message type.

  3. What message types those fields have, so that the parser can recurse
     into them.

The compiler is responsible for laying out the in-memory representation of
each message. A message consists of a fixed header (the fields of 
`dynamic.Message`), an array of "bitfield words", which are used to store
has-bits of optional fields and the values of `bool` fields, and the storage for
all of the field values. For example, an `optional int32` compiles to one bit
plus four bytes of storage.

The offsets for the storage of a given field, plus a thunk for actually
extracting the field value as a `protobuf.Value`, are stored in the field's
`Getter`. There are dozens of storage strategies for a field; rather than
storing the field's type information, we record the address of a thunk that
implicitly contains this information. This means that getting a field costs
an indirect branch, instead of whatever mess Go would choose to codegen for
a massive switch.

The compiler also selects the parser thunks for the field, of which there may
be up to two. These are called into when the parser decides to actually
populate a field; again, these are thunks because a single indirect branch is
faster than a massive switch. Each thunk is keyed by its Protobuf record tag,
which is stored in an unusual form described in the parser. The compiler
is responsible for assembling this key.

This compiler's equivalent of instruction selection (which we call archetype
selection) is the mechanism by which a `FieldDescriptor` is associated with one
of dozens of `compiler.Archetype`s, which represent the various ways in which a
field can be parsed and accessed. For example, for each `protoreflect.Kind`,
there is an archetype for that field appearing as implicit presence (called
"singular" here), explicit presence (i.e. "optional") or repeated.

The addition of extra archetypes is not a performance penalty if they are
not used in a particular parser, which allows for arbitrary levels of
specialization for a given field. For example, messages are always stored as
pointers, but particularly small messages (such as messages with one or two
fields) could be directly inlined into the message.

## Type Representation

All of the types that a particular compiled `tdp.Type` depends on live on a
giant byte buffer, called a `tdp.Library`. Each `*tdp.Type` is a pointer to this
buffer. This buffer also contains a pointer to the descriptor the `tdp.Type` was
compiled from, to ensure that the GC does not sweep the descriptors while the
`tdp.Type` is still alive.

Each `tdp.Type` is followed by at least one `tdp.Field`. The fields are laid out
in field index order. This is useful for efficiently implementing reflection.

It also contains a `tdp.TypeParser`, which contains `tdp.FieldParsers`.
Each includes an pointer to a `tdp.Type` for its message type, if it has one,
as well as offset and parsing information.

Note that the compiled `tdp.Type` does NOT contain a `reflect.Type` for the
struct that the compiler laid out for it. Go reflection is not used for
manipulating dynamic message types; instead, we rely on raw loads and stores,
and the value allocated from a `tdp.Type` does not have a GC shape that contains
pointers.

This allows us to never perform stores of types which contain pointers, even
though the dynamic message types do contain pointers (into the arena in which
they are allocated), and thus avoids the need for write barriers, which incur
touching a highly-contended atomic global, triggering a guaranteed cache
miss. This in turn is fine, because the pointers we manipulate are arena
pointers that are marked for us via other means.

Finally, each `tdp.TypeParser` contains a hand-written hashmap that maps field
numbers to field indices, for quickly re-synchronizing the parser in the event
of encountering fields out of the expected order. Currently, all types have a
number table, but many message types have number/index relationships that are
1:1, in which case number == index for all fields; as a future optimization,
we should remove the table in this case.

## Parser Design

The parser is an VM for the programs encoded in `tdp.TypeParser`s. It is a
threaded interpreter, which means that all of its state is threaded by-value
through function calls, to minimize the need for spilling state to the stack
across calls (regardless, Go's mediocre register allocator still
unnecessarily spills a lot of state).

The parser state is split across three types, `vm.P1`, `vm.P2`, and
`vm.p3`, to work around various bugs in Go's codegen that would result in
worse performance. These structs should be thought of as being the same
entity.

The main loop of the interpreter is `vm.loop`, which is not actually
a loop but rather a strongly connected component of blocks joined by `goto`s.
This is because the main loop has three different sections which we want
to be able to jump to from any other section. These are:

  1. Partially decoding a field tag.

  2. Matching the partially decoded tag against the parsers for the current
     field.

  3. Resynchronizing the field pointer in the event of failing to make
     progress in (2) enough times.

The core parser loop isn't even recursive; we manage our own message recursion
stack instead.

We will now dive into each of these components separately.

## Partial Tag Decode

It turns out that we don't need to fully decode a tag in most cases. Each
parser thunk within a field knows what the field needs to look like when
encoded on the wire, so we could just compare bytes without shifting or
variable-length masking.

However, we also need to accept overlong tags, up to 10 bytes. To do this,
we first load eight bytes and use bit tricks to count the number of sign bits
(treating the uint64 as a janky uint8x8 SIMD vector). If this is large
enough, we hit a slow path to check the remaining two bytes.

Then, we use the varint byte length to mask off all bytes after the highest
order one which is relevant; in the case of 8, 9, and 10 byte varints, we
don't mask off any bytes.

Finally, we clear the sign bits, which are the only discrimination between
the various potential encodings for the varint. All of the bytes beyond the
non-zero bytes for this tag must be zero for the match to succeed; if the
tag was over-long, those bytes are left untouched by the prior masking step.

This step also advances the parser past the varint, so there's no need to
store the length of the varint anymore past this point.

This step only needs to be performed once per record, and the resulting
partially decoded value is cached to compare against the `tdp.Tag` values
in the next phase. The interpreter only jumps to this block when it is ready
to start parsing a new record.

## Tag Matching and Fallback

Each field parser contains the tag (fused field number + wire type) it
expects for that field, so it's sufficient to perform an equality check. This
avoids the need to actually decode the varint by slightly shifting each byte,
and then shifting out the wire type.

If a field matches, it calls into the parser thunk, which consumes the field
value and may chose to advance the field table pointer in the parser state.
Some thunks choose not to do this for optimization reasons: when encountering
a non-packed repeated field, the thunk predicts the next record will be for
the same field. For packed repeated fields, however, it is very likely that
the entire field value is contained in one record, so the parser can advance
to the next field. (This can be thought of as a sort of statically-computed
branch prediction.)

Then, the parser jumps back to the partial tag decode code.

If a field does not match, the field table pointer is advanced and we try
again a few more times. If we keep failing, we instead fall into the
fallback code, where we fully decode the tag and search the number table,
and potentially skip some unknown fields.

## Parser Thunks

As mentioned previously, there are many, many parser thunks. This is because
if unused, those thunks will not appear in the BHT (branch history table)
entry for the indirect branch to the thunk in `vm.P1.Field()`. This means
that it is beneficial to hyper-specialize thunks so that within a thunk,
additional branches are kept to a minimum. Thus, instead of having one thunk
that handles both optional and singular fields, and branches internally based
on some value, we have one thunk for each, which amortizes the cost of that
branch into the thunk call's branch instruction.

Similarly, there are distinct parsers for varint, zigzag, and fixed integers.

That said, if parsers can be reused across archetypes with zero overhead, it
is beneficial to do so as a matter of instruction cache friendliness. For
example, int32, uint32, and enum fields of the same presence discipline will
use the same thunk, because they're all parsed and stored as 32-bit varints.
The same is true of fixed32, sfixed32, and float32.

To learn more about a particular archetype's implementation choices, look at
the `field_*.go` files.

## Parser Threading And Inlining

Many functions in the parser have been manually inlined, for two reasons:

  1. Go's inline heuristic is not good. It will not inline code in many cases
     where the win is obvious (such as functions that are only called in one
     place).

  2. We want to avoid calling functions that do not thread parser1 and
     parser2 in a consistent way, because this results in spills across
     calls.

We don't make very much use of `protowire`'s primitives, because they don't
inline well and perform bounds checks that we statically know to be
redundant, but which Go does not perform sufficient inlining to realize.
For example, the core varint decoding code for field values (not tags) is
an almost unchanged inlined copy of `protowire.ConsumeVarint`, because this
is a very hot function and doing so improves performance significantly,
particularly because we can remove many of the branches.
