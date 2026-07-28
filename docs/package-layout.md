# Package Layout Policy

## 목표

- public package는 작고 idiomatic하며 독립적으로 유용하게 유지한다.
- catch-all utility package를 만들지 않는다.
- bluetape-go wrapper를 추가하기 전에 Go standard library와 검증된 dependency를
  우선한다.
- 명확한 public contract가 생기기 전까지 implementation detail은 private로
  유지한다.

## Public Packages

Top-level directory는 안정적인 user-facing API를 표현할 때 public package다.

- `core`
- `testing`
- `testcontainers/...`
- `leader`
- `leader/redis`

향후 public package도 같은 규칙을 따른다. 명확한 domain, example, package docs,
test가 있을 때만 top-level package를 만든다.

## `internal`

package 사이에서 공유하지만 public API로 의도하지 않은 코드는 `internal/`을
사용한다.

좋은 후보:

- 사용자에게 직접 유용하지 않은 shared test helper.
- serializer, compressor, lock, retry policy의 implementation detail.
- public API가 아직 안정화되는 동안 필요한 compatibility shim.

user-facing package를 `internal/` 아래에 두지 않는다. 사용자가 import해야 한다면
docs와 test를 갖춘 public package에 속한다.

## Package Documentation

모든 public package는 release-ready로 간주되기 전에 package documentation을
가져야 한다.

- package-level purpose.
- primary API example.
- 관련이 있을 때 concurrency와 context behavior.
- 노출되는 경우 error semantics와 sentinel error.
- 해당되는 경우 bluetape4k Kotlin/JVM behavior와의 compatibility note.

## Source Comments

이 repository의 source comment는 한국어로 작성한다. 짧고 Go-native하게 유지하되,
exported declaration comment는 여전히 exported identifier로 시작하고 한국어 조사
앞에 공백을 둔다. 그래야 `go doc`, pkg.go.dev, linter가 comment를 API와 연결할
수 있다.

## Examples

실제 backend 문제를 해결하는 example을 우선한다.

- leader-guarded scheduler.
- cache warmer.
- migration gate.
- Redis near-cache invalidation.
- resilient HTTP client/server call.
- LocalStack-backed AWS example.
