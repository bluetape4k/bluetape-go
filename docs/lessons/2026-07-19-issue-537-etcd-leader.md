# etcd Leader Election 교훈 (#537)

이 노트는 milestone `v0.19.0`에서 제공한 etcd-backed `leader.Elector` provider의 재사용 가능한 구현 및
검증 교훈을 기록한다.

## L1: official `Campaign` cleanup은 client context에 묶인다

### 문제

caller context를 취소해도 `concurrency.Election.Campaign`이 반환됐다는 보장은 없다. official
implementation은 shared etcd client context로 발행한 transaction을 통해 candidate를 아직 제거하고 있을 수
있다. 따라서 campaign만 취소하는 supervisor는 shutdown이 끝났다고 믿는 동안 blocked goroutine을 leak할 수
있다.

### 결정

caller context는 admission 및 waiting boundary로, caller-owned etcd client는 최종 coordinated hard-stop
boundary로 다룬다. 정상 shutdown은 campaign을 취소하고 protected work를 join한 뒤 bounded `Resign`을
시도한다. official campaign이 여전히 반환되지 않으면 supervisor가 shared client를 닫고 모든 campaign을
join하며 unresolved cleanup inventory를 보존한다. restart 전에는 별도 healthy client로 정확한 candidate
range가 비었음을 증명한다.

### 증거

`TestBlockedOfficialCampaignCleanupRequiresClientHardStop`은 cleanup transaction을 가로채 cancellation만으로는
campaign이 join되지 않음을 증명하고, client close가 이를 release함을 증명한다. provider는 별도
linearizable read가 정확한 candidate key absence를 증명할 때까지 `ErrCommitUnknown`과 generation inventory를
보존한다. `leader/etcd/example_test.go`의 shutdown example도 같은 순서를 보여 준다.

### 향후 가드

client hard-stop 및 exact-range reconciliation sequence를 caller-context timeout으로 대체하지 않는다. upstream
`Campaign` cleanup context는 provider public API는 아니지만 operational dependency이므로 etcd client upgrade
때마다 hostile cleanup test를 다시 실행해야 한다.

## L2: server-granted TTL이 cleanup budget authority다

### 문제

requested lease duration은 etcd가 grant한 lease duration의 증거가 아니다. request 값을 expiry 또는
reconciliation authority로 쓰면 cleanup이 너무 빠르거나 늦을 수 있고, candidate key가 사라졌다는 점도
증명하지 못한다.

### 결정

requested duration을 `Grant`용 integer seconds로 round하고 server response를 validate한 뒤, granted TTL을
`EffectiveTTL`로 publish한다. 이 값은 reconciliation 및 operational deadline scheduling에만 쓴다. cleanup
state는 successful revoke 또는 linearizable exact-generation absence proof 뒤에만 clear한다. elapsed TTL 자체는
충분한 증거가 아니다.

### 증거

`TestEffectiveTTLTransitionsAndRejectsInvalidGrant`는 requested, unpublished, active, last-published transition을
검증한다. campaign test는 invalid grant를 거부하고 operation budget에 granted TTL을 쓴다. public package
README와 backend-neutral `leader.Elector` documentation은 timer와 cleanup proof의 구분을 명시적으로 보존한다.

### 향후 가드

`leader.Options.Lease`, local wall-clock time, missed keepalive 하나에서 expiry를 추론하지 않는다. effective
TTL을 노출하는 future provider는 그 값이 requested인지 server-authoritative인지 문서화하고 ownership loss proof를
scheduling과 분리해야 한다.

## L3: session, exact-key watch, `Proclaim`은 서로 다른 사실을 증명한다

### 문제

etcd session이 살아 있다는 것은 lease keepalive health만 증명한다. 정확한 election candidate key가 아직
존재하거나 같은 generation을 나타낸다는 증거는 아니다. 마찬가지로 `Campaign` 승리는 provider-specific token이
observer에게 durably published됐다는 증거가 아니다.

### 결정

official `Session`은 lease maintenance에, campaign header revision에서 시작하는 exact candidate-key watch는
generation loss에, bounded `Proclaim`은 leadership 노출 전 provider token publish에 사용한다. watch 생성과
publication 성공 뒤에만 leadership을 visible하게 한다. generation identity에는 key, lease, creation revision을
포함해 ABA replacement를 continued ownership으로 오해하지 않게 한다.

### 증거

`TestCampaignPublishesOnlyAfterWatchCreated`는 publication boundary를 고정하고,
`TestCampaignBoundsProclaim`은 bounded token update를 고정한다. monitor test는 watch cancellation과 compaction을
다루며, `TestCleanupReconciliationRejectsABA`는 identity text가 재사용되더라도 creation-revision 변경을 replacement로
처리함을 증명한다.

### 향후 가드

session health, exact ownership, observer publication을 하나의 boolean이나 RPC result로 합치지 않는다. election
observation 변경은 세 independent signal과 generation tuple을 보존해야 한다.

## L4: provider timing profile은 case를 보존하고 timed-out work를 격리해야 한다

### 문제

shared conformance suite는 real server에서 비현실적으로 느려지거나, timed-out case가 provider goroutine을 남길 때
unsafe해질 수 있다. timeout을 늘리면 containment bug가 숨고, abort path 없이 줄이면 cross-case interference가 생긴다.

### 결정

필수 `leadertest` 15개 case를 모두 유지하고 etcd-specific timing profile을 제공한다: 3-second lease,
1-second renewal interval, 12-second case timeout, 4-second wait timeout, 2-second resign timeout. 모든 case에는
dedicated client를 주고, timed-out work가 다음 case 전에 join되도록 client close를 bounded abort callback으로 쓴다.

### 증거

`TestEtcdElectorConformance`는 profile과 per-case client registry를 설정한다. full provider test는 28.24초에 완료됐고,
`leader/leadertest`와 `leader/etcd` race run은 43.48초에 완료됐다. lost response, renewal failure, owner loss,
stale resign, exact contention, redaction을 포함한 15개 case가 모두 통과했다.

### 향후 가드

slow case를 삭제하거나 faulted client를 case 간 공유하거나 한 backend를 위해 global timing default를 올리지 않는다.
provider profile을 조정하고 abort containment를 유지하며, profile 변경 시 normal 및 race wall-clock evidence를 보고한다.

## L5: lease authorization은 lease creation이 아니라 attached key를 따른다

### 문제

etcd lease ID는 creator-owned capability도 prefix-owned resource도 아니다. 누가 `Grant`를 호출했는지 또는 lease RPC
하나에서 isolation을 추론하면 authenticated principal 사이의 경계를 과장할 수 있다.

### 결정

authenticated test로 정확한 server behavior를 고정한다. v3.6.13에서는 다른 principal이 unattached lease를 revoke할 수
있지만, attached-key authorization은 principal이 attached candidate key를 쓸 수 없을 때 cross-principal `Revoke`와
`KeepAliveOnce`를 모두 거부한다. same-range principal은 서로 trusted로 보고, mutually untrusted tenant는 별도 cluster에
둔다. pinned check는 election safety evidence이지 일반 tenant isolation 증거가 아니다.

### 증거

`TestEtcdAuthorizationBoundaries`는 immutable v3.6.13 fixture에 대해 symmetric own-range access, sibling-range denial,
unattached revoke, attached cross-principal revoke 및 keepalive denial을 증명한다.

### 향후 가드

모든 etcd server upgrade마다 attached-lease denial check 두 개를 다시 실행한다. 이를 source inspection만으로 대체하거나
lease ID를 security capability로 홍보하지 않는다.
