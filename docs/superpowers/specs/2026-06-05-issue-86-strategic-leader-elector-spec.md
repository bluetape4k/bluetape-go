# Issue 86 Strategic Leader Elector Spec

Issue: #86
Milestone: 0.3.0

## Context

`leader.Elector`는 한 group에서 하나의 leader token을 얻는 lock-contention
모델이다. `leader.GroupElector`는 bounded slot 모델이다. #86은 후보들이 자신을
등록하고, 모든 노드가 동일한 후보 목록에 deterministic strategy를 적용해 같은
winner를 계산하는 candidate-list 모델을 추가한다.

## Goals

- `leader`에 candidate metadata, result, strategy, scorer, strategic elector
  계약을 추가한다.
- FIFO, seed-stable random, scored strategy를 제공한다.
- idle-time, success-rate, candidate-weight, weighted-composite scorer를 제공한다.
- `leader/redis`에 Redis-backed candidate registry와 `RunIfLeader`를 구현한다.
- unit, Testcontainers, stress, cancellation, example, README locale pair를
  포함한다.

## Non-goals

- 기존 `leader.Elector`/`leader.GroupElector` 동작 변경.
- 새 runtime dependency 추가.
- non-deterministic distributed random election.
- scheduler/heartbeat daemon/local registry.

## Public API

```go
type CandidateInfo struct {
    NodeID          string
    RegisteredAt    time.Time
    LastStartedAt   time.Time
    LastCompletedAt time.Time
    SuccessCount    int64
    FailureCount    int64
    Weight          float64
    Metadata        map[string]string
}

type CandidateResult int

const (
    CandidateSucceeded CandidateResult = iota + 1
    CandidateFailed
)

type ElectionStrategy interface {
    Elect(candidates []CandidateInfo) (CandidateInfo, bool)
}

type CandidateScorer interface {
    Score(candidate CandidateInfo, all []CandidateInfo) float64
}

type StrategicElector[T any] interface {
    RegisterCandidate(ctx context.Context, group string, info CandidateInfo, ttl time.Duration) error
    UnregisterCandidate(ctx context.Context, group string, nodeID string) error
    ListCandidates(ctx context.Context, group string) ([]CandidateInfo, error)
    UpdateResult(ctx context.Context, group string, nodeID string, result CandidateResult) error
    RunIfLeader(ctx context.Context, group string, strategy ElectionStrategy, action func(context.Context) (T, error)) (T, bool, error)
}
```

## Strategy Behavior

- Empty candidate list는 `(CandidateInfo{}, false)`를 반환한다.
- FIFO는 가장 이른 `RegisteredAt`, 그 다음 lexicographic `NodeID`를 선택한다.
- Random은 `NodeID`로 정렬한 후보 목록에 seed-stable pseudo-random index를 적용한다.
- Scored는 가장 높은 score를 선택하고 tie는 FIFO ordering으로 해소한다.
- Strategy는 caller-owned slice를 mutate하지 않는다.

## Scorer Behavior

- Idle time은 `LastCompletedAt`이 있으면 그 시점부터, 없으면 `RegisteredAt`부터 계산한다.
- Idle score는 후보군의 maximum idle duration 기준 0-100으로 normalize한다.
- Success-rate score는 history가 없으면 0, 있으면 `success/(success+failure)*100`이다.
- Candidate-weight scorer는 `CandidateInfo.Weight`를 반환한다.
- Weighted scorer는 scorer list가 비어 있거나 weight가 0 이하이면 생성 실패한다.

## Redis Behavior

- `NewStrategic[T]`는 nil client와 invalid `leader.Options`를 거부한다.
- `RegisterCandidate`는 blank group/nodeID와 non-positive TTL을 거부한다.
- `RegisterCandidate`는 zero `RegisteredAt`을 `time.Now().UTC()`로 채운다.
- `ListCandidates`는 expired/missing candidates를 pruning하고 `NodeID` 기준으로 반환한다.
- `UpdateResult`는 success/failure count와 `LastCompletedAt`을 갱신한다.
- `RunIfLeader`는 현재 member가 winner일 때만 action을 실행하고 결과를 기록한다.
- Backend/context error는 `errors.Is`가 가능하도록 wrap한다.

## Tests

- `leader`:
  - FIFO tie-break.
  - random seed determinism.
  - scored strategy와 scorer behavior.
  - weighted scorer validation.
  - candidate derived metrics.
- `leader/redis`:
  - register/list/unregister.
  - TTL expiry pruning.
  - result update.
  - leader action executes only on winner.
  - non-leader action is not executed.
  - `AsyncJobTester` cancellation.
  - `GoroutineStressTester` concurrent register/elect stress.

## Documentation

- `leader/README.md`, `leader/redis/README.md`.
- `README.md`, `README.ko.md`.
- scored idle-time election example.
- implementation lesson and local 7-tier review artifact.
