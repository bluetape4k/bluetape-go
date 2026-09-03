# Issue #524 CloudWatch 예시 lesson

CloudWatch Metrics와 Logs는 AWS라는 공통 공급자를 사용해도 batching,
ordering, cardinality, sequence-token 계약이 서로 다르다. 작은 compile-only
예시라도 signal별 request limit을 preflight하고, fake가 SDK request를 깊은
복사로 보존하며, SDK 응답 직후 cancellation을 다시 확인해야 한다.

다음 observability 작업에서는 raw payload와 고카디널리티 값을 기본 logger나
metric label로 노출하지 않고, deprecated sequencing contract를 현재 문서와
테스트에 함께 고정한다. 실제 credentials/live endpoint는 기본 CI 증적이
아니며 caller가 opt-in한다.
