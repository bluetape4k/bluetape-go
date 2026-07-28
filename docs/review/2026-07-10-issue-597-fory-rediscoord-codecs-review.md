# Issue #597 Fory Rediscoord Codecs Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10 KST
범위: `origin/develop...HEAD`
Baseline: `origin/develop` at `21f402b2d83559268f399a4f2f02c86fa2c59af8`

## 증거

- Apache Fory Go is pinned at `v1.3.0` and imported only by the opt-in
  `cache/rediscoord/fory` child package.
- Native-fast and native-compatible codecs explicitly disable xlang and
  reference tracking, apply bounded metadata options, and wrap payloads in the
  fail-closed `BTFY` envelope.
- The codec copies Fory's runtime-owned marshal bytes while holding its mutex;
  mutex release is defer-based and codec copies share one internal runtime and
  synchronization state.
- `MaxPayloadBytes` is enforced before wrapper allocation. Positive
  `MaxResultBytes` uses an exact JSON/base64 size preflight before encoding,
  bounds Redis reads with `GETRANGE max+1`, and checks again before publication.
- Provider and registration causes are replaced by sanitized sentinel causes;
  `CodecError.Error()` and `Unwrap()` do not expose payload/provider text.
- Registration and provider panics, including Fory's optional panic-on-error
  mode, are recovered at their package boundaries and converted to sanitized
  sentinels.
- Both profiles prove nil and empty `[]byte`, empty string, zero struct,
  compatible added-field, malformed envelope/provider, constructor panic,
  copied-codec concurrency, and concurrent round-trip behavior.
- `go test -count=1 ./cache/rediscoord/fory` passed.
- `go test -race -count=1 ./cache/rediscoord/fory` passed.
- `go test -p 1 -count=1 ./cache/rediscoord` passed.
- `go test -race -p 1 -count=1 ./cache/rediscoord` passed.
- `make fmt-check`, `make tidy-check`, `make vet`, and `make lint` passed;
  lint reported `0 issues`.
- Repository-wide `make test` did not pass on this machine. Repeated runs
  failed in unchanged PostgreSQL, MongoDB, and Redis Testcontainers packages
  with connection-refused, connection-reset, readiness timeout, and malformed
  startup-response symptoms. The changed packages passed in those runs and in
  isolated normal/race runs.

## 7-Tier 검토

| Lane | P0 | P1 | Decision |
| --- | ---: | ---: | --- |
| Performance | 0 | 0 | Size preflight, panic-safe locking, and allocation fixes applied. |
| Stability | 0 | 0 | Changed-package normal/race evidence is green. |
| Security | 0 | 0 | Trusted-internal boundary, bounds, and redaction are explicit. |
| Operator/Ops | 0 | 0 | Rollout tuple, rollback, and bounded cleanup are documented. |
| Developer/API | 0 | 0 | Root whitelist, copy-safe shared state, wire limit, and zero semantics are explicit. |
| User/Caller | 0 | 0 | Compile-checked examples and EN/KO usage/runbook are aligned. |
| Main integration | 0 | 0 | Scope remains #597; direct Redis cache and benchmarks remain separate. |

## PR 이후 검토

PR #600 Step 7-R initially found one P1: a panic from `Options.Register`
escaped codec construction. The correction adds a constructor panic boundary,
returns a redacted registration `CodecError`, and proves panic text is not
formatted or unwrapped.

The same correction closes the Developer/API copy-safety P2 by moving the Fory
runtime and mutex into one shared internal state. A copied `Codec` therefore
uses the same lock, with race-tested concurrent round trips. The review also
expanded unsupported-root and zero-value negative paths, clarified that
`CodecError.Unwrap()` exposes only a sanitized package cause, and added a
compile-checked Fory `StampedeCache` integration example.

## 유예한 작업

- `MaxResultBytes=0` intentionally preserves the existing unlimited behavior.
  Fory deployments should set a positive value as shown in the README. Making
  a bounded default mandatory would be a separate compatibility change; the
  provider remains scoped to trusted internal Redis traffic.
- Issue #598 owns direct Redis value-cache provider behavior.
- Issue #599 owns JSON versus native-fast versus native-compatible benchmarks.
  Its deliverable must include the exact command and environment, raw result
  path, result table, Chart, and written analysis. This branch makes no
  throughput or storage claim.

P0=0 P1=0
