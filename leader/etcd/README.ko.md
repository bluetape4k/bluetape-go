# leader/etcd

[English](README.md) | [한국어](README.ko.md)

`leader/etcd`는 etcd v3 공식 `Session`과 `Election` primitive로 단일 leader
`leader.Elector` contract를 구현합니다. Production에서는 하나의 quorum에 연결된
caller-owned 인증/TLS client를 사용합니다.

## Preflight

Contender를 시작하기 전에 모든 endpoint가 예상한 member와 leader를 보고하는지
확인하고, bounded linearizable Put/Get/Delete roundtrip을 실행합니다. Production은
홀수 크기 quorum과 쓰기 가능한 majority를 사용하고 아래 RBAC range를 고정해야
합니다. Real-server suite의 검증 대상은 etcd `v3.6.13`이며, 다른 minor version은
별도 compatibility run이 필요합니다.

Repository의 Testcontainers fixture는 test-only plaintext 환경입니다. Platform별
immutable digest를 고정하지만 production 설정으로 복사해서는 안 됩니다.

## Import And Client Ownership

```go
import etcdleader "github.com/bluetape4k/bluetape-go/leader/etcd"
```

모든 elector는 `etcdleader.New`로 생성합니다. `Elector`의 zero value는 사용할 수
없습니다. `New`는 network I/O를 수행하지 않고 `*clientv3.Client`의 ownership도
가져가지 않습니다. Shared client는 모든 사용자와 campaign call이 join된 뒤 caller가
닫습니다. coordinated hard-stop exception은 blocked Campaign입니다. 다른 shared-client
사용자를 모두 drain한 뒤 client를 닫아 call을 깨우고, 아래 예제처럼 bounded join합니다.

Production client는 비어 있지 않은 root CA pool을 읽고 hostname을 검증할
`ServerName`을 지정하며 `InsecureSkipVerify=false`를 유지합니다. 또한 scope가 제한된
username/password 또는 동등하게 제한된 client certificate로 인증합니다.
Compile-checked client 구성은 `ExampleNew_productionTLS`를 참고합니다.

## Usage

```go
elector, err := etcdleader.New(client, leader.Options{
    Group:         "billing-workers",
    MemberID:      "worker-1",
    Lease:         30 * time.Second,
    RenewInterval: 10 * time.Second,
})
if err != nil {
    return err
}
```

`Campaign`은 synchronous call입니다. Nil을 반환했더라도 protected work를 시작하기
전에 `campaignCtx.Err()`를 확인하고 즉시 `IsLeader()`가 true인지 요구합니다. Work
중에는 매 작업 단위 전과 긴 작업에서는 최소 `min(RenewInterval, 1s)`마다 `IsLeader`를
확인합니다. 재획득 전에 이전 protected work를 중지하고 join해야 합니다. 전체
acquire/work/resign 흐름은 `ExampleNew`에 있습니다.

`New`는 caller-owned `*clientv3.Client`와 `leader.Options`만 받은 뒤 option을 normalize하고
validate합니다. Raw `concurrency.SessionOption`, session adoption, restart-resume API는
의도적으로 제공하지 않으며 Campaign마다 새 provider-owned session을 만듭니다. `Leader`는
opaque `<MemberID>:<random>` 값을 반환합니다. 전체 string으로만 비교하고 suffix를 fencing
token 또는 durable identity로 parsing하지 않습니다.

## Encoded Election Range

Provider는 `KeyPrefix`와 `Group`을 각각 unpadded URL Base64로 인코딩해
`/bluetape4k/leader` 아래에 배치합니다. Candidate는 해당 identity의 정확한
`[candidateRoot, rangeEnd)` 구간을 사용합니다. Encoding은 sibling group 간 delimiter
충돌을 막지만 `KeyPrefix`는 hostile tenant 격리가 아니라 collision 격리입니다. 이
format은 Go 전용이며 Kotlin/JVM leader participant와 호환되지 않습니다.

