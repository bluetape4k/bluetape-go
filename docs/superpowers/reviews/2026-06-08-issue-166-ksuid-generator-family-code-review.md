# Issue #166 KSUID Generator Family Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-08 KST
브랜치: `issue-166-ksuid`
기준: `origin/develop` at `189388a6cb8b3c175ac2c4183efd2a5b91384a4a`
이슈: #166 `Port KSUID generator family`

## Step 6-R Integrated Result

| Tier | Reviewer | Initial P0 | Initial P1 | Initial P2 | Initial P3 | Final P0 | Final P1 | Final P2 | Final P3 | Result |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| Tier 1 Security | code-reviewer subagent | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | PASS |
| Tier 2 Ops/SRE reliability | code-reviewer subagent | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | PASS |
| Tier 3 Structural impact | architect subagent | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | PASS |
| Tier 4 Go code quality | local integrated review | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | PASS |
| Tier 5 Tests/types/silent failure | test-engineer subagent | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | PASS |
| Tier 6 Performance/stability | local integrated review | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | PASS |
| Tier 7 Documentation/release/evidence | local integrated review | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | PASS |

Final gate: `P0=0 P1=0 P2=0 P3=0`.

## Findings And Resolution

### P1 Fixed

- Test-engineer found that Segment `ksuid.FromParts` converts
  `t.Unix() - 1400000000` to `uint32`, so injected clocks before the KSUID epoch
  or beyond the maximum 32-bit seconds offset could wrap and produce a
  valid-looking KSUID with the wrong timestamp.
- Resolution:
  - `id/ksuid.go` now validates the captured `now` before entropy read or
    `segmentio/ksuid.FromParts`.
  - `id/ksuid_test.go` now covers before-epoch and after-max injected clocks and
    asserts no ID plus `ErrInvalidOptions`.

### P2 Fixed

- Test-engineer found deterministic KSUID coverage verified length, payload, and
  dependency round-trip but did not lock the exact canonical Base62 string.
- Resolution:
  - `TestKSUIDGeneratorUsesInjectedTimeAndEntropy` now asserts
    `3Epfe5fwjidB4aKO8WJm6x2QaIK` for the fixed 2026-06-08 timestamp and
    `abcdefghijklmnop` payload.

### P3 Fixed

- Security/Ops reviewer found that custom entropy readers and custom clocks
  needed a public concurrency-safety caveat.
- Structural reviewer found that `EntropyError` and README dependency-boundary
  wording still mentioned UUID/ULID but not KSUID.
- Local documentation review found WIP checklist numbering drift after #32
  closure.
- Resolution:
  - `WithKSUIDEntropy`, `WithKSUIDTime`, and `id/README*.md` now document custom
    concurrency-safety requirements.
  - `EntropyError` and `id/README*.md` now include KSUID in dependency/error
    boundary wording.
  - `WIP.md` release checklist numbering was repaired.

## 검증 증거

- `git diff --check`
- `go test -count=1 ./id`
- `go test -race -count=1 ./id`
- `go test -count=1 ./id -run 'TestKSUID|TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe|ExampleNewKSUIDGenerator' -v`
- `go test -race -count=1 ./id -run 'TestKSUID|TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v`
- `go test -count=1 ./...`
- `make race`
- `go vet ./...`
- `golangci-lint run ./...`
- `gofmt` check
- `go test -run '^$' -bench '^BenchmarkKSUIDNextString$' -benchmem ./id`
- `make ci`

`make ci` was rerun after staging the intentional `go.mod`/`go.sum` dependency
diff, and the full target passed, including `tidy-check`, `fmt-check`, `vet`,
`lint`, `test`, and `race`.
