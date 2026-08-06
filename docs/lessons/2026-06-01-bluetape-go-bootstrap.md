# bluetape-go Bootstrap

## Context

초기 계획은 좁은 `leader-go` repository였지만, 실제 경계는 `core`,
`testing`, `testcontainers` 같은 shared package와 `leader` 같은 domain package를
함께 담을 수 있는 단일 `bluetape-go` module이 더 적합했다.

## Decision

`github.com/bluetape4k/bluetape-go`를 하나의 Go module로 bootstrap한다. 첫
구현은 shared validation, eventual test helper, Redis Testcontainers fixture,
Redis-backed leader election처럼 작고 검증 가능한 표면으로 제한한다.

## Outcome

repository는 initial module structure, CI workflow, README, MIT license, Redis
leader smoke test, real container 기반 Testcontainers test를 실행하는 Nightly
workflow를 갖게 됐다.

## Verification

- `gofmt -w core testing testcontainers leader`
- `go mod tidy`
- `go test ./...`
- `actionlint .github/workflows/ci.yml .github/workflows/nightly-tests.yml`
- `golangci-lint config verify`
- `make ci`

## Future Guard

package boundary와 release cadence가 `bluetape-go` 안에서 증명되기 전에는
`leader-go`, `core-go`, `testcontainers-go`를 별도 repository로 나누지 않는다.

CI와 Nightly test command는 `go test -count=1`로 uncached 실행을 유지한다.
그래야 Testcontainers startup이 Go test cache에 가려지지 않는다.

`core`는 Kotlin extension shape를 port하지 말고 concept만 가져온다. error-returning
validation, 작은 generic helper, standard-library adoption 또는 deferred area의
명시적 문서화를 우선한다.

`collections`는 Go standard `slices`, `maps` package를 감싸지 않는다. chunking,
grouping, distinct-by-key, error-aware map/filter처럼 반복되는 service
transformation에만 helper를 추가한다.
