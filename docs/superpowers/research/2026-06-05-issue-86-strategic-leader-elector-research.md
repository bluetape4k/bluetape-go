# Issue 86 Strategic Leader Elector Research

Issue: #86
Milestone: 0.3.0

## 질문

`bluetape-go`가 기존 `leader`/`leader/redis` 계약을 깨지 않고 pluggable
leader election strategy를 추가하려면 어떤 Go API와 Redis 저장 계약이 적절한가?

## 저장소 근거

| 근거 | 관찰 | 결정 |
|---|---|---|
| GitHub issue #86 | `CandidateInfo`, `ElectionStrategy`, FIFO/random/scored strategy, Redis registry, Testcontainers smoke, README update를 요구한다. | #86은 단일 feature PR로 처리한다. |
| `leader/elector.go` | 기존 single-leader API는 lock-contention style `Campaign`/`Resign` 계약이다. | 기존 `Elector` 계약은 변경하지 않는다. |
| `leader/group.go`, `leader/redis/group.go` | Redis group elector는 Go-owned key format, Lua, Redis server `TIME`, Testcontainers stress를 사용한다. | #86도 Go-owned Redis key와 기존 Redis 패턴을 따른다. |
| `testing/concurrency` | 새 coordination 기능은 `GoroutineStressTester`, `AsyncJobTester`를 명시적으로 써야 한다. | Redis strategic elector에 stress/cancellation test를 넣는다. |
| `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md` | 0.3.0의 가치는 coordination과 consistent behavior다. | generic storage wrapper가 아니라 전략 기반 coordination API에 집중한다. |

## Kotlin Reference

`bluetape4k-leader`의 참고 개념:

- `StrategicLeaderElector`: 후보 등록, 후보 목록 조회, 전략 적용, winner일 때만 action 실행.
- `CandidateInfo`: node id, registration time, last start/completion time,
  success/failure count, metadata.
- `ElectionStrategy`: 동일 후보 목록에서 deterministic winner를 계산한다.
- FIFO: `registeredAt`, `nodeId` 순서.
- Random: `nodeId` 정렬 후 seed-stable random 선택.
- Scored: 가장 높은 score, tie는 FIFO ordering.
- Scorers: idle time, success rate, weighted composition.

## Go 방향

- Public API는 `leader` package에 둔다.
- Kotlin wire compatibility는 목표가 아니다.
- 전략은 `[]CandidateInfo`에서 `(CandidateInfo, bool)`을 반환한다.
- `RunIfLeader`는 Go generic으로 action result type을 보존한다.
- Random strategy는 distributed split-brain 방지를 위해 seed-stable variant만 제공한다.
- Redis backend는 candidate JSON key와 live index ZSET 조합을 사용한다.

## Redis 계약

- Candidate key: `bluetape:leader-strategy:<group>:candidates:<nodeID>`
- Live index: `bluetape:leader-strategy:<group>:index`
- Register:
  - Redis `TIME` 기준 expiry score를 계산한다.
  - candidate JSON을 PX TTL로 저장한다.
  - index ZSET에 `nodeID`와 expiry score를 기록한다.
- List:
  - Redis `TIME`으로 expired index member를 제거한다.
  - missing candidate key를 index에서 제거한다.
  - candidates를 `NodeID` 기준으로 정렬해서 반환한다.
- UpdateResult:
  - 기존 candidate JSON을 읽고 success/failure count와 completion time을 갱신한다.
  - candidate가 없으면 `leader.ErrNotLeader`로 판별 가능해야 한다.

## Non-goals

- Kotlin/JVM Redis interop.
- local in-memory strategic elector.
- random seed generation protocol.
- scheduler, heartbeat daemon, background registry refresher.

## 검증 후보

```bash
go test -count=1 ./leader
go test -count=1 ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'
go test -race -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'
make ci
git diff --check
```
