# Lock Conformance Test

## 계약

`Run`은 provider-neutral function adapter로 immediate acquire/release, expiry,
cancellation, ownership, lost response, exact contention을 검증합니다.

## 사용법

`Harness.New`는 owner-bound `AcquireFunc`를 반환합니다. `Control`은 실제 mutation
boundary에 gate를 설치하고 하나의 mutation이 linearize된 뒤에만 실패를 주입해야 합니다.

## Commit-Unknown 복구

Acquire는 non-nil release callback과 typed provider error를 함께 반환할 수 있습니다.
즉시 cleanup하세요. Release 응답 유실은 false와 typed error를 반환하므로 같은 callback을
재시도합니다. Owner 비교가 replacement를 보호하며 TTL이 최종 fallback입니다.

## 진단

Gate와 result 대기는 bounded입니다. Runner 실패는 stable case name만 보고하며 adapter
error, owner value, key, endpoint, token을 출력하지 않습니다.

## 테스트

```bash
go test -race -count=1 ./lock/locktest
```
