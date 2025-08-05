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

import "google.golang.org/protobuf/compiler/protogen"

const (
	hyperpbPath         = "buf.build/go/hyperpb"
	hyperpbExportPath   = "buf.build/go/hyperpb/cmd/protoc-gen-hyperpb/export"
	protoreflectPkgPath = "google.golang.org/protobuf/reflect/protoreflect"
)

var (
	hyperpbMessage = protogen.GoIdent{
		GoImportPath: hyperpbPath,
		GoName:       "Message",
	}
	hyperpbNew = protogen.GoIdent{
		GoImportPath: hyperpbPath,
		GoName:       "NewMessage",
	}
	hyperpbList = protogen.GoIdent{
		GoImportPath: hyperpbPath,
		GoName:       "List",
	}
	hyperpbShared = protogen.GoIdent{
		GoImportPath: hyperpbPath,
		GoName:       "Shared",
	}
	hyperpbOption = protogen.GoIdent{
		GoImportPath: hyperpbPath,
		GoName:       "UnmarshalOption",
	}

	exportType = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "Type",
	}
	exportReflect = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "Reflect",
	}
	exportDNI = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "DoNotImplement",
	}
	exportMessage = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "UnsafeMessage",
	}
	exportCastEnum = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "CastRepeatedEnum",
	}
	exportCastMessage = protogen.GoIdent{
		GoImportPath: hyperpbExportPath,
		GoName:       "CastRepeatedMessage",
	}

	reflectMessage = protogen.GoIdent{
		GoImportPath: protoreflectPkgPath,
		GoName:       "Message",
	}
	reflectType = protogen.GoIdent{
		GoImportPath: protoreflectPkgPath,
		GoName:       "MessageType",
	}

	unsafePointer = protogen.GoIdent{GoImportPath: "unsafe", GoName: "Pointer"}

	mathFloat32 = protogen.GoIdent{GoImportPath: "math", GoName: "Float32frombits"}
	mathFloat64 = protogen.GoIdent{GoImportPath: "math", GoName: "Float64frombits"}
)
