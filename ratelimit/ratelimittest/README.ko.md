# Rate Limit Conformance Test

## 계약

`Run`은 parent-independent neutral `Result`로 burst, refill, isolation,
cancellation, one-debit lost response, exact concurrent admission을 검증합니다.

## 사용법

Provider result를 field-by-field로 변환합니다. Gate와 failure injection은 실제 debit
boundary에 연결해야 하며 public-call wrapper gate만으로는 충분하지 않습니다.

## Commit-Unknown 복구

Indeterminate provider는 zero `Result`와 typed error를 반환합니다. 요청이 한 번 debit됐을
수 있으므로 자동 replay하지 않습니다. 보수적인 full-refill interval
(`Burst / RatePerSecond`)을 기다리거나 caller budget에서 한 번의 debit을 흡수합니다.

## 진단

Gate와 result 대기는 bounded입니다. Runner 실패는 stable case name만 보고하며 adapter
error, key, endpoint, provider response text를 출력하지 않습니다.

## 테스트

```bash
go test -race -count=1 ./ratelimit/ratelimittest
```
