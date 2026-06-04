GO ?= go
GOLANGCI_LINT ?= golangci-lint

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')
COVERAGE_DIR ?= coverage
COVERAGE_PROFILE ?= $(COVERAGE_DIR)/coverage.out
COVERAGE_TEXT ?= $(COVERAGE_DIR)/coverage.txt
COVERAGE_HTML ?= $(COVERAGE_DIR)/coverage.html

.PHONY: help fmt fmt-check tidy tidy-check vet lint test race coverage bench-cache bench-compression ci

help:
	@printf '%s\n' \
		'Targets:' \
		'  fmt         Format Go sources with gofmt' \
		'  fmt-check   Fail when Go sources are not gofmt-formatted' \
		'  tidy        Run go mod tidy' \
		'  tidy-check  Run go mod tidy and fail on go.mod/go.sum drift' \
		'  vet         Run go vet ./...' \
		'  lint        Run golangci-lint' \
		'  test        Run uncached go test ./... including Testcontainers tests' \
		'  race        Run uncached go test -race ./... including Testcontainers tests' \
		'  coverage    Generate Go coverage profile, summary, and HTML report' \
		'  bench-cache Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks' \
		'  ci          Run the local CI gate'

fmt:
	@gofmt -w $(GO_FILES)

fmt-check:
	@test -z "$$(gofmt -l $(GO_FILES))"

tidy:
	@$(GO) mod tidy

tidy-check:
	@$(GO) mod tidy
	@git diff --exit-code -- go.mod go.sum

vet:
	@$(GO) vet ./...

lint:
	@$(GOLANGCI_LINT) run ./...

test:
	@$(GO) test -count=1 ./...

race:
	@$(GO) test -race -count=1 ./...

coverage:
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE_PROFILE) ./...
	@$(GO) tool cover -func=$(COVERAGE_PROFILE) | tee $(COVERAGE_TEXT)
	@$(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)

bench-cache:
	@$(GO) test -run '^$$' -bench '^BenchmarkMemory' -benchmem ./cache
	@$(GO) test -run '^$$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
	@$(GO) test -run '^$$' -bench '^BenchmarkStampedeCache' -benchmem ./cache/rediscoord

bench-compression:
	@$(GO) test -run '^$$' -bench '^BenchmarkCompressors' -benchmem ./compression

ci: tidy-check fmt-check vet lint test race
