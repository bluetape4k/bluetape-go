# Strategic Leader Elector Lessons

Issue: #86
Milestone: 0.3.0

## Lesson

Redis-backed candidate statistics must be updated atomically. A read/modify/write
sequence can lose success or failure counts when multiple elected action runs
finish at the same time, so result updates use a Lua script that preserves the
candidate TTL and increments counters in one Redis command.

## Applied Guardrails

- Register/list bookkeeping uses Redis server time, not local process clocks.
- Candidate listing prunes expired ZSET members and missing candidate values.
- Strategy implementations copy input slices before sorting, so election does
  not mutate caller-owned candidate lists.
- Random election sorts by node ID before seed-stable selection, so caller input
  order does not affect the chosen candidate.
- Result counters have an exact concurrent update regression test.

## Validation Evidence

- `go test -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress|ExampleNewStrategic'`
- `go test -race -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'`
