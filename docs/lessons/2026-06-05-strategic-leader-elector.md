# Strategic Leader Elector Lessons

Issue: #86
Milestone: 0.3.0

## Lesson

Redis-backed candidate statistic은 atomically update해야 한다. 여러 elected action
run이 동시에 끝날 때 read/modify/write sequence는 success 또는 failure count를
잃을 수 있으므로, result update는 candidate TTL을 보존하고 counter를 하나의 Redis
command에서 증가시키는 Lua script를 사용한다.

## Applied Guardrails

- register/list bookkeeping은 local process clock이 아니라 Redis server time을
  사용한다.
- candidate listing은 expired ZSET member와 missing candidate value를 prune한다.
- strategy implementation은 sorting 전에 input slice를 copy해 caller-owned candidate
  list를 mutate하지 않는다.
- random election은 seed-stable selection 전에 node ID로 sort해 caller input order가
  선택 결과에 영향을 주지 않게 한다.
- result counter에는 exact concurrent update regression test가 있다.

## Validation Evidence

- `go test -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress|ExampleNewStrategic'`
- `go test -race -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'`
