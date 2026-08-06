# Issue #62 S3 Example 교훈

## 변경된 점

`examples/s3`는 bluetape S3 client wrapper를 도입하지 않고 S3를 example-only track으로
유지한다. copyable example은 AWS SDK for Go v2 client를 직접 사용하고,
`testcontainers/floci`는 local integration test에서만 의존한다.

## 유지할 규칙

- follow-up issue가 반복되는 Go-specific boilerplate를 증명하지 않는 한 AWS service
  client는 caller-owned로 유지한다.
- emulator-backed example에는 opt-in Floci smoke test를 사용해 일반 `go test ./...`가
  stable하게 유지되게 한다.
- concrete consumer가 key policy, envelope metadata, compatibility behavior를 필요로
  하지 않는 한 KMS/client-side encryption은 deferred로 문서화한다.

## 검증 Reminder

향후 AWS example issue에서는 PR creation 전에 normal package test, opt-in Floci smoke,
race test, serial Docker-backed race smoke를 실행한다.
