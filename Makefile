GO ?= go
GOLANGCI_LINT ?= golangci-lint

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')
COVERAGE_DIR ?= coverage
COVERAGE_PROFILE ?= $(COVERAGE_DIR)/coverage.out
COVERAGE_TEXT ?= $(COVERAGE_DIR)/coverage.txt
COVERAGE_PACKAGES ?= $(COVERAGE_DIR)/coverage-packages.md
COVERAGE_HTML ?= $(COVERAGE_DIR)/coverage.html

.PHONY: help fmt fmt-check tidy tidy-check vet lint test race coverage bench-cache bench-ratelimit bench-compression bench-id ci

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
		'  coverage    Generate Go coverage profile, package summary, and HTML report' \
		'  bench-cache Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks' \
		'  bench-ratelimit Run opt-in local rate limiter benchmarks' \
		'  bench-id    Run opt-in id generator benchmarks' \
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
	@{ \
		printf '%s\n' '| Package | Coverage | Covered | Statements |'; \
		printf '%s\n' '|---|---:|---:|---:|'; \
		awk 'BEGIN { prefix = "github.com/bluetape4k/bluetape-go/" } NR == 1 && $$1 == "mode:" { next } { file = $$1; sub(/:.*/, "", file); pkg = file; sub(/\/[^\/]*$$/, "", pkg); statements = $$2 + 0; hits = $$3 + 0; key = $$1 " " $$2; if (!(key in seen)) { seen[key] = 1; block_pkg[key] = pkg; total[pkg] += statements } if (hits > 0 && !(key in covered_seen)) { covered_seen[key] = 1; covered[pkg] += statements } } END { for (pkg in total) { display = pkg; sub("^" prefix, "", display); if (display == "") display = "."; pct = total[pkg] == 0 ? 100 : covered[pkg] * 100 / total[pkg]; printf "%s\t| `%s` | %.1f%% | %d | %d |\n", display, display, pct, covered[pkg], total[pkg] } }' $(COVERAGE_PROFILE) | sort | cut -f2-; \
		awk 'NR == 1 && $$1 == "mode:" { next } { statements = $$2 + 0; hits = $$3 + 0; key = $$1 " " $$2; if (!(key in seen)) { seen[key] = 1; total += statements } if (hits > 0 && !(key in covered_seen)) { covered_seen[key] = 1; covered += statements } } END { pct = total == 0 ? 100 : covered * 100 / total; printf "| **Total** | **%.1f%%** | **%d** | **%d** |\n", pct, covered, total }' $(COVERAGE_PROFILE); \
	} > $(COVERAGE_PACKAGES)
	@$(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)

bench-cache:
	@$(GO) test -run '^$$' -bench '^BenchmarkMemory' -benchmem ./cache
	@$(GO) test -run '^$$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
	@$(GO) test -run '^$$' -bench '^BenchmarkStampedeCache' -benchmem ./cache/rediscoord

bench-ratelimit:
	@$(GO) test -run '^$$' -bench '^Benchmark' -benchmem ./ratelimit

bench-compression:
	@$(GO) test -run '^$$' -bench '^BenchmarkCompressors' -benchmem ./compression

bench-id:
	@$(GO) test -run '^$$' -bench '^Benchmark(UUIDV4|UUIDV7|ULIDRandom|ULIDMonotonicParallel|KSUIDNextString|KSUIDMillisNextString|SnowflakeNextInt64|SnowflakeNextInt64SameMillisecond|SnowflakeNextInt64Parallel)$$' -benchmem ./id

ci: tidy-check fmt-check vet lint test race
