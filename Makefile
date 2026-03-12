# Copyright 2025 Buf Technologies, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# See https://tech.davis-hansson.com/p/make/
SHELL := bash
.DELETE_ON_ERROR:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory

BIN := .tmp/bin
TESTS := .tmp/tests
export PATH := $(abspath $(BIN)):$(PATH)
export GOBIN := $(abspath $(BIN))

COPYRIGHT_YEARS := 2025
LICENSE_IGNORE := testdata/

GOOS_HOST := $(shell go env GOOS)
GOARCH_HOST := $(shell go env GOARCH)

GOOS ?=
GOARCH ?=
GOAMD64 ?=
GOARM64 ?=

GOTOOLCHAIN ?= local
GOEXPERIMENT ?= simd

HOST_ENV ?= GOTOOLCHAIN=$(GOTOOLCHAIN) GOEXPERIMENT=$(GOEXPERIMENT)
EXEC_ENV ?= GOOS=$(GOOS) GOARCH=$(GOARCH) GOAMD64=$(GOAMD64) GOARM64=$(GOARM64) GOTOOLCHAIN=$(GOTOOLCHAIN) GOEXPERIMENT=$(GOEXPERIMENT)

# Go will carelessly pick these up on host-side builds if we don't unexport them.
unexport GOOS
unexport GOARCH

HYPERTESTFLAGS ?=
TESTFLAGS ?=
BENCHFLAGS ?= -test.benchmem

GO ?= go
HOST_TARGET ?=
GO_HOST := $(HOST_TARGET) $(GO)
GO := $(EXEC_ENV) $(GO)

TOOLS := ./internal/tools/external/go.mod
TEST := $(GO_HOST) tool hypertest -o $(TESTS) $(HYPERTESTFLAGS)
DUMP := $(GO_HOST) tool hyperdump
LINT := $(GO_HOST) tool -modfile $(TOOLS) golangci-lint
BUF := $(GO_HOST) tool -modfile $(TOOLS)

TAGS ?= ""
REMOTE ?= ""

ASM_FILTER ?= ^buf.build/go/hyperpb
ASM_INFO ?= fileline

BENCHMARK ?= .

PKG ?=
ifeq ($(PKG),)
	PKGS := ./...
else
	PKGS := $(PKG)
endif
PKG ?= .

.PHONY: help
help: ## Describe useful make targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "%-30s %s\n", $$1, $$2}'

.PHONY: all
all: ## Build, test, and lint (default)
	$(MAKE) test
	$(MAKE) lint

.PHONY: clean
clean: ## Delete intermediate build artifacts
	@# -X only removes untracked files, -d recurses into directories, -f actually removes files/dirs
	git clean -Xdf

.PHONY: test
test: build ## Run unit tests
	$(TEST) -remote=$(REMOTE) -tags=$(TAGS) -checkptr -p $(PKGS) -- \
		$(TESTFLAGS)

.PHONY: bench
bench: build $(BIN)/hypertest ## Run benchmarks
	$(TEST) -remote=$(REMOTE) -tags=$(TAGS) -p $(PKGS) \
		-csv hyperpb.csv -table - -- \
		-test.bench '$(BENCHMARK)' $(BENCHFLAGS)

.PHONY: profile
profile: build $(BIN)/hypertest ## Profile benchmarks and open them in pprof
	$(TEST) -remote=$(REMOTE) -tags=$(TAGS) -p $(PKG) -profile -- \
		-test.run '^B' -test.bench '$(BENCHMARK)' \
		-test.benchtime 5s $(BENCHFLAGS)
	@$(GO_HOST) tool pprof -http localhost:8000 $(TESTS)/*.test $(TESTS)/*.prof

.PHONY: asm
asm: build ## Generate assembly output for manual inspection
	$(GO) test -tags=$(TAGS) -c -o hyperpb.test $(PKG) $(TESTFLAGS)
	$(DUMP) \ 
		-s '$(ASM_FILTER)' \
		-info $(ASM_INFO) \
		-prefix 'buf.build/go/hyperpb' \
		-nops \
		-o hyperpb.s \
		hyperpb.test

.PHONY: build
build: generate ## Build all packages
	$(GO) build -tags=$(TAGS) $(PKGS)

.PHONY: show-env
show-env: ## Print the Go tool's interpreted environment.
	go version
	$(GO) env

.PHONY: lint
lint: generate ## Lint
	$(GO_HOST) vet -unsafeptr=false ./...
	$(LINT) -v run \
		--timeout 3m0s \
		--modules-download-mode=readonly

.PHONY: lintfix
lintfix: generate ## Automatically fix some lint errors
	$(LINT) run \
		--timeout 3m0s \
		--modules-download-mode=readonly \
		--fix

.PHONY: generate
generate: internal/gen/*/*.pb.go ## Regenerate code and licenses
	$(GO_HOST) generate ./...
	$(GO_HOST) tool -modfile $(TOOLS) license-header \
		--license-type apache \
		--copyright-holder "Buf Technologies, Inc." \
		--year-range "$(COPYRIGHT_YEARS)" \
		--ignore $(LICENSE_IGNORE)

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	go get toolchain@none
	go get -u -t ./...
	go mod tidy -v

.PHONY: checkgenerate
checkgenerate:
	@# Used in CI to verify that `make generate` doesn't produce a diff.
	git --no-pager diff --exit-code >&2

internal/gen/*/*.pb.go: internal/proto/*/*/*.proto internal/proto/*/*/*/*.proto
	$(BUF) generate --clean
	$(BUF) generate --template buf.vt.gen.yaml
