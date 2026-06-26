# Issue 337 - Lingua Language Boundary

## Lesson

Lingua-Go is the closest parity path for the source `lingua` module, but its
language models and transitive dependencies are operationally meaningful. Keep
the dependency behind `textsearch/language` and verify `go list -deps
./textsearch` stays free of Lingua imports.

## Boundary

- Build detectors once per language subset and reuse them across goroutines.
- Prefer caller-selected subsets over all-language detectors when the domain is
  known.
- Keep unknown input as a caller-visible `Detected=false` result, not an
  operational failure.
- Treat language detection as a preprocessing hint, not a security or
  moderation boundary.
- Use `GoroutineStressTester` and `go test -race` for shared detector reuse
  claims.

## Verification Targets

- `go test -count=1 ./textsearch/language`
- `go test -race -count=1 ./textsearch/language`
- `go list -deps ./textsearch | rg "pemistahl|lingua-go" || true`
- `make ci`
