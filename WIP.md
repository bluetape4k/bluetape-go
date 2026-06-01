# WIP

## Current Milestone

`0.1.0` - Foundation packages: core support, collections, goroutine helpers,
codecs/compression, Redis leader election, and shared test infrastructure.

## Current Setup Work

- Establish bilingual README coverage.
- Add project hygiene files: changelog, WIP log, Makefile, lint config, research
  index, package layout policy, and release guide.
- Strengthen CI before continuing feature issues.
- Keep both regular CI and Nightly workflows running Testcontainers-backed tests
  against real containers.
- Implement issue #8 core support as small idiomatic Go helpers, not a direct
  Kotlin extension API port.
- Implement issue #9 collections as focused generic slice/map helpers that do
  not duplicate Go's standard `slices` and `maps` packages.

## Next Feature Issues

1. #8 Port core support functions into idiomatic Go helpers.
2. #9 Port collection slice and map helper functions.
3. #10 Design goroutine and context helper package.
4. #12 Design binary serializer interfaces and safe defaults.
5. #13 Port compressor package with streaming support.
6. #14 Harden leader API contracts and lifecycle semantics.
7. #15 Decide Redis key compatibility with bluetape4k-leader.

## Decision Log

- Keep `README.md` and `README.ko.md` synchronized when package scope, roadmap,
  install guidance, or development commands change.
- Use `develop` as the default branch and run CI on `develop`, `main`, and pull
  requests.
- Keep public packages idiomatic to Go. Do not mechanically port Kotlin
  extension APIs.
- Prefer small packages with clear service value over catch-all utility bags.
- Use Testcontainers-backed smoke tests for infrastructure packages.
- Use `-count=1` in CI and Nightly test commands so Go's test cache cannot hide
  Testcontainers execution.
- Keep `core` limited to helpers that reduce repeated service code while staying
  obvious to Go readers.
- Keep `collections` focused on transformations with clear service value:
  chunking, grouping, distinct-by-key, and error-aware map/filter.
- Document milestone research before adding broad implementation issues.
