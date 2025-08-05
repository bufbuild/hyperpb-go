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

package generator

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/gofeaturespb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	genGoDocURL       = "https://protobuf.dev/reference/go/go-generated"
	SupportedFeatures = uint64(
		pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL |
			pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)

	MinEdition = descriptorpb.Edition_EDITION_PROTO2
	MaxEdition = descriptorpb.Edition_EDITION_2023
)

// Main is the main function of the generator. This is called by main() in
// the actual main package.
func Main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stdout, "%v %v\n", filepath.Base(os.Args[0]), version())
		return
	}

	if len(os.Args) == 2 && os.Args[1] == "--help" {
		fmt.Fprintf(os.Stdout, "See "+genGoDocURL+" for usage information.\n")
		return
	}

	var flags flag.FlagSet
	protogen.Options{
		ParamFunc:       flags.Set,
		DefaultAPILevel: gofeaturespb.GoFeatures_API_OPAQUE,
	}.Run(func(pl *protogen.Plugin) error {
		g := new(Generator)
		for _, f := range pl.Files {
			if f.Generate {
				g.Generate(pl, "hyper-"+string(f.GoPackageName), f)
			}
		}

		pl.SupportedFeatures = SupportedFeatures
		pl.SupportedEditionsMinimum = MinEdition
		pl.SupportedEditionsMaximum = MaxEdition
		return nil
	})
}

func version() string {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return "<unknown>"
	}
	return build.Main.Version
}
