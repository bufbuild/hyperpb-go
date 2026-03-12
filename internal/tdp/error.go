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

package tdp

import (
	"errors"
	"fmt"
	"io"
)

// These match the errors in protowire.
const (
	ErrorOk ErrorCode = iota

	ErrorTruncated
	ErrorFieldNumber
	ErrorOverflow
	ErrorReserved
	ErrorEndGroup
	ErrorRecursionDepth

	ErrorUTF8
	ErrorTooBig
)

var errs = [...]error{
	ErrorOk:             nil,
	ErrorTruncated:      io.ErrUnexpectedEOF,
	ErrorFieldNumber:    errors.New("invalid field number"),
	ErrorOverflow:       errors.New("variable length integer overflow"),
	ErrorReserved:       errors.New("cannot parse reserved wire type"),
	ErrorEndGroup:       errors.New("mismatching end group marker"),
	ErrorRecursionDepth: errors.New("recursion depth exceeded"),
	ErrorUTF8:           errors.New("invalid UTF-8 in string"),
	ErrorTooBig:         errors.New("input was larger than 4GB"),
}

// ErrorCode is one of the possible types of errors in [Error].
type ErrorCode int

// ErrorAt returns a new error with this code.
func (c ErrorCode) ErrorAt(offset int) Error {
	return Error{c, offset}
}

// GetCode extracts the error code out of an [Error].
func GetCode(e Error) ErrorCode {
	return e.code
}

// Error is an error returned by the TDP parser.
//
// Do not add new methods to this type, since those become public API.
type Error struct {
	code   ErrorCode
	offset int
}

// Offset returns the offset at which the error occurred.
func (e *Error) Offset() int {
	return e.offset
}

// Unwrap implements error unwrapping viz [errors.Unwrap].
func (e *Error) Unwrap() error {
	return errs[e.code]
}

// Error implements [error].
func (e *Error) Error() string {
	return fmt.Sprintf("hyperpb: parser error at offset %d/%#x: %v", e.offset, e.offset, e.Unwrap())
}
