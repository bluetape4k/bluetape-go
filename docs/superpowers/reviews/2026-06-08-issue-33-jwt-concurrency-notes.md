# Issue #33 JWT Concurrency Notes

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Task: JWT local rotation and parse concurrency evidence
이슈: #33
날짜: 2026-06-08

## 범위

The #33 `jwt` package performs local CPU/crypto work and process-local
in-memory key lookup. It does not expose external I/O, background workers,
caller-visible cancellation boundaries, or distributed repositories.

## Lock Scope

- `keyChainRepository.find` uses `sync.RWMutex.RLock` only while scanning the
  in-memory retained key slice.
- `keyChainRepository.rotate` double-checks the current key under the write lock
  before creating an expiration-triggered replacement, so concurrent post-expiry
  compose calls converge on one new current key instead of evicting each other.
- `Provider.keyFunc` calls `repo.find` during the `golang-jwt` key callback and
  receives copied verification key material before signature verification
  continues.
- No repository write lock is held during JWT signature verification.
- `keyChainRepository.forceRotate` builds the next `KeyChain` before acquiring
  the write lock, then holds the write lock only while prepending and evicting
  retained keys.

## Stress Coverage

`TestConcurrentComposeParseAndRotate` uses `GoroutineStressTester` with
concurrent compose/parse and forced-rotation tasks against one shared provider.
The package race gate is:

```bash
go test -race -count=1 ./jwt
```

## AsyncJobTester

AsyncJobTester N/A: #33 core JWT operations are local CPU/crypto work with no caller-observable cancellation boundary.

Future distributed repositories in #173 must introduce `context.Context`
contracts and cancellation/deadline tests for Redis, Mongo, or other external
I/O adapters.
