# Leader Conformance Test

## 계약

`Run`은 모든 provider에 하나의 필수 single-elector contract를 적용합니다.
Factory와 control은 필수이며 capability skip은 없습니다.

## 사용법

Caller-owned backend fixture를 사용하고 하나의 `leader.Options` identity를 normalize한
뒤 `Harness.New`에서 provider elector를 생성합니다. `Control`에는 deterministic owner
probe, operation count, replacement, post-linearization failure injection만 노출합니다.

## Commit-Unknown 복구

Provider는 dispatch된 실패를 typed `leader.OperationError`로 반환해야 합니다. Commit을
확인할 수 없으면 `leader.ErrCommitUnknown`도 함께 match합니다. Caller는 type-first로
처리하고 bounded `Resign`, lease TTL fallback 후에 새 campaign을 시작합니다.

## 진단

Runner 실패는 stable case name만 보고하며 adapter error, owner value, key, endpoint,
token을 출력하지 않습니다. Case가 실패하면 provider-local log를 별도로 확인하세요.

## 테스트

```bash
go test -race -count=1 ./leader/leadertest
```
