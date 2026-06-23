Resolves #222.

## Summary

- Added focused compile-checked `testing` examples for table tests, package-local
  builders, golden files, deterministic random data, and cancellation
  assertions.
- Documented the Go-native testing shape in English and Korean READMEs.
- Kept the scope intentionally narrow: no assertion DSL, faker dependency, or
  JUnit-style parameter-source API.

## Review

- Step 6-R 7-tier review artifact is included under `docs/superpowers/reviews/`.
- Step 6-R verdict: P0=0, P1=0.
- Go stress: `GoroutineStressTester` and `AsyncJobTester` are not applicable;
  the examples introduce no shared state, goroutine lifecycle, or
  goroutine-safe public claim.

## Validation

- PASS `go test -count=1 ./testing`
- PASS `go test -race -count=1 ./testing`
- PASS `make fmt-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `git diff --check`
- PASS staged `make tidy-check`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] Table-test example added.
- [x] Package-local fixture builder example added.
- [x] Golden-file example added under package `testdata`.
- [x] Deterministic random data example added.
- [x] Cancellation assertion example added.
- [x] English and Korean docs updated.
- [x] 7-tier review completed with P0=0 P1=0.
- [x] Final local gates completed.
- [ ] GitHub CI pending.
