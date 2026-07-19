# Issue #537 etcd Leader Step 3-P Risk Prediction

Date: 2026-07-19 KST

Issue: [#537](https://github.com/bluetape4k/bluetape-go/issues/537)

Approved design: `docs/superpowers/specs/2026-07-19-issue-537-etcd-leader-design.md`

Approved plan: `docs/superpowers/plans/2026-07-19-issue-537-etcd-leader-plan.md`

## Baseline Evidence

| Check | Result |
|---|---|
| Go / platform | `go1.26.5`, `darwin/arm64` |
| Testcontainers module | `github.com/testcontainers/testcontainers-go v0.42.0` |
| Module download | `go mod download` exit `0` |
| Artifact diff | `git diff --check origin/develop...HEAD` exit `0` |
| Worktree | Temporary workflow evidence files가 생기기 전 clean 상태였고, `leader/etcd` source 또는 dependency 변경은 없었다. |
| Full CI | `/usr/bin/time -p make ci` exit `0`; `real 534.57`, `user 82.48`, `sys 26.68` |
| Docker | `docker info --format ...` exit `0`; `server=28.4.0 os=linux arch=aarch64`. Task 6 fixture 실행 전에 readiness와 target platform을 다시 증명한다. |

Pre-change CI는 현재 25분 job timeout 안에서 통과했다. 이 수치는 구현 후 회귀와 Docker-backed
admission budget을 비교하는 기준이며, Testcontainers fixture 자체의 준비 상태를 대신 증명하지 않는다.

## Risk Ledger

| Risk | Trigger | Signal | Prevention | Recovery | Owner |
|---|---|---|---|---|---|
| Integer-TTL drift | `leader.Options.Lease`가 sub-second 또는 정수 초 경계 밖이고 etcd가 요청과 다른 TTL을 grant한다. | server-granted TTL이 비정상적이거나 0 이하이고, 또는 실제 generation budget에 사용한 positive grant와 저장된 `EffectiveTTL`이 불일치한다. 요청값과 다른 정상적인 positive server grant 및 올림 결과 자체는 허용한다. | Task 2에서 항상 올림하는 overflow-safe `requestedTTL`과 transition/race tests를 두고, Task 3부터 server-granted TTL만 generation budget에 사용한다. | 비정상적이거나 0 이하이거나 저장/사용 값이 불일치하는 TTL은 publish하지 않고 generation을 cancel/join/revoke한다. proof가 없으면 cleanup inventory와 `ErrCommitUnknown`을 유지한다. | 구현: main session; 운용 판단: caller/operator |
| Campaign cleanup blocked on `client.Ctx` | caller context 취소 후 official `Election.Campaign`이 long-lived `client.Ctx()` 기반 Resign에서 멈춘다. | caller deadline 이후에도 Campaign call/Session handle이 join되지 않는다. | Detached wrapper를 금지하고 Task 1 fail-stop containment와 Task 7 blocked-Campaign hard-stop subprocess를 구현한다. | case root를 취소하고 join grace 후 해당 case의 모든 shared-client 사용자를 조정해 caller-owned client를 닫는다. 별도 healthy diagnostic client로 선형화 proof 전까지 cleanup inventory를 보존한다. | 라이브러리 containment: main session; client close 권한: caller/operator |
| Cancellation/publication race | Campaign 성공 직전 caller cancellation callback과 ownership publication이 동시에 실행된다. | canceled caller에게 success를 반환하거나 `published=true`와 cleanup ownership이 동시에 남는다. | Task 3에서 동일 mutex 아래 cancellation과 publication을 직렬화하고 callback stop/join 및 barrier test를 둔다. | cancellation winner면 publish를 금지하고 generation shutdown/revoke/reconciliation을 수행한다. publication winner면 callback을 join한 뒤 established leadership만 반환한다. | main session |
| Nil Session after Grant/NewSession failure | lease grant 후 `NewSession`이 실패하거나 Session을 만들기 전에 cleanup이 시작된다. | nil dereference, known lease 미회수, 닫히지 않는 `shutdownDone`이 나타난다. | Task 3 `shutdown`을 nil-session-safe `sync.Once`로 만들고 Grant/NewSession failure tests에서 lease inventory와 join을 검증한다. | generation을 cancel하고 known lease를 bounded revoke한다. proof 실패 시 exact-key reconciliation과 cleanup-pending retry로 전환한다. | main session |
| Monitor created before publication failure | watch/monitor handle을 만든 뒤 Proclaim snapshot, Created handshake, publication recheck가 실패한다. | Campaign이 실패했지만 monitor goroutine/ticker/Session이 살아 있거나 stale state를 변경한다. | Task 3에서 monitor handle을 generation-owned로 만들고 모든 pre-publication failure에 monitor join과 Session.Done close assertions를 둔다. | 동일 generation shutdown owner가 cancel/orphan/join하고 remote revoke/reconciliation proof 후에만 state를 지운다. | main session |
| Watch-created handshake timeout | exact-key watch가 server `Created` notification을 deadline 안에 보내지 않는다. | publication 전 Created wait timeout, watch cancel/error, 또는 준비되지 않은 monitor가 관찰된다. | Task 3에서 `WithCreatedNotify`와 bounded server acknowledgement를 publication 선행 조건으로 테스트한다. | publish하지 않고 watch를 cancel/join하며 Session shutdown과 revoke/reconciliation을 실행한다. | main session |
| Compaction | monitor watch revision이 compact되거나 watch response가 compaction을 보고한다. | `CompactRevision > 0`, canceled watch, revision-lost error가 발생한다. | Task 4에서 exact-key watch compaction을 terminal loss로 다루는 fake-watch tests, Task 7 interleaving tests를 둔다. | 즉시 `IsLeader`를 false로 하고 generation을 shutdown한다. exact absence가 증명되지 않으면 cleanup pending을 유지하고 선형화 reconciliation한다. | main session; compaction 정책 관찰: operator |
| Mismatched PUT | 동일 candidate key에 token, create revision 또는 lease가 다른 PUT이 관찰된다. | watch PUT의 key/token/createRev/lease tuple이 generation snapshot과 불일치한다. | Task 3 Created-then-mismatch 및 Task 4 exact tuple validation tests로 fail-closed 규칙을 고정한다. | local leadership을 즉시 지우고 Session/monitor를 join한다. replacement/absence proof 전까지 cleanup inventory를 보존한다. | main session |
| Overlapping key ranges | raw `KeyPrefix`/`Group` 결합이나 잘못된 range end로 sibling election을 함께 스캔한다. | slash 포함 입력 또는 encoded sibling의 `[root,end)`가 겹치거나 Leader 조회가 다른 group을 반환한다. | Task 2에서 두 segment를 `base64.RawURLEncoding`으로 인코딩하고 exact candidate root와 `GetPrefixRangeEnd` isolation tests를 둔다. | 영향 범위를 차단하고 key derivation을 수정한 뒤 constructor/unit 및 real-server sibling tests를 재실행한다. 이미 섞인 ownership은 operator가 별도 prefix로 migration한다. | 구현: main session; 기존 key migration: caller/operator |
| Proclaim overlap | renew RPC가 `RenewInterval`보다 느린데 ticker가 후속 Proclaim을 중첩/queue한다. | `maxInFlight > 1`, operation count가 cadence 상한을 초과하거나 Resign join이 지연된다. | Task 4 single monitor loop와 generation-derived bounded context를 사용하고 slow-Proclaim count/race tests로 `maxInFlight == 1`을 강제한다. | generation을 cancel해 in-flight RPC를 끝내고 monitor를 join한다. 실패는 ownership loss로 처리하며 새 renew를 시작하지 않는다. | main session |
| Stale monitor ABA | 이전 generation monitor가 늦게 종료되며 같은 elector의 새 generation state를 clear한다. | 새 Campaign 성공 직후 `IsLeader`가 false가 되거나 current generation ID가 과거 terminal event로 바뀐다. | Task 4에서 immutable generation ID와 locked current-generation comparison을 적용하고 stale-generation/rapid interleaving tests를 둔다. | stale event는 state mutation 없이 자기 generation만 shutdown/join한다. 영향이 확인되면 보호 작업을 중단하고 현재 owner를 다시 선형화 조회한다. | main session; 보호 작업 중단: caller |
| Nil official Resign without delete | `Election.Resign`이 nil을 반환하지만 compare miss 등으로 candidate key가 남는다. | nil Resign 뒤 exact-key Get에서 동일 token/lease/revision key가 존재한다. | Task 5에서 모든 dispatched Resign 결과 뒤 bounded revoke와 선형화 exact-key proof를 필수화하는 tests를 둔다. | proof 실패 시 `ErrCommitUnknown`과 cleanup inventory를 유지하고 같은 elector에서 Resign/reconcile을 재시도한다. | main session |
| Stale revision cleanup | retained create revision이 교체된 candidate 또는 새 generation의 revision과 다르다. | ResumeElection compare miss, exact Get이 동일 key지만 다른 createRev/token/lease를 반환한다. | Task 5 cleanup comparison에 key/createRev/token/lease를 포함하고 stale-revision 및 cleanup reconciliation ABA tests를 둔다. | exact replacement면 과거 generation을 안전하게 clear한다. 동일 owner 여부가 불명확하면 fail closed하고 inventory를 유지한다. | main session |
| Lease-level cross-principal revoke | RBAC principal이 다른 election key가 붙은 lease를 revoke하거나 권한 모델 변화로 격리가 깨진다. | B가 A candidate lease revoke에 성공하거나 authz 결과가 pinned v3.6.13 behavior와 달라진다. | Task 7 isolated authenticated fixture에서 symmetric range 권한, attached/unattached revoke를 검증하고 same-range principals를 mutual trust로 문서화한다. | behavior drift면 design review를 재개하고 separate cluster를 사용한다. 침해된 owner는 fail closed하고 모든 보호 작업을 중단한다. | 검증: main session; RBAC/cluster 격리: operator |
| Shared-client hard stop | blocked Campaign을 끊기 위해 client를 닫지만 다른 group/case 사용자도 같은 client를 공유한다. | 무관한 election 호출까지 실패하거나 client close 후 살아 있는 shared user가 발견된다. | Task 6에서 conformance case별 client registry를 사용하고, Task 7에서 close 전 모든 case-local 사용자를 조정하는 hard-stop test를 둔다. elector는 client를 자체 close하지 않는다. | caller/operator가 영향 사용자 전체를 quiesce한 뒤 client를 close/join하고 새 healthy client로 재구성한다. cleanup proof 전에는 restart gate를 열지 않는다. | client lifecycle: caller/operator; test containment: main session |
| Testcontainers leak | fixture startup/cleanup 부분 실패, client/container 미등록, serial rule 위반이 발생한다. | 테스트 종료 후 container/client가 남거나 lease/watcher baseline이 복구되지 않고 다음 test가 오염된다. | Task 6에서 `internal/testcleanup`, partial-create cleanup, immutable digest/platform, readiness를 사용하고 Docker-backed package를 serial 실행한다. | leaked resource를 bounded terminate하고 fixture를 새로 만든다. cleanup 실패는 test failure로 남기며 후속 Docker test를 신뢰하지 않는다. | main session; CI/Docker capacity: operator |
| Dependency graph churn | etcd client 또는 Testcontainers module 추가가 예상 밖 gRPC/protobuf/Prometheus/zap/`x/*` 버전을 바꾼다. | `go.mod`/`go.sum` diff가 선택 버전 범위를 넘어가거나 `go mod verify`/tidy-check가 실패한다. | Task 2는 etcd client v3.6.13만, Task 6는 import 생성 후 etcd module v0.42.0만 단계적으로 pin하고 각 단계에서 tidy/verify graph를 review한다. | 의도 밖 upgrade를 제거하고 최소 direct dependency set으로 tidy를 다시 실행한다. 필요한 graph 변경은 dependency review로 되돌린다. | main session |
| 32-contender leak | cancellation/resign/abort 후 contender Sessions, leases, watchers, monitors 또는 Proclaims가 남는다. | Task 7 baseline delta가 lease/session/watcher/monitor/proclaim 중 하나라도 0으로 돌아오지 않는다. | 32-contender resource test에서 시작 전 baseline, 최대 live count, teardown-to-zero를 deadline 내 polling하며 process goroutine count는 사용하지 않는다. | case roots 취소, Resign/reconcile, case client close를 수행하고 zero baseline까지 기다린다. 미복구면 subprocess/fixture를 fail-stop하고 원인을 보존한다. | main session; CI resource budget: operator |
| Rapid reacquisition without caller fencing | 이전 leader의 protected work가 join되기 전에 같은 group에서 새 Campaign이 성공한다. | 두 generation의 보호 작업이 겹치거나 old worker가 새 owner 이후 side effect를 낸다. | Task 7 rapid-reacquisition test에서 이전 protected work stop/join 또는 explicit test fencing generation을 강제하고, docs에서 elector token이 fencing token이 아님을 명시한다. | old work를 즉시 중단/join하고 외부 fenced resource의 generation/version으로 stale write를 거부한다. fencing이 없는 workload는 자동 재획득을 제한한다. | protected-work/fencing: caller; provider lifecycle test: main session |

## Implementation Gate

위 예방책은 승인된 Task 1~7의 RED tests에 먼저 고정한다. 구현 중 이 ledger의 recovery가
caller-owned client를 provider가 임의로 닫게 하거나, mandatory conformance case를 skip/완화하게
하거나, detached Campaign wrapper를 요구하면 source 변경을 중단하고 Step 2 design review로
돌아간다. Task 7까지의 real-server, race, auth, hard-stop, resource evidence가 통과하기 전에는
해당 risk를 해소된 것으로 간주하지 않는다.
