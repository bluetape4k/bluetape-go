# Issue 412 Redis Testcontainers Coverage 교훈

## Context

Issue #411은 Redis HyperLogLog 동작과 Testcontainers 기반 테스트를 추가했다.
하지만 issue #412에는 bounded context, serial Testcontainers 실행, local command
surface에 대한 명시적 coverage evidence가 여전히 필요했다.

## Lesson

follow-up issue가 이전 feature PR로 대부분 충족되었다면 구조를 복제하지 말고
증거로 남은 gap을 닫는다. Testcontainers 기반 Go package에서는 startup,
readiness, live operation, cleanup timeout을 코드에서 보이게 만들고,
package-local test와 race command를 두 README locale 파일에 모두 문서화한다.

## Applied

- 재사용 가능한 Redis test timeout 상수와 `redisTestContext(t)`를 추가했다.
- Bloom과 HyperLogLog 테스트의 unbounded Redis integration operation context를
  교체했다.
- Redis image, bounded context policy, coverage matrix, stress helper, serial
  execution guidance를 문서화했다.
