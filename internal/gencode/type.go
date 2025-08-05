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

package gencode

import (
	"fmt"
	"sync/atomic"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// This map is only written to during initialization, which all runs on the
// same goroutine, so it does not need lock protection.
var types = make(map[protoreflect.MessageDescriptor]*Type)

// Type is a pointer to a compiled type that can be updated atomically by the
// gencode runtime.
//
// Gencode contains globals for each type relevant to it, which are registered
// with the gencode runtime at startup.
//
// We depend on the ordinary Protobuf Go gencode to provide us with compiled-in
// descriptors.
type Type struct {
	desc  protoreflect.MessageDescriptor
	value atomic.Pointer[hyperpb.MessageType]
}

// Init initializes a Type.
//
// This is meant to be called only once the desired type has landed in
// GlobalTypes, so its package must import the "ordinary" package to ensure
// correct ordering.
func (t *Type) Init(name protoreflect.FullName) {
	ty, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		panic(fmt.Errorf("could not find descriptor for %s; this is a gencode bug", name))
	}
	t.desc = ty.Descriptor()
	types[t.desc] = t
}

// Get returns the associated hyperpb.Type, or compiles it if necessary.
func (t *Type) Get() *hyperpb.MessageType {
	p := t.value.Load()
	if p != nil {
		return p
	}

	// Slow case. Need to compile a type. When we do, we use this as an
	// opportunity to fill in types for all of its dependencies, too.
	ty := hyperpb.CompileMessageDescriptor(
		t.desc,
		hyperpb.WithExtensionsFromTypes(protoregistry.GlobalTypes),
	)

	t.store(ty)
	return ty
}

// store records ty as the type for this value, and recurses into its dependent
// types.
func (t *Type) store(ty *hyperpb.MessageType) {
	if t == nil || !t.value.CompareAndSwap(nil, ty) {
		return
	}

	// Update all of the submessage types if possible, too.
	for t := range xunsafe.Cast[tdp.Type](ty).Submessages() {
		types[t.Descriptor].store(xunsafe.Cast[hyperpb.MessageType](t))
	}
}
