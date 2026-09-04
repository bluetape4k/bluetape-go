# #523 SQS S3 envelope 구현 리뷰

## 2026-09-04 보강 검토

독립 performance/security/API 리뷰에서 확인된 P1/P2 위험을 재검증하고 다음
수정을 반영했다.

- `Receive`는 기본 512 MiB aggregate payload budget을 envelope preflight에서
  계산하며, 초과 batch는 S3 `GetObject` dispatch 전에 `ErrPayloadTooLarge`로
  종료한다. near-limit 두 object 회귀 테스트에서 S3 호출 0회를 확인했다.
- `GetObject` response 직후 cancellation 경로도 response body를 닫고, S3/SQS
  side effect 뒤 cancellation은 `ErrCanceled`와 `OrphanedObject`/
  `QueueDeleted` 상태를 보존한다.
- SDK가 cancellation 시 nil output을 반환해도 side effect 완료 여부를
  결정할 수 없으므로 동일한 orphan/queue-deleted 상태를 보존한다. SQS
  `DeleteMessage`가 output과 error를 함께 반환하는 경우에도 queue ack
  가능성을 `QueueDeleted`로 전달한다.
- 첫 side effect 이후 다음 SDK dispatch 직전의 cancellation도 동일한 상태로
  반환하며, 결정적 context fake가 queue/object dispatch 0회를 회귀 검증한다.
- payload 하나의 SHA-256 digest를 envelope와 S3 checksum에 재사용하고,
  `Error.GoString`과 `%v %+v %#v` adversarial 검증으로 provider cause redaction을
  고정했다.

보강 후 이 worktree의 targeted test, race, vet와 `git diff --check`가 PASS이며
현재 구현 판정은 `P0=0, P1=0`이다. 원격 CI와 merge gate는 새 commit의 exact head에서
다시 확인한다.

운영 관점의 P2로 `ReceiveMessage` 뒤 object read 전 cancellation 시 receipt
handle을 반환하지 않는 API 경계를 확인했다. visibility/retry/reconciliation
소유권을 EN/KO README, spec와 lesson에 명시했으며 API 확장은 이 범위에 넣지
않는다. P0/P1 차단은 없다.

## 판정

- 검토 대상 구현 tree: baseline `HEAD=906a68fdb41551ccaa6ce1394a2370e654ade10e`와
  이 worktree의 issue #523 변경
- 대상 범위: `messaging/sqsextended`, root README locale pair, spec/plan/risk와
  이 구현 리뷰·lesson 문서
- Step 6-R 통합 판정: `PASS (P0=0, P1=0, P2=1, P3=0)`
- 원격 PR/CI와 live AWS 호출: 아직 실행하지 않음. 부모 workflow의 PR gate에서
  exact head 기준으로 별도 확인한다.

이번 구현은 SQS 전체 wrapper가 아니라 caller-owned AWS SDK for Go v2 client의
최소 method subset을 받는 Go-native adapter다. S3 object와 SQS envelope의
ownership, failure, cancellation, cleanup 순서를 package API와 fake-first test로
고정했다.

## 변경 요약과 근거

- version 1 canonical JSON envelope와 SHA-256 checksum:
  `messaging/sqsextended/envelope.go:11-87`
- duplicate/unknown/trailing/non-canonical field와 64 KiB envelope bound:
  `messaging/sqsextended/envelope.go:89-171`
- narrow SQS/S3 interface, typed-nil constructor, payload bound:
  `messaging/sqsextended/provider.go:20-84`
- S3-first send와 SQS failure 후 explicit orphan cleanup:
  `messaging/sqsextended/provider.go:109-181`
- bounded S3 read, exact size/checksum, no automatic acknowledgement:
  `messaging/sqsextended/provider.go:203-292,409-435`
- SQS-first delete와 `QueueDeleted` 상태:
  `messaging/sqsextended/provider.go:298-379`
- sentinel/typed error와 provider diagnostic redaction:
  `messaging/sqsextended/errors.go:9-129`
- fake deep-copy, output anomaly, cancellation, ordering과 concurrent isolation:
  `messaging/sqsextended/provider_test.go:25-527`
- compile-checked usage example: `messaging/sqsextended/example_test.go:10-64`
- EN/KO package 운영 경계: `messaging/sqsextended/README.md`와
  `messaging/sqsextended/README.ko.md`

## 여섯 관점과 통합 관점

| 관점 | P0 | P1 | P2 | P3 | 결론 |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | envelope 64 KiB와 payload 256 MiB 기본 상한, `ContentSize+1` bounded read로 입력·응답 확대를 제한한다. 별도 latency/throughput 주장은 하지 않는다. |
| Stability | 0 | 0 | 0 | 0 | SDK response 직후 context를 확인하고 response body를 모든 read 경로에서 닫으며, malformed output을 sentinel로 분류한다. |
| Security | 0 | 0 | 0 | 0 | bucket/key, payload, checksum과 raw AWS error를 public formatting에서 제외하고 canonical/UTF-8/size 검증을 dispatch 전에 수행한다. |
| Operator/Ops | 0 | 0 | 0 | 0 | credentials, retry, timeout, logger, queue/bucket/IAM, lifecycle, DLQ, replay와 visibility extension은 caller/operator가 소유한다. |
| Developer/API | 0 | 0 | 0 | 0 | narrow interface, immutable provider configuration, compile-time SDK assertions, Korean Go doc과 EN/KO README를 확인했다. |
| User/Caller | 0 | 0 | 1 | 0 | caller bucket/key를 그대로 유지하고 S3→SQS send, SQS→S3 delete order와 orphan/queue-deleted 상태를 관찰할 수 있다. Receive cancellation의 receipt/visibility reconciliation 경계는 문서화했다. |
| Main integration | 0 | 0 | 0 | 0 | #523 범위에만 변경했으며 parent #517의 후속 SQS/Kinesis/Step Functions와 운영 provisioning은 건드리지 않았다. |

P0/P1 차단 finding은 없다. live AWS/emulator compatibility, production IAM와
orphan sweeper는 이 package의 증명 범위가 아니며 README/spec에 명시적으로
caller-owned 또는 defer로 기록했다.

## 검증 증적

| 명령 | 결과 |
|---|---|
| `gofmt -w messaging/sqsextended/*.go` | PASS |
| `go test -count=1 ./messaging/sqsextended` | PASS (0.220s) |
| `go test -race -count=1 ./messaging/sqsextended` | PASS (1.408s) |
| `go vet ./messaging/sqsextended` | PASS |
| `go test -run Example -count=1 ./messaging/sqsextended` | PASS (0.348s) |
| `make fmt-check` | PASS |
| `make tidy-check` | PASS |
| `make vet` | PASS |
| `make lint` | PASS (`0 issues`) |
| `git diff --check`와 신규 파일 trailing-whitespace scan | PASS |
| `make test` | 전체 target은 기존 `leader/sql.TestPostgresLifecycle/renewal`의 1.001초 timeout으로 실패했으나 `messaging/sqsextended`는 PASS; 실패 package 단독 재실행은 PASS (5.863s) |

`make test`의 failure는 변경 package가 아닌 기존 `leader/sql` Testcontainers
renewal 경로에서 발생했고, 해당 package 단독 재실행은 통과했다. 검증하지 않은 live AWS, LocalStack, Floci endpoint와
원격 PR CI는 PASS로 간주하지 않는다.

## 남은 게이트

- 부모 agent가 이 worktree 변경을 통합한 뒤 원격 PR exact-head CI를 확인한다.
- merge, branch cleanup, workflow dispatch와 release/tag는 부모의 별도 승인
  gate에 남긴다.
