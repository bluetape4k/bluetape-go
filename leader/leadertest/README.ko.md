# Leader Conformance Test

## 계약

`Run`은 모든 provider에 하나의 필수 single-elector contract를 적용합니다.
Factory와 control은 필수이며 capability skip은 없습니다.

## 사용법

Caller-owned backend fixture를 사용하고 하나의 `leader.Options` identity를 normalize한
뒤 `Harness.New`에서 provider elector를 생성합니다. `Control`에는 deterministic owner
probe, operation count, replacement, post-linearization failure injection만 노출합니다.

```go
func TestProviderConformance(t *testing.T) {
    leadertest.Run(t, providerHarness(t))
}
```

## Timeout Containment

Provider에 hard-stop callback이 필요하면 `RunWithConfig`를 사용합니다. `CaseTimeout`은
cancellation과 containment를 시작할 뿐 최종 join을 제한하지 않습니다. `Abort`는 provider
work를 unblock해야 하며, cancellation을 무시하고 join되지 않는 provider가 process를
fail-stop하도록 test command에 outer `go test -timeout`을 지정합니다. Compile-checked
형태는 `ExampleRunWithConfig`를 참고합니다.

## Commit-Unknown 복구

Provider는 dispatch된 실패를 typed `leader.OperationError`로 반환해야 합니다. Commit을
확인할 수 없으면 `leader.ErrCommitUnknown`도 함께 match합니다. Caller는 type-first로
처리하고 bounded `Resign`, lease TTL fallback 후에 새 campaign을 시작합니다.

## 진단

Runner 실패는 stable case name과 `context`, `state`, `provider`, `contract` 같은 safe reason
category를 보고합니다. Adapter error, owner value, key, endpoint, token은 출력하지 않으며
provider-local log를 별도로 확인합니다.

## 테스트

```bash
go test -race -count=1 -timeout=10m ./leader/leadertest
```
