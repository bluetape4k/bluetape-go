# Issue #490 Mongo Strategic Elector Review

Date: 2026-07-10

Scope:

- `leader/mongo/strategic.go`
- `leader/mongo/strategic_test.go`
- `leader/mongo/README.md`
- `leader/mongo/README.ko.md`
- top-level and `leader` package README references

## Evidence

- Redis strategic elector contract reviewed for API parity and behavior shape.
- Existing Mongo single/group elector storage, index, retry, and clock options
  reviewed for local consistency.
- Mongo strategic implementation stores one candidate document per node and uses
  `group_key, lease_until` scans plus atomic `$inc` result updates.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Live scans use the existing `group_key, lease_until` index and bounded candidate sets; no background goroutine or polling loop was added. |
| Stability | Pass | Expired candidate pruning is explicit before strategy evaluation, and missing/expired result updates return `leader.ErrNotLeader`. |
| Security | Pass | No credential, auth, or tenant boundary was added; metadata is caller-owned candidate input and is copied on read/write. |
| Operator/Ops | Pass | Mongo client, collection, write concern, index creation, and cleanup remain caller-owned; README calls out clock-skew and strategy consistency assumptions. |
| Developer/API | Pass | Public API follows the existing `leader.StrategicElector` contract and reuses `leader.CandidateInfo`, strategies, and `CandidateResult` without package-specific wrappers. |
| User/Caller | Pass | Bilingual docs now show `NewStrategic`, storage fields, cleanup behavior, and test commands. |
| Integration | Pass | The existing single/group elector API remains unchanged; strategic support is additive inside `leader/mongo`. |

## Findings

- P0: 0
- P1: 0

## Residual Risk

The strategic elector relies on process clocks for candidate leases, matching
the existing Mongo leader behavior. Production deployments should keep clocks
synchronized and use write concern appropriate for their topology.