Candidate range 안의 key를 직접 Put/Delete하면 leadership을 강제로 잃게 할 수
있으므로, 이 권한은 상호 신뢰하는 operator와 election principal에게만 줍니다.

## Lease And Ownership Signals

요청한 lease는 정수 초 단위로 올림됩니다. etcd가 다른 server-granted TTL을 반환할 수
있으며, 현재 또는 가장 최근 fully published grant를 나타내는 `EffectiveTTL`만 retry
scheduling에 사용합니다.
`RenewInterval`은 최소 100 milliseconds이고 `Lease`보다 짧아야 하므로 published
leader 하나당 periodic `Proclaim` cadence는 10 Hz보다 빠르지 않습니다. 전체 capacity는
published leader group 수와 각 group의 configured `Proclaim` rate를 곱해 계획합니다.

Capacity inventory는 모든 group의 live contenders 상한도 포함해야 합니다. 각 contender는
예상 leases/sessions/candidate keys 하나씩을 사용합니다. Blocked contender마다 predecessor를
기다리는 campaign wait watch가 최대 하나 추가되고, published group마다 ownership
exact-key watches 중 하나가 추가됩니다. aggregate Proclaim QPS는 published group별
`1 / RenewInterval`의 합으로 계산하고, 전체 contender/watch/publication envelope를 대상
etcd cluster에서 검증합니다.

Ownership은 서로 다른 세 신호를 함께 사용합니다.

- 공식 `Session`이 lease keepalive를 소유하고 lease loss 때 종료됩니다.
- `RenewInterval`마다 bounded `Proclaim`이 candidate revision을 확인하며 renewal은
  겹치지 않습니다.
- Exact-key watch가 deletion, replacement, compaction, stream loss를 감지합니다.

어느 신호든 실패하면 local `IsLeader`를 즉시 false로 바꿉니다. Local state는 실행
guard일 뿐 fencing token이나 remote deletion proof가 아닙니다.

## Failure Recovery

`leader.OperationError`, `leader.ErrCommitUnknown`, `leader.ErrCleanupPending`는
`errors.Is`와 `errors.As`로 검사합니다. Diagnostic string은 key, endpoint, lease ID,
owner token을 redaction합니다. Error chain은 명시적인 `errors.Is`/`errors.As` 검사를 위해
raw etcd cause를 보존하지만, unwrapped 또는 joined raw cause를 sanitize하지 않고 log나
telemetry에 기록하지 않습니다.

Cancellation은 공식 `Election.Campaign`이 장시간 유지되는 caller client context로
cleanup하는 경로에 들어갈 수 있습니다. Client가 healthy할 때 같은 elector로 bounded
`Resign`을 재시도합니다. Cleanup이 계속 불확실하면 elector와 inventory를 유지합니다.
`EffectiveTTL` 대기는 다음 linearizable exact-key reconciliation 시점만 정할 뿐,
시간 경과 자체가 deletion을 증명하지 않습니다.
아직 publish되지 않은 진행 중 `Campaign`은 자체 cleanup을 소유하므로 concurrent
`Resign`은 no-op입니다. Supervisor는 resign 전에 campaign call을 cancel하고 join합니다.

## Shutdown And Reconciliation

Logical group마다 다음 순서를 지킵니다.

1. campaign context를 취소하고 새 protected work를 막습니다.
2. bounded join grace 동안 기다리고 protected work를 join합니다.
3. Call이 join되면 client가 healthy한 동안 같은 elector로 `Resign`을 재시도합니다.
4. Call이 계속 막히면 해당 service client의 모든 사용자를 조정하고 caller-owned
   client를 hard stop으로 닫은 뒤 call을 join합니다.
5. 해결하지 못한 generation을 저장하고 해당 process lane을 종료합니다.
6. 별도의 healthy diagnostic client가 exact range absence 또는 replacement를 증명한
   뒤에만 restart합니다.

