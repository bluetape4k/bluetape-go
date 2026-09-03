# Issue #524 CloudWatch 예시 lesson

CloudWatch Metrics와 Logs는 AWS라는 공통 공급자를 사용해도 batching,
ordering, cardinality, sequence-token 계약이 서로 다르다. 작은 compile-only
예시라도 signal별 request limit과 timestamp/range를 preflight하고, fake가 SDK
request를 깊은 복사로 보존하며, SDK 응답 직후 cancellation과 partial rejection을
다시 확인해야 한다.

`PutLogEvents` partial response는 수락된 event와 거부된 event를 함께 만들 수
있다. typed rejection metadata를 사용해 거부 index만 재시도하고 전체 batch
중복 전송을 피하는 책임은 caller에게 둔다. entity rejection은 event 수락이
부분적일 수 있으므로 별도 reconciliation 정책으로 다룬다.

다음 observability 작업에서는 raw payload와 고카디널리티 값을 기본 logger나
metric label로 노출하지 않고, deprecated sequencing contract를 현재 문서와
테스트에 함께 고정한다. 실제 credentials/live endpoint는 기본 CI 증적이
아니며 caller가 opt-in한다.
