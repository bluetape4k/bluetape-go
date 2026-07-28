# Redis Tiered Value Cache 교훈 (#535)

**Related issue:** #535

**Affected package:** `cache/redisvalue`

## L1: local reference cache와 serialized remote cache는 경계가 다르다

### 문제

모든 L1 write 전에 값을 serialize하거나 clone하면 local tier가 두 번째 serialization boundary가 된다.
pointer-valued caller는 refill 뒤 다른 object를 받게 되고, 정상 L1 hit도 remote tier에만 필요한
비용을 치른다.

### 결정

`TieredCache[V]`는 독점 소유 L1에 `V`를 직접 저장한다. `serialization.Serializer[V]`를 호출하는 것은
`ValueCache[V]`뿐이다. pointer-valued `V`를 선택한 caller는 cache에 있는 동안 cached object를
immutable snapshot으로 다룬다.

### 증거

`TestTieredCacheSetPreservesReference`,
`TestTieredCacheHealthyL1SkipsRemoteAndSerializer`,
`TestTieredCacheL2HitStoresDecodedReference`,
`TestTieredCacheMixedStressRetiresState`, 그리고
`TestRedisValueIntegration/pointer-isolation`은 unit, stress, race, real-Redis 수준에서 이 경계를
증명한다.

### 향후 가드

향후 RESP3 작업은 `InvalidateLocal` 또는 `ClearLocal`만 호출한다. `Set`, `Delete`, `Clear`는 L2를
mutate하므로 invalidation event를 이 메서드들로 보내지 않는다.

## L2: redacted error에는 debug 및 structured-log contract가 필요하다

### 문제

`Error()`만 검토하면 wrapped cause가 의도적으로 `errors.Is`와 `errors.As`로 접근 가능하더라도 debug
formatting과 structured logging이 implicit behavior로 남는다.

### 결정

`CacheError`는 redacted `GoString` 및 `slog.LogValuer` contract를 구현한다. 테스트는 provider,
serializer, partial-clear, joined cleanup failure를 `%v`, `%+v`, `%#v`, structured value에 걸쳐
다룬다. outer local-blocked error가 cleanup failure와 join되어도 nested partial-clear progress는
계속 보인다.

### 향후 가드

raw provider cause를 보존하는 새 public error는 causal inspection과 별도로 ordinary, debug,
structured-log formatting을 테스트해야 한다.

## L3: green race run은 approved concurrency matrix를 대체하지 않는다

### 문제

초기 stress test는 race freedom과 cleanup을 증명했지만 spec의 모든 generation-fence 및 mutation-order
acceptance criterion을 증명하지는 않았다. 첫 Step 6-R stability lane이 이 증거 공백을 잡았다.

### 결정

deterministic latch test는 same-key mutation과 경쟁하는 delayed refill, `ClearLocal`, blocked reader와
token waiter, loader completion, namespace clear, admitted delete를 다룬다. repeated same-key wave는
wave당 loader 하나를 assert한다. real Redis test는 dispatch-time cancellation cleanup과 blocked-state
repair를 통한 provider failure를 다룬다.

### 향후 가드

final review 전에 모든 spec concurrency bullet을 named test에 매핑하고 exact side-effect total을
assert한다. `go test -race`는 supporting evidence이지 traceability를 대체하지 않는다.

## L4: admission과 publication에는 별도 fence proof가 필요하다

### 문제

loader 또는 provider callback 내부에서만 멈추면 in-flight cleanup은 증명하지만, side-effect ticket
발급 후 admitted loader, `SET`, `DEL` 호출 전 경계를 분리하지 못한다.

### 결정

deterministic local-state seam이 one-shot ticket을 발급하고 decorator를 해당 generation 밖으로 전환한 뒤,
이미 admitted된 side effect가 정확히 한 번 실행되지만 결과는 L1에 publish될 수 없음을 증명한다.

### 향후 가드

state machine이 admission과 effect execution을 분리하면 ticket과 이후 publication classification을
독립적으로 테스트한다. callback latch만으로는 두 경계를 모두 증명하지 못한다.

## L5: option 이름만으로는 operational budget이 설명되지 않는다

### 문제

timeout field와 ACL command를 나열하는 것만으로는 operator가 budget을 어떻게 산정할지, 어떤 Redis/TLS
baseline을 요구할지, blocked local tier를 어떻게 alert/recover할지 알 수 없다.

### 결정

두 package README는 invalidation 및 cleanup timeout을 해당 timeout이 덮어야 하는 work에 연결한다. TLS
사용 시 verified TLS certificate가 있는 Redis 6+를 요구하고, logical database를 security boundary로
보지 않으며, blocked-state alert와 explicit recovery를 executable documentation parity contract의 일부로
둔다.

### 향후 가드

operational documentation test는 option 및 command 이름뿐 아니라 decision rule과 recovery action을
보존해야 한다.

## L6: public example은 failure policy를 모델링해야 한다

### 문제

compile-checked example도 mutation 또는 repair error를 버리거나 ordinary client identity로 namespace
clear를 보여 주면 unsafe behavior를 가르칠 수 있다.

### 결정

example은 모든 mutation 및 invalidation result를 확인하고, fresh bounded context로만 blocked state를
repair하며, 별도 credential의 admin client로 namespace clear를 구성한다. migration guidance도
`ValueCache` 채택은 incremental하며 `redisfory` data를 rewrite하지 않는다고 명시한다.

### 향후 가드

example은 단순히 compile되는 syntax가 아니라 caller code로 review한다. error handling, credential,
recovery context, migration boundary가 production guidance와 맞아야 한다.

example의 별도 clear-admin client는 credential input도 이름으로 드러낸다. 명시적 identity가 없는 두 번째
client instance는 ACL separation을 증명하지 않는다.

## L7: bounded read에도 하나의 consistency point가 필요하다

### 문제

`GETRANGE`는 bounded payload admission을 제공했지만, empty result에서는 missing key와 valid empty payload를
구분하기 위해 나중에 `EXISTS`가 필요했다. 다른 client가 두 command 사이에 key를 만들거나 삭제하면 서로
다른 Redis 시점의 bytes와 existence가 결합되어 empty cache hit가 조작될 수 있다.

### 결정

non-empty payload에는 single-command path를 유지한다. 첫 bounded read가 empty이면 bounded `GETRANGE`와
`EXISTS`를 하나의 `MULTI`/`EXEC` transaction 안에서 다시 실행한다. 따라서 ordinary Redis identity에는
transaction command가 포함되며, deterministic two-client integration test가 결함을 드러낸 정확한
interleaving을 고정한다.

### 향후 가드

absence와 empty value가 모두 의미 있으면 서로 다른 backend snapshot의 payload probe와 existence probe를
결합하지 않는다. 하나의 atomic transaction/script 또는 의도적으로 versioned non-empty envelope을 사용하고,
원래 probe 사이의 cross-client mutation을 테스트한다.
