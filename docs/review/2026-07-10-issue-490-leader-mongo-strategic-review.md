# Issue #490 Mongo Strategic Elector Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10

Scope:

- `leader/mongo/strategic.go`
- `leader/mongo/strategic_test.go`
- `leader/mongo/README.md`
- `leader/mongo/README.ko.md`
- top-level and `leader` package README references

## 증거

- Redis strategic elector contract reviewed for API parity and behavior shape.
- Existing Mongo single/group elector storage, index, retry, and clock options
  reviewed for local consistency.
- Mongo strategic implementation stores one candidate document per node and uses
  `group_key, lease_until` scans plus atomic `$inc` result updates.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Live scans use the existing `group_key, lease_until` index and bounded candidate sets; no background goroutine or polling loop was added. |
| Stability | Pass | Expired candidate pruning is explicit before strategy evaluation, and missing/expired result updates return `leader.ErrNotLeader`. |
| Security | Pass | No credential, auth, or tenant boundary was added; metadata is caller-owned candidate input and is copied on read/write. |
| Operator/Ops | Pass | Mongo client, collection, write concern, index creation, and cleanup remain caller-owned; README calls out clock-skew and strategy consistency assumptions. |
| Developer/API | Pass | Public API follows the existing `leader.StrategicElector` contract and reuses `leader.CandidateInfo`, strategies, and `CandidateResult` without package-specific wrappers. |
| User/Caller | Pass | Bilingual docs now show `NewStrategic`, storage fields, cleanup behavior, and test commands. |
| Integration | Pass | The existing single/group elector API remains unchanged; strategic support is additive inside `leader/mongo`. |

## 발견 사항

- P0: 0
- P1: 0

## 잔여 위험

The strategic elector relies on process clocks for candidate leases, matching
the existing Mongo leader behavior. Production deployments should keep clocks
synchronized and use write concern appropriate for their topology.
