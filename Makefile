GO ?= go
GOLANGCI_LINT ?= golangci-lint

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')
COVERAGE_DIR ?= coverage
COVERAGE_PROFILE ?= $(COVERAGE_DIR)/coverage.out
COVERAGE_TEXT ?= $(COVERAGE_DIR)/coverage.txt
COVERAGE_PACKAGES ?= $(COVERAGE_DIR)/coverage-packages.md
COVERAGE_HTML ?= $(COVERAGE_DIR)/coverage.html
BENCH_COUNT ?= 5
BENCH_CPU ?= 1,2,4

.PHONY: help fmt fmt-check tidy tidy-check vet lint test race coverage bench-cache bench-ratelimit bench-compression bench-id bench-rules bench-web-gin bench-web-gin-regression check-bench-web-gin ci

help:
	@printf '%s\n' \
		'Targets:' \
		'  fmt         Format Go sources with gofmt' \
		'  fmt-check   Fail when Go sources are not gofmt-formatted' \
		'  tidy        Run go mod tidy' \
		'  tidy-check  Run go mod tidy and fail on go.mod/go.sum drift' \
		'  vet         Run go vet ./...' \
		'  lint        Run golangci-lint' \
		'  test        Run uncached go test ./... with serial package execution for Testcontainers safety' \
		'  race        Run uncached go test -race ./... with serial package execution for Testcontainers safety' \
		'  coverage    Generate Go coverage profile, package summary, and HTML report' \
		'  bench-cache Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks' \
		'  bench-ratelimit Run opt-in local rate limiter benchmarks' \
		'  bench-id    Run opt-in id generator benchmarks' \
		'  bench-rules Run opt-in rules composite and inference benchmarks' \
		'  bench-web-gin Run the opt-in Gin adapter benchmark capture (BENCH_COUNT/BENCH_CPU)' \
		'  bench-web-gin-regression Compare Gin adapter capture with BENCH_BASELINE' \
		'  check-bench-web-gin Validate the Gin adapter benchmark capture contract without running benchmarks' \
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
	@$(GO) test -p 1 -count=1 ./...
	@$(GO) test -vet=off ./batch/testdata/compat

race:
	@$(GO) test -race -p 1 -count=1 ./...

coverage:
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -p 1 -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE_PROFILE) ./...
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
	@$(GO) test -run '^$$' -bench '^Benchmark(UUIDV4NewString|UUIDV4NewStringParallel|UUIDV4ReuseGenerator|UUIDV4ReuseGeneratorParallel|UUIDV7NewString|UUIDV7NewStringParallel|UUIDV7ReuseGenerator|UUIDV7ReuseGeneratorParallel|ULIDRandom|ULIDRandomParallel|ULIDMonotonic|ULIDMonotonicParallel|KSUIDNextString|KSUIDNextStringParallel|KSUIDMillisNextString|KSUIDMillisNextStringParallel|SnowflakeNextInt64|SnowflakeNextInt64SameMillisecond|SnowflakeNextInt64Parallel)$$' -benchmem ./id

bench-rules:
	@$(GO) test -run '^$$' -bench '^BenchmarkRules' -benchmem -count=5 ./rules

bench-web-gin:
	@scripts/capture-gin-adapter-benchmark.sh "$(BENCH_COUNT)" "$(BENCH_CPU)"

BENCH_BASELINE ?= docs/research/outputs/issue-543/bench-baseline.json
BENCH_RESULTS ?= docs/research/outputs/issue-543/bench-results.json
BENCH_REGRESSION_REPORT ?= docs/research/outputs/issue-543/bench-regression.json

bench-web-gin-regression: bench-web-gin
	@python3 scripts/compare-gin-adapter-benchmark.py --baseline "$(BENCH_BASELINE)" --candidate "$(BENCH_RESULTS)" --output "$(BENCH_REGRESSION_REPORT)"

check-bench-web-gin:
	@scripts/capture-gin-adapter-benchmark_test.sh
	@scripts/compare-gin-adapter-benchmark_test.sh

ci: tidy-check fmt-check vet lint test race check-bench-web-gin
