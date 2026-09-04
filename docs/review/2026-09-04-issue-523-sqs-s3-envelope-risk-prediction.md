# Issue #523 risk prediction

| 위험 | 가능성/영향 | 예방·검증 |
|---|---|---|
| `PutObject` 성공 뒤 `SendMessage` 실패 | 높음/중간: orphan storage | 자동 cleanup 금지, `OrphanedObject` 상태와 explicit delete capability를 테스트한다. |
| visibility timeout 만료 전 payload read 실패 | 중간/높음: duplicate delivery | caller가 `VisibilityTimeout`을 전달하고 확장 정책을 소유한다는 문서/요청 pass-through를 검증한다. |
| `GetObject`가 declared size보다 많은 bytes 반환 | 중간/높음: memory exhaustion/corruption | configured max와 `ContentSize+1` bounded read를 사용하고 extra byte를 거부한다. |
| payload checksum mismatch | 중간/높음: silent corruption | SHA-256 exact compare와 no-delete test를 둔다. |
| SQS ack 후 S3 delete 실패 | 중간/중간: storage leak | SQS-first order, typed `ErrObjectDeleteFailed`, queue-deleted observability를 고정한다. |
| provider error/detail가 public error에 노출 | 중간/높음: secret/PII leak | safe sentinel/operation만 포맷하고 `%+v` redaction test를 둔다. |
| fake가 SDK request/body를 alias | 중간/중간: false tests/race | fake deep-copy와 concurrent request isolation test를 둔다. |
| SDK response shape drift 또는 nil output | 낮음/높음: panic/ambiguous result | nil output, missing ID, missing body와 `ErrMalformedOutput` matrix를 둔다. |
| envelope duplicate/unknown/trailing field | 중간/중간: ambiguous wire contract | strict decoder와 canonical re-marshal equality를 검증한다. |

판정: 위 위험은 package-local fake/race 검증으로 다루며, live AWS credential,
queue/bucket provisioning과 emulator compatibility는 범위 밖이다.
