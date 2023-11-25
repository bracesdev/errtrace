PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# 'go install' into the project's bin directory
# and add it to the PATH.
export GOBIN ?= $(PROJECT_ROOT)/bin
export PATH := $(GOBIN):$(PATH)

# only use -race if NO_RACE is unset.
RACE=$(if $(NO_RACE),,-race)

.PHONY: test
test:
	go test $(RACE) -v ./...
	go test $(RACE) -tags safe -v ./...
	go test -gcflags='-l -N' ./... # disable optimizations/inlining

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
