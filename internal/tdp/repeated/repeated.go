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

// Package repeated contains shared layouts for repeated field implementations,
// for sharing between the tdp packages and the gencode packages.
package repeated

// DoNotImplement is used for tagging types in this package in such a way
// that users cannot implement hyperpb.List themselves.
type DoNotImplement struct{ _ struct{} }
