# Issue #411 Redis HyperLogLog 교훈

## Lesson

Redis HyperLogLog는 넓은 probabilistic 명령 facade가 아니라 좁은 core Redis
wrapper로 두는 편이 가장 안전하다. HLL을 `PFADD`, `PFCOUNT`, `PFMERGE`에
한정하면 RedisBloom module 가정 없이 기존 Redis Testcontainers fixture로 동작을
증명할 수 있다.

## Evidence

- #410은 HLL을 먼저 선택했고 `CF*` runtime 지원이 명확해질 때까지 Cuckoo를
  미뤘다.
- 구현은 `probabilistic.Hasher` 출력의 SHA-256 hex digest를 저장해 Redis 명령
  payload에 caller raw value가 들어가지 않게 했다.
- `GoroutineStressTester`와 `AsyncJobTester`가 동시 HLL 호출과 cancellation
  동작을 덮는다.
- 첫 full test/race gate가 실수로 병렬 시작되었기 때문에, 신뢰 가능한 증거를 위해
  변경된 Testcontainers package를 순차로 다시 실행했다.

## Future Rule

Redis probabilistic 추가 작업은 구조별 contract를 분리한다. Bloom은 false
positive가 있는 membership, HLL은 approximate cardinality, Cuckoo는 RedisBloom
`CF*` fixture/runtime 가정이 문서화된 뒤에만 다룬다.