관련 없는 사용자가 남아 있는 shared client를 닫지 않습니다. Timer만 보고 restart,
cutover, cleanup inventory 삭제를 결정하지 않습니다.

## RBAC And TLS

Principal마다 encoded `[candidateRoot, rangeEnd)`에만 read/write permission을 줍니다.
v3.6.13 test는 own-range Put/Get/Delete/Watch 성공과 sibling-range 거부를 양방향으로
증명합니다. 또한 unattached lease는 어떤 user든 revoke할 수 있지만, 허용 범위 밖
key가 붙은 lease는 revoke하거나 `KeepAliveOnce`할 수 없음을 확인합니다. 이는
creator-owned 또는 prefix-scoped lease capability가 아니라 attached-key authorization
검사입니다.

같은 range를 공유하는 principal은 서로 신뢰해야 합니다. Pinned 결과는
cross-principal election-lease interference를 줄이지만 일반적인 hostile-tenant isolation을
증명하지 않습니다. 상호 신뢰하지 않는 tenant는 별도 cluster에 두고, server version이
바뀔 때마다 cross-principal revoke와 keepalive denial test를 모두 재실행합니다. TLS는
발급 CA와 endpoint hostname을 모두 검증해야 합니다.

## Quorum, Compaction, And Fencing

Coordination availability에는 linearizable read와 쓰기 가능한 majority가 필요합니다.
Minority partition에서는 campaign하지 않습니다. Quorum을 복구하고 Status와
linearizable roundtrip을 확인한 뒤 unresolved candidate를 reconcile하고 contender를
재시작합니다.

Watch compaction이나 stream interruption은 fail closed로 처리합니다. API에는 fencing
token이 없으므로 stale leader가 etcd 연결을 잃은 뒤에도 외부 side effect를 계속할 수
있습니다. Safety-critical resource에는 별도의 fencing 또는 generation check가
필요합니다.

## Migration And Rollback

Cutover는 group별 stop-the-world입니다. Protected work를 멈추고 기존 provider를 drain한
뒤 safety boundary를 증명하고 etcd contender를 시작합니다. Rollback도 대칭입니다.
Protected work와 모든 etcd campaign을 중지하고, bounded same-elector cleanup 후 healthy
client로 exact candidate range가 비었음을 증명하고 zero etcd contenders를 확인한 뒤에만
이전 provider를 복원합니다. Provider overlap에는 외부 fencing authority가 필요합니다.

## Observability

Synchronous operation은 bounded-cardinality provider/operation/result/latency metric으로
감쌉니다. Work 전과 긴 work에서는 최소 `min(RenewInterval, 1s)`마다 `IsLeader`를
sampling하고 첫 true-to-false transition에 `leadership_lost` event 하나를 남깁니다.
Call boundary에서 `ErrCommitUnknown`과 `ErrCleanupPending`을 inventory하고, 후속 retry가
cleanup을 증명해도 failed cleanup attempt를 각각 기록합니다. Endpoint, key, lease ID,
owner token, rendered raw error는 label이나 log에 넣지 않습니다. 예제 recorder는
synchronous and non-blocking이어야 하며 입력을 sanitize한 뒤 기록하거나 bounded internal
queue로 넘깁니다.

## Tested Scope

Serial real-server suite는 `leader/leadertest` 15개 case 전체, 32-way contention,
cancellation, keepalive, external revoke, exact-key loss, watch interruption, restart,
post-success response loss, stale cleanup, authenticated range isolation, hard-stop join,
resource baseline 복귀를 검증합니다. Multi-node partition, TLS transport, cross-minor
compatibility를 검증했다고 주장하지 않습니다.

## Test

Docker-backed test는 직렬로 실행합니다.

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
```

배포, reconciliation, cutover, rollback, quorum recovery gate는 영·한
[v0.19.0 provider rollout runbook](../../docs/release/v0.19.0-provider-conformance-runbook.md)을
따릅니다.
