PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# 'go install' into the project's bin directory
# and add it to the PATH.
export GOBIN ?= $(PROJECT_ROOT)/bin
export PATH := $(GOBIN):$(PATH)

# only use -race if NO_RACE is unset.
RACE=$(if $(NO_RACE),,-race)

GOLANGCI_LINT_ARGS ?=

.PHONY: test
test:
	go test $(GO_TEST_FLAGS) $(RACE) -v ./...
	go test $(GO_TEST_FLAGS) $(RACE) -tags safe -v ./...
	go test $(GO_TEST_FLAGS) -gcflags='-l -N' ./... # disable optimizations/inlining

COVERDIR ?= $(shell mktemp -d)
.PHONY: cover
cover: export GOEXPERIMENT = coverageredesign
cover:
	mkdir -p $(COVERDIR)
	make test GO_TEST_FLAGS="-test.gocoverdir=$(COVERDIR) -coverpkg=./... -covermode=atomic"
	go tool covdata textfmt -i=$(COVERDIR) -o=cover.out
	go tool cover -html=cover.out -o=cover.html

# NOTE:
# The cover target uses the undocumented test.gocoverdir flag
# to generate three coverage reports and merge them with the coverage redesign.
#
# Ref: https://github.com/golang/go/issues/51430#issuecomment-1344711300
#
# The alternative is to generate three coverage reports
# and figure out how to merge them ourselves.

.PHONY: bench
bench:
	go test -run NONE -bench . -cpu 1

.PHONY: bench-parallel
bench-parallel:
	go test -run NONE -bench . -cpu 1,2,4,8

README.md: $(wildcard doc/*.md)
	@if ! command -v stitchmd >/dev/null; then \
		echo "stitchmd not found. Installing..."; \
		go.abhg.dev/stitchmd@latest; \
	fi; \
	echo "Generating $@"; \
	stitchmd -o $@ doc/SUMMARY.md

.PHONY: lint
lint: golangci-lint

.PHONY: golangci-lint
golangci-lint:
	@if ! command -v golangci-lint >/dev/null; then \
		echo "golangci-lint not found. Installing..."; \
		mkdir -p $(GOBIN); \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN); \
	fi; \
	echo "Running golangci-lint"; \
	golangci-lint run $(GOLANGCI_LINT_ARGS) ./...
