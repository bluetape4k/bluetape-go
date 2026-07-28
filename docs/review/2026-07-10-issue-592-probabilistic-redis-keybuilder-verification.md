# Issue #592 Probabilistic Redis Key Builder Verification

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10 KST
게이트: Step 5 verifier
Baseline: `068df42615303090be1f57e03a494b596962e8e7`

## 요구사항 증거

| Spec invariant | Evidence | Verdict |
|---|---|---|
| Bloom key bytes remain exact | `TestBuildKeysUsesClusterHashTag` plus adapter `StructuralKey` output; focused targeted test passed. | PASS |
| HLL key bytes remain exact | `TestBuildHyperLogLogKeyKeepsSharedBuilderCompatibleLayout`; focused targeted test passed. | PASS |
| Colons remain valid in hash tag | `TestKeyBuilderForNamespaceKeepsClusterHashTag` uses `tenant-a:emails` and exact `:bits` key. | PASS |
| Local validation remains first | `keyBuilderForNamespace` calls `validateNamespace` before builder construction; invalid-input regression test passed. | PASS |
| Short local redacted ID remains | Existing `redactedRedisKeyID` is unchanged; new HLL marker test requires `redis-key:[0-9a-f]{12}` with no raw marker/key leak. | PASS |
| `RedisError` and metadata sentinels remain | `errors.go` is untouched; focused package tests include provider behavior and configuration metadata cases. | PASS |
| Scripts, commands, algorithms remain | Diff only changes `keys.go` and `options_test.go`; no Lua/filter/HLL command source changed. | PASS |
| No public API or README behavior changes | No exported identifiers or README files changed; `git diff --stat` confirms two implementation/test files only before this artifact. | PASS |

## 최신 검증 증거

| 명령 | 결과 |
|---|---|
| `go test -count=1 ./probabilistic/redis -run 'KeyBuilderForNamespace|KeepsSharedBuilderCompatibleLayout'` before adapter | Expected RED: `undefined: keyBuilderForNamespace`. |
| Same focused key/validation/error suite after adapter | PASS. |
| `make fmt-check` | PASS. |
| `make tidy-check` | PASS. |
| `go vet ./probabilistic/redis ./redis` | PASS. |
| `go test -p 1 -count=1 ./probabilistic/redis ./redis` | PASS. |
| `go test -p 1 -race -count=1 ./probabilistic/redis` | PASS. |
| `git diff --check` | PASS. |

## 경계 판정

The adapter maps any fixed-prefix, hash-tag, or structural-key builder failure
to a local opaque configuration error without wrapping shared `redis` errors.
For caller input, local `validateNamespace` remains the first and observable
validation contract. The existing package-specific redaction and `RedisError`
surface are unchanged.

## 남은 필수 게이트

Run Step 6/6-R on the implementation diff, then full repository CI with
`TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false` before
publication. Benchmark evidence remains N/A; issue #560 owns any required
cross-provider table, chart, and analysis.
