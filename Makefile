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
	go test $(RACE) -v ./...
	go test $(RACE) -tags safe -v ./...
	go test -gcflags='-l -N' ./... # disable optimizations/inlining

.PHONY: cover
cover:
	go test -coverprofile cover.unsafe.out -coverpkg ./... $(RACE) -v ./...
	go test -coverprofile cover.safe.out -coverpkg ./... $(RACE) -tags safe -v ./...
	go test ./... -gcflags='-l -N' ./... # disable optimizations/inlining

.PHONY: bench
bench:
	go test -run NONE -bench . -cpu 1

.PHONY: bench-parallel
bench-parallel:
	go test -run NONE -bench . -cpu 1,2,4,8

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
