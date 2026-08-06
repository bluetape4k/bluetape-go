# Issue #63 SQS/SNS Example 교훈

## 변경된 점

`examples/sqs-sns`는 bluetape service-client wrapper를 도입하지 않고 SQS와 SNS를
example-only track으로 유지한다. copyable example은 AWS SDK for Go v2 client를 직접
사용하고, `testcontainers/floci`는 local integration test에서만 의존한다.

## 유지할 규칙

- follow-up issue가 반복되는 Go-specific boilerplate를 증명하지 않는 한 AWS service
  client는 caller-owned로 유지한다.
- emulator-backed example에는 opt-in Floci smoke test를 사용해 일반 `go test ./...`가
  stable하게 유지되게 한다.
- retry, visibility timeout, DLQ, SNS-to-SQS policy caveat는 broad wrapper 뒤에 숨기지
  말고 문서화한다.

## 검증 Reminder

향후 AWS example issue에서는 PR creation 전에 normal package test, opt-in Floci smoke,
race test, serial Docker-backed race smoke를 실행한다.
