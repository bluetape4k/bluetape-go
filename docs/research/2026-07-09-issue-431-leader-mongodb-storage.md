# Issue #431 MongoDB Leader Storage Evaluation

Issue: [#431](https://github.com/bluetape4k/bluetape-go/issues/431)  
Milestone: Backlog  
Date: 2026-07-09  
Decision: **single-elector first slice로 `leader/mongo`를 구현한다**

## 결정

이 research branch가 land된 뒤에만 MongoDB-backed `leader.Elector` package를 추가한다. 첫 implementation issue는
`leader.Elector`용 single-leader campaign, renewal, observation, owner-token release를 지원해야 한다.

첫 MongoDB slice에서는 `GroupElector` 또는 `StrategicElector`를 ship하지 않는다. 둘 다 valid follow-up이지만 서로 다른 document
shape와 contention proof가 필요하다.

- `GroupElector`는 concurrent acquisition에서도 정확한 `MaxLeaders`를 보존하는 slot model이 필요하다.
- `StrategicElector`는 candidate registry, pruning policy, deterministic read model, strategy-specific failover test가 필요하다.

## Current Repo Evidence

| Evidence | Result |
|---|---|
| `leader/elector.go` | shared `Elector` contract는 `Campaign`, `Resign`, `IsLeader`, `Leader`를 이미 분리한다. |
| `leader/group.go` | `GroupElector`는 `MaxLeaders`, active count, slot availability를 추가하므로 single-document variant가 아니다. |
| `leader/options.go` | lease, renew interval, group, member ID, key prefix validation은 이미 backend-neutral이다. |
| `leader/README.md` / `.ko.md` | backend renewal failure는 `IsLeader`를 false로 만들어야 한다. |
| `leader/redis/README.md` | Redis는 single, group, strategic implementation을 이미 제공하지만 각각 다른 storage pattern을 사용한다. |
| `testcontainers/mongodb` | #430은 MongoDB integration-test fixture를 공유하면서 client, database, collection, index, test data는 caller-owned로 둔다. |
| `docs/lessons/2026-07-07-mongodb-testcontainer-fixture.md` | MongoDB helper는 client lifecycle 또는 caller-owned storage decision을 숨기면 안 된다. |

## 중요한 MongoDB Semantics

| Semantics | Design implication |
|---|---|
| `findOneAndUpdate`는 한 document를 atomic하게 filter/update한다. | single-elector acquisition 및 renewal에는 normalized leader key당 lease document 하나를 사용한다. |
| single-document write는 atomic이다. | active owner token과 `lease_until`을 같은 document에 저장한다. 첫 slice에서 ownership을 collection 여러 개로 나누지 않는다. |
| TTL index는 expired document를 asynchronous하게 제거한다. | TTL은 cleanup 전용이다. lease validity는 `lease_until <= now` 또는 `lease_until > now` 같은 query predicate에서 나온다. |
| `$currentDate`는 server-side timestamp를 설정할 수 있다. | `updated_at`에는 server-side timestamp를 우선한다. client clock skew를 수용하기 전에 server-side `lease_until` 계산용 aggregation-pipeline update를 평가한다. |
| write concern은 caller configuration이다. | production recommendation으로 `majority` write concern을 문서화하되 caller-provided collection을 조용히 mutate하지 않는다. |

Sources:

- MongoDB TTL indexes: <https://www.mongodb.com/docs/manual/core/index-ttl/>
- MongoDB single-document atomicity: <https://www.mongodb.com/docs/manual/core/write-operations-atomicity/>
- MongoDB `$currentDate`: <https://www.mongodb.com/docs/manual/reference/operator/update/currentDate/>
- MongoDB `findOneAndUpdate`: <https://www.mongodb.com/docs/manual/reference/method/db.collection.findOneAndUpdate/>
- MongoDB Go driver `FindOneAndUpdate`: <https://www.mongodb.com/docs/drivers/go/current/usage-examples/updateOne/#find-and-update>

## Proposed Single-Elector Shape

| Field | Purpose |
|---|---|
| `_id` or `key` | `<keyPrefix>:<group>` 같은 unique normalized leader key. |
| `group` | diagnostics용 human-readable group name. |
| `member_id` | `leader.Options`에서 온 caller member ID. |
| `token` | preferably `memberID:random`인 opaque owner token. `Leader`가 반환한다. |
| `lease_until` | acquire/read predicate가 사용하는 authoritative lease-expiry instant. |
| `created_at` / `updated_at` | diagnostics 및 cleanup support. |

첫 slice의 index:

- unique `_id` 또는 unique `{key: 1}`.
- cleanup용 optional TTL index on `lease_until` with `expireAfterSeconds: 0`.
- group 또는 strategic elector work 전에는 group/strategy index 없음.

## Operation Design

| Operation | Required behavior |
|---|---|
| `Campaign(ctx)` | context cancellation 또는 successful ownership까지 loop한다. expired ownership에 atomic conditional update를 시도하고, upsert duplicate-key race는 lost acquisition으로 처리한 뒤 기존 renewal interval/backoff 뒤 retry한다. |
| `Renew` | stored token이 이 elector의 token과 일치할 때만 update한다. zero-match renewal은 leadership loss이므로 renewal loop를 멈추고 `IsLeader`를 false로 만든다. |
| `Resign(ctx)` | token이 일치할 때만 delete 또는 clear한다. 다른 owner가 이미 document를 대체했다면 local leadership을 clear한 뒤 success를 반환한다. non-leader의 `Resign`은 idempotent다. |
| `Leader(ctx)` | `lease_until > now`인 document만 read하고 stored token을 반환한다. expired document는 TTL cleanup이 아직 없어도 no leader로 본다. |
| `IsLeader()` | local state만 반환한다. successful acquisition 뒤 true가 되고 resign, failed renewal, context-driven shutdown, observed owner loss 뒤 false가 된다. |

## Time And Clock Policy

implementation은 write timestamp에 MongoDB server time을 우선하고, Go driver로 가능하다면 `lease_until`도 server-side에서 계산한다.
첫 slice가 Go에서 `lease_until`을 계산한다면 contender 간 bounded clock skew가 필요하고 lease duration이 expected skew plus
operation latency보다 커야 한다고 문서화한다.

TTL monitor timing은 correctness에 참여하면 안 된다. TTL monitor는 lease document를 expiry보다 늦게 삭제할 수 있다. acquisition과
observation은 모든 active query가 `lease_until`을 비교하기 때문에만 correct하다.

## Cancellation And Lifecycle

- caller가 MongoDB client, database, collection, index, write concern configuration을 소유한다.
- `Campaign`과 `Resign`은 caller context를 존중한다.
- renewal loop는 `Resign`, lost ownership, context cancellation, ownership proof를 막는 backend error에서 멈춰야 한다.
- cleanup context는 request context cancellation 뒤 local teardown에 한해 bounded `context.WithoutCancel`을 사용할 수 있다.
  hidden global goroutine 또는 client는 없다.

## Follow-Up Scope

follow-up implementation issue:

- [#485](https://github.com/bluetape4k/bluetape-go/issues/485)
  `feat: Add MongoDB single leader elector backend`

#485 scope:

- `leader/mongo` single `leader.Elector`.
- caller-owned `*mongo.Collection`을 받는 option.
- owner-token acquire, renew, release, read predicate.
- TTL cleanup index documentation 또는 helper.
- `testcontainers/mongodb` integration test.
- 한 번에 local leader 하나만 있음을 증명하는 contention/race test.

defer:

- MongoDB `GroupElector`.
- MongoDB `StrategicElector`.
- transaction-backed 또는 multi-document design.
- JVM wire/document compatibility.
- generic distributed-lock package.

## Verification Plan For Implementation

future implementation PR은 다음을 포함해야 한다.

- `go test -count=1 ./leader ./leader/mongo`
- `go test -race -count=1 ./leader ./leader/mongo`
- `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`
- multiple contender와 short lease를 가진 contention test.
- failed renewal이 `IsLeader`를 false로 바꾸는 test.
- takeover에 TTL cleanup이 필요 없음을 증명하는 expired-document test.

## Outcome

이 research note, README pointer, review artifact, durable wiki preservation이 land되면 #431을 닫는다. follow-up issue #485는
single-elector MongoDB backend만 구현하고 이 문서를 acceptance boundary로 사용한다.
