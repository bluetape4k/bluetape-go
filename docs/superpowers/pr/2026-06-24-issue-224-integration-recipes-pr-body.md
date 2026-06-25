Closes #224.

## Summary

- Add `examples/integration` with compile-checked recipes across batch,
  workflow, cache, resilience, id, JWT, Redis lock/leader, and
  Testcontainers Redis.
- Document service-free, race, and Docker-backed smoke commands in English and
  Korean package READMEs.
- Link the new example package from the root English and Korean READMEs.

## Validation

- PASS `go test -count=1 ./examples/integration`
- PASS `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 go test -p 1 -count=1 ./examples/integration`
- PASS `go test -race -count=1 ./examples/integration`
- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## Review

- Step 6-R: P0=0 P1=0, main integration fallback with seven-lane separation.
- Step 7-R: PENDING

## DoD Status

- PASS #224 runnable integration recipes.
- PASS English/Korean docs sync.
- PASS local validation gates.
- PENDING PR metadata parity with #224.
