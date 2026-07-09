# Issue #489 MongoDB Group Leader Review

Issue: [#489](https://github.com/bluetape4k/bluetape-go/issues/489)
Date: 2026-07-09
Scope: `leader/mongo` `leader.GroupElector` backend

## Evidence

- `leader/mongo/group.go` implements `leader.GroupElector` with one MongoDB
  lease document per bounded group slot.
- `leader/mongo/elector_test.go` covers max-leader admission, failed extra
  contender, resign/reclaim, expired takeover before TTL cleanup, renewal loss,
  and concurrent stress bounds.
- `leader/mongo/README.md` and `.ko.md` document slot document shape,
  `MaxLeaders` guarantee, caller-owned MongoDB resources, write concern, TTL
  cleanup, and bounded clock-skew assumptions.
- `EnsureIndexes` now creates the cleanup TTL index plus a `group_key,
  lease_until` index for active slot counting.

## 7-Tier Review

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | Active count uses an indexed `group_key, lease_until` query; campaign scans at most `MaxLeaders` slots per retry. |
| Stability | PASS | Resign cancels renewal and waits for worker shutdown; failed renewal clears local ownership. |
| Security | PASS | Owner-token predicates protect renew and resign from deleting or extending another owner. |
| Operator/Ops | PASS | README states caller-owned collection/write concern and that TTL is cleanup only. |
| Developer/API | PASS | No shared `leader` API change; `NewGroup` mirrors existing Redis group construction. |
| User/Caller | PASS | Tests prove exact `MaxLeaders`, context cancellation, reclaim, and renewal-loss behavior. |
| Integration | PASS | P0=0 P1=0. Main-session integration accepts bounded slot documents as the minimal MongoDB group model. |

## Validation

- `go test -count=1 ./leader ./leader/mongo`
- `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`
- `go test -race -count=1 ./leader ./leader/mongo`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `git diff --check`

## Residual Risk

Changing `MaxLeaders` downward while old higher-numbered slots are still active
can temporarily make `ActiveCount` exceed the new limit. `AvailableSlots` clamps
to zero, so new owners are not over-admitted while old slots drain.
