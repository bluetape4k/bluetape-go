# Issue #524 CloudWatch 예시 사전 위험 검토

## 예측 위험

| 위험 | 완화/증적 |
|---|---|
| Metrics/Logs limits 누락 | request builder validation과 table-driven limit tests |
| cancellation 이후 SDK 호출/성공 반환 | dispatch 직전과 response 직후 `ctx.Err()` 검사 |
| payload 또는 provider message 노출 | 고정 sentinel/operation error와 redaction assertions |
| sequence token의 오래된 직렬화 가정 | README와 example에서 현재 parallel PutLogEvents 계약 명시 |
| 고카디널리티 기본 계측 | dimension count만 검증하고 raw values를 오류/label로 출력하지 않음 |
| live credential 의존 | fake client와 compile-only examples, opt-in live path 없음 |

판정: P0/P1 위험은 구현·검증 단계에서 차단한다. 새로운 global state,
retry worker, provider abstraction이 필요해지면 이 계획을 중단하고 설계를
갱신한다.
