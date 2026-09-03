# Issue #524 CloudWatch metrics 및 logs 예시 설계

## 상태와 범위

- 상태: 사용자가 승인한 설계
- 부모 이슈: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
- 작업 이슈: [#524](https://github.com/bluetape4k/bluetape-go/issues/524)
- 구현 worktree: `feat/issue-524-cloudwatch-examples`
- 기준 head: `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 대상: `examples/cloudwatch` compile-checked examples와 bilingual README

이 변경은 애플리케이션이 AWS SDK for Go v2의 CloudWatch Metrics와 CloudWatch
Logs를 호출자 소유 client로 사용하는 경계를 보여준다. 예시는 인증, client
lifecycle, global logger/registry, retry, provisioning, live credentials를
소유하지 않는다.

## 결정

Metrics와 Logs는 서로 다른 서비스 계약이므로 하나의 공통 publisher나 전역
상태로 합치지 않는다. 테스트는 SDK method subset을 주입하는 fake로 request
shape, context 전달, 호출 전/후 cancellation, limits, redacted diagnostics를
검증한다. 예시 함수는 단일 호출의 bounded request를 만들고 호출자 deadline을
존중한다.

`PutMetricData` 예시는 한 request에 최대 1,000 metrics, 30 dimensions와
1 MiB HTTP payload 제한을 문서화하며 metric name/namespace와 dimension
cardinality를 입력 단계에서 확인한다. `NaN`/`Inf`는 전송하지 않는다.

`PutLogEvents` 예시는 시간순 event, event당 1 MiB, batch 10,000 events,
전체 1 MiB(각 event당 26 bytes overhead 포함), 24시간 span을 적용한다.
현재 CloudWatch Logs sequence token은 무시/폐기되었고 동일 stream에 대한
병렬 `PutLogEvents`가 허용된다는 사실을 README와 example 주석에 고정한다.

오류 문자열에는 metric 값, log body, AWS provider message가 들어가지 않는다.
호출자는 `errors.Is`로 cancellation을 검사하고, 안전한 operation/sentinel만
관찰성 hook이나 caller-owned logger에 전달한다. high-cardinality dimension,
log field와 raw payload를 기본 계측 label로 사용하지 않는다.

## 비목표

- CloudWatch client/config/credential/retry/timeout/lifecycle 관리
- global logger, metrics registry, OpenTelemetry exporter 또는 batching worker
- live AWS/LocalStack/Floci 기본 테스트, IAM/provisioning, release/publish
- Logs sequence-token cache/serialization 또는 Metrics aggregation policy
- CloudWatch alarms, dashboards, retention, subscription filter 설정

## SPW gate

| 단계 | 증적 |
|---|---|
| SPW-01 요구사항 | live issue body와 `bluetape4k-github`/공식 SDK 계약 확인 |
| SPW-02 설계 | 이 문서의 분리된 Metrics/Logs API, limits, ownership 결정 |
| SPW-03 계획 | `docs/superpowers/plans/2026-09-04-issue-524-cloudwatch-examples-plan.md` |
| SPW-04 구현 | fake-first RED→GREEN example/test diff |
| SPW-05 검증 | targeted test, race, fmt/tidy/vet 및 repository CI 증적 |
