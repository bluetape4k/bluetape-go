# #520 EventBridge 감사 publisher 구현 리뷰

## 판정

- 기준 diff: `origin/develop...90dba26382713f715748ef1510df11e7ba42b3fd`
- 대상: `audit/sqloutbox/eventbridge`와 parent README locale pair, Go module,
  설계/계획/lesson 문서
- Step 6-R 통합 판정: `PASS (P0=0, P1=0, P2=0, P3=0)`
- 외부 AWS provisioning/live 호출: 범위 외이며 실행하지 않음
- 원격 PR CI: 아직 PR 생성 전이므로 `PENDING`

이번 실행 환경에서는 별도 native reviewer lane을 생성하지 않고 main
session이 여섯 관점과 통합 관점을 각각 독립 read-only pass로 수행했다. 따라서
독립 human/external review attestation은 제공하지 않으며, 아래 판정은 main
integration fallback의 exact-head 증적이다.

## 변경 요약

`Client`의 `PutEvents` method subset만 주입받는 immutable `Publisher`를 추가했다.
한 `Publish`는 검증된 `sqloutbox.Record`를 하나의 EventBridge entry로 전달하고,
`event_id`/`idempotency_key`를 detail에 보존한다. `EventBusName`이 비어 있으면
default bus pointer를 생략하며, retry/backoff, credentials, client lifecycle,
topology, downstream deduplication은 caller/operator가 소유한다.

주요 구현 근거:

- API와 constructor/typed-nil/zero-value 계약: `audit/sqloutbox/eventbridge/publisher.go:25-86`
- context checkpoint와 single-entry request mapping: `audit/sqloutbox/eventbridge/publisher.go:112-169`
- record identity, UTF-8, raw input/detail/entry size preflight:
  `audit/sqloutbox/eventbridge/publisher.go:187-335`
- safe sentinel/error wrapping과 allowlisted provider code:
  `audit/sqloutbox/eventbridge/errors.go:9-129`
- compile-checked public example: `audit/sqloutbox/eventbridge/example_test.go:13-30`
- fake deep-copy/context/blocking/concurrency contract:
  `audit/sqloutbox/eventbridge/publisher_test.go:25-72`, `458-482`
- parent/child README parity와 운영 경계: `audit/sqloutbox/README.md:99-108`,
  `audit/sqloutbox/README.ko.md:99-108`, `audit/sqloutbox/eventbridge/README.md:37-98`,
  `audit/sqloutbox/eventbridge/README.ko.md:37-99`
- dependency is limited to `github.com/aws/aws-sdk-go-v2/service/eventbridge v1.47.0`:
  `go.mod:5-17`

## 여섯 관점 + 통합 관점

| 관점 | P0 | P1 | P2 | P3 | 근거와 결론 |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | single-entry와 preflight bounded path (`publisher.go:187-221`)를 확인했다. 별도 성능 향상 주장은 없고, package normal/race와 full CI로 경로를 검증했다. |
| Stability | 0 | 0 | 0 | 0 | caller context를 전후로 확인하고 (`publisher.go:112-146`), nil/wrong-count/impossible-count/invalid EventId를 malformed로 닫는다 (`publisher.go:148-168`). |
| Security | 0 | 0 | 0 | 0 | provider message/detail/credential를 public formatting에서 제외하고 (`errors.go:28-95`), input UTF-8/size를 dispatch 전에 거부한다 (`publisher.go:224-335`). redaction과 invalid UTF-8 응답 테스트가 있다. |
| Operator/Ops | 0 | 0 | 0 | 0 | logger, retry, credential, topology를 설치하지 않으며 README에 caller/operator 경계를 명시했다 (`eventbridge/README.md:77-85`). |
| Developer/API | 0 | 0 | 0 | 0 | narrow interface, constructor validation, compile assertion/example, documented constructor-only zero value를 확인했다 (`publisher.go:25-56`, `doc.go:1-11`). |
| User/Caller | 0 | 0 | 0 | 0 | exact Source/DetailType와 stable outbox identity를 보존하고 EventBridge response EventId와 분리한다 (`publisher.go:128-136`, README:72-75). |
| Main integration | 0 | 0 | 0 | 0 | #520 범위에만 변경했고 기존 Relay의 at-least-once/cancellation semantics와 README EN/KO parity를 유지했다. #521/#522 이후 transport는 범위 밖이다. |

P0/P1 차단 finding은 없다. 생산 코드 concurrency scan에서 goroutine,
`GlobalScope`, `runBlocking`, unsafe 또는 global logger는 발견되지 않았고,
`context.Background()`는 nil context 정규화 목적의 `publisher.go:344-349`에서만
사용한다. 테스트의 goroutine/fake mutex는 `publisher_test.go:25-72,458-482`에
한정된다.

## 검증 증적

| 명령 | 결과 |
|---|---|
| `go test -count=1 ./audit/sqloutbox/eventbridge` | PASS |
| `go test -race -count=1 ./audit/sqloutbox/eventbridge` | PASS |
| `go vet ./audit/sqloutbox/eventbridge` | PASS |
| `go test -count=1 ./audit/sqloutbox/...` | PASS |
| `go test -race -count=1 ./audit/sqloutbox/...` | PASS |
| `make fmt-check` | PASS |
| `make tidy-check` | PASS |
| `make vet` | PASS |
| `make lint` | PASS, `0 issues` |
| `make test` | PASS, repository-wide package tests |
| `make race` | PASS, repository-wide race tests |
| `make ci` | PASS, repository gates와 benchmark/diagnostic guards |
| `git diff --check origin/develop...HEAD` | PASS |
| Codex self-audit + bluetape contract validator | PASS, `Contract issues: 0` |

Testcontainers-backed 기존 package도 repository-wide `make test`, `make race`,
`make ci`에서 통과했으며, live AWS endpoint/credential는 접근하지 않았다.
`go test -run Example -count=1 ./audit/sqloutbox/eventbridge`는 전체 package
검증에 포함된 compile-checked example로 별도 live setup 없이 확인된다.

## 남은 게이트와 위험

- `PENDING`: semantic branch push, PR 생성, 원격 exact-head CI와 required matrix.
- `PENDING`: merge 직전 fresh head/base/checks/thread 재검토 및 사용자 명시 승인.
- 범위 외: AWS bus/rule/target/IAM provisioning, live smoke, release/tag/changelog.
- 독립 human review attestation은 solo-developer 실행 범위에서 N/A이며, main
  session six-lens fallback으로 대체했다.

다음 mutation은 PR CI가 exact head에서 green인 뒤에만 진행한다. merge는
`--match-head-commit`으로 관찰된 SHA를 고정하고, 승인 전 auto-merge를 사용하지
않는다.
