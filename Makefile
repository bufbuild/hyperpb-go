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

GO_VERSION := go1.25.0
BUF_VERSION := v1.56.0 # Keep in sync w/ .github/workflows/buf.yaml.
LINT_VERSION := v2.4.0 # Keep in sync w/ .github/workflows/ci.yaml.

GOOS_HOST := $(shell go env GOOS)
GOARCH_HOST := $(shell go env GOARCH)

GOOS ?=
GOARCH ?=
GOAMD64 ?=
GOARM64 ?=

HOST_ENV ?= GOTOOLCHAIN=local
EXEC_ENV ?= GOOS=$(GOOS) GOARCH=$(GOARCH) GOAMD64=$(GOAMD64) GOARM64=$(GOARM64) GOTOOLCHAIN=local

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
TEST := $(EXEC_ENV) $(BIN)/hypertest -o $(TESTS) $(HYPERTESTFLAGS)

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
test: build $(BIN)/hypertest ## Run unit tests
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
	$(GO_HOST) run ./internal/tools/hyperdump \
		-s '$(ASM_FILTER)' \
		-info $(ASM_INFO) \
		-prefix 'buf.build/go/hyperpb' \
		-nops \
		-o hyperpb.s \
		hyperpb.test

.PHONY: build
build: generate ## Build all packages
	$(GO) build -tags=$(TAGS) $(PKGS)

.PHONY: lint
lint: $(BIN)/golangci-lint ## Lint
	$(GO_HOST) vet -unsafeptr=false ./...
	$(BIN)/golangci-lint -v run \
		--timeout 3m0s \
		--modules-download-mode=readonly

.PHONY: lintfix
lintfix: $(BIN)/golangci-lint ## Automatically fix some lint errors
	$(BIN)/golangci-lint run \
		--timeout 3m0s \
		--modules-download-mode=readonly \
		--fix

.PHONY: generate
generate: internal/gen/*/*.pb.go $(BIN)/license-header ## Regenerate code and licenses
	$(GO_HOST) generate ./...
	$(BIN)/license-header \
		--license-type apache \
		--copyright-holder "Buf Technologies, Inc." \
		--year-range "$(COPYRIGHT_YEARS)" \
		--ignore $(LICENSE_IGNORE)

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	go mod edit -toolchain=$(GO_VERSION)
	go get -u -t ./...
	go mod tidy -v

.PHONY: checkgenerate
checkgenerate:
	@# Used in CI to verify that `make generate` doesn't produce a diff.
	git --no-pager diff --exit-code >&2

internal/gen/*/*.pb.go: $(BIN)/buf internal/proto/*/*/*.proto internal/proto/*/*/*/*.proto
	$(BIN)/buf generate --clean
	$(BIN)/buf generate --template buf.vt.gen.yaml

.PHONY: $(BIN)/hypertest
$(BIN)/hypertest: generate
	@mkdir -p $(@D)
	$(GO_HOST) build -o $(BIN)/hypertest ./internal/tools/hypertest

$(BIN)/buf: Makefile
	@mkdir -p $(@D)
	$(GO_HOST) install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

$(BIN)/license-header: Makefile
	@mkdir -p $(@D)
	$(GO_HOST) install github.com/bufbuild/buf/private/pkg/licenseheader/cmd/license-header@$(BUF_VERSION)

$(BIN)/golangci-lint: Makefile
	@mkdir -p $(@D)
	$(GO_HOST) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
