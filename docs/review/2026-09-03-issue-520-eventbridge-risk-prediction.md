# Issue #520 EventBridge 위험 예측

## 예측 기준

대상은 `service/eventbridge v1.47.0` SDK와 `audit/sqloutbox.Publisher` 사이의
경계다. 위험은 구현 전에 예측하고, 각 mitigation은 계획의 파일 또는
실행 명령으로 추적한다.

| 위험 | 가능성 | 영향 | 완화/검증 |
|---|---|---|---|
| SDK generated response shape 또는 module dependency drift | 중간 | 중간~높음 | `v1.47.0` direct pin, `var _ Client = (*awseventbridge.Client)(nil)`, `go list -m`, `go mod tidy`, `go vet` |
| AWS entry size가 detail cap보다 큰 metadata를 포함 | 높음 | 높음 | encoded detail + UTF-8 source/type/bus preflight, 256 KiB 미만 boundary와 `+1` 테스트 |
| network return 직후 context cancellation | 중간 | 높음 | response 직후 `ctx.Err()` 우선 검사, fake blocking/after-cancel test, relay status contract test |
| per-entry `ErrorMessage` 또는 transport error가 secret을 노출 | 중간 | 높음 | sanitized `Error`, safe code allowlist, `errors.Is` cause-only, `Error()`/`%+v`/redaction assertions |
| fake가 input pointer/byte slice를 alias해 false positive | 중간 | 중간 | mutex-safe deep-copy fake, request isolation test, `go test -race` |
| caller가 bus/rule/target 또는 downstream idempotency를 adapter 책임으로 오해 | 중간 | 높음 | child/parent README와 Go doc에 ownership 표기, no provisioning/client construction API |
| relay가 cancellation과 retryable transport failure를 혼동 | 낮음~중간 | 높음 | existing `Relay.RunOnce` integration matrix: cancellation leaves claimed, transport/entry failure marks failed |
| default bus pointer를 빈 문자열로 전송해 AWS가 잘못된 bus를 조회 | 낮음 | 중간 | constructor/accessor test와 captured request에서 `EventBusName == nil` 확인 |
| invalid record 또는 oversized detail이 SDK까지 도달 | 낮음 | 높음 | preflight call count `==0`, entry identity/`Entry.Validate` table tests |
| public README locale drift | 중간 | 중간 | EN/KO required-term/heading parity check, docs review artifact, `git diff --check` |

## stop conditions

P0/P1 보안·정합성 위험, transport error redaction 실패, entry size preflight
실패, cancellation post-response 우선순위 실패가 발견되면 PR/merge gate를
열지 않는다. AWS live provisioning 또는 새 batching/retry abstraction이
필요해지면 #520 범위를 중지하고 별도 issue/설계를 만든다.

## 판정

예측된 위험은 Task 2~6의 concrete test/code/docs guard로 커버된다. 현재
외부 의존성의 실제 runtime 동작은 fake 및 remote CI에서 확인하기 전까지
`PENDING`이며, 이는 구현 완료 주장이 아니다.
