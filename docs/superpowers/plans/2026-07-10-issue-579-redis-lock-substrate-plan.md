# Redis lock substrate migration 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** Reuse the 공유 Redis substrate in `lock/redis` 변경하지 않고 its 공개 API, stored key bytes, 또는 legacy custom-token behavior.

**아키텍처:** `Mutex.TryLock` uses `btredis.NewOwnerToken` 만 for default-generated ownership 및 records a compatible `btredis.Lease` when the selected token is canonical. `Lease.Unlock` dispatches `btredis.CompareAndDelete` for that canonical lease; legacy custom tokens retain the 기존 Lua command but convert provider failures to a redacted `btredis.OpError`. TTL validation remains local because the legacy 패키지 permits any positive duration, while the 공유 helper requires at least one millisecond.

**기술 스택:** Go 1.26, `github.com/redis/go-redis/v9`, 기존 Redis Testcontainers fixture, `github.com/bluetape4k/bluetape-go/redis`.

---

## 파일 지도

| 파일 | 책임 |
|---|---|
| `lock/redis/mutex.go` | Select generated/공유 versus custom compatibility ownership path 및 redact acquire/unlock provider 오류. |
| `lock/redis/mutex_test.go` | 고정 계약 regression cases using one serial Testcontainers Redis instance per 테스트. |
| `lock/redis/README.md` | Explain 오류-원인 preservation 및 redacted Redis 진단. |
| `lock/redis/README.ko.md` | 한국어 parity for the same operational guarantee. |

### 작업 1: 고정 Contract Regression Tests

**complexity:** 보통

**파일:**
- Modify: `lock/redis/mutex_test.go`

- [ ] **단계 1: Write failing generated-token substrate coverage**

추가 a 테스트 that acquires 함께 an empty `Options.Token`, parses the 공개
`Lease.Token()` through the 공유 패키지, 및 unlocks normally:

```go
func TestMutexGeneratedTokenUsesSharedOwnerToken(t *testing.T) {
    ctx := context.Background()
    client := redisClient(ctx, t)
    mutex := newMutex(t, client, testLockKey(t), "", time.Second)

    lease, err := mutex.TryLock(ctx)
    if err != nil {
        t.Fatalf("try lock: %v", err)
    }
    if _, err := btredis.ParseOwnerToken(lease.Token()); err != nil {
        t.Fatalf("generated token should be canonical: %v", err)
    }
    released, err := lease.Unlock(ctx)
    if err != nil || !released {
        t.Fatalf("unlock generated lease: released=%t err=%v", released, err)
    }
}
```

- [ ] **단계 2: 실행 the new 테스트 전에 implementation**

실행: `go test -p 1 -count=1 ./lock/redis -run TestMutexGeneratedTokenUsesSharedOwnerToken`

예상: FAIL because the legacy generator emits a 32-character token.

- [ ] **단계 3: Write failing custom-token compatibility coverage**

추가 a 테스트 함께 `Token: " owner-a "` that asserts the 기존 normalized
`lease.Token() == "owner-a"`, reads the same Redis value, 및 performs a
successful unlock. This protects the legacy non-canonical script path.

- [ ] **단계 4: 추가 redacted provider-오류 coverage**

사용 a closed `*redis.Client`, a key containing a unique secret marker, 및 a
custom token containing a second marker. 검증 `TryLock` 및 `Unlock` retain
`errors.Is(err, redis.ErrClosed)` while neither 오류 string contains either
marker.

- [ ] **단계 5: 실행 focused regression 테스트**

실행: `go test -p 1 -count=1 ./lock/redis -run 'GeneratedToken|CustomToken|Redacted'`

예상: generated-token 및 redaction 테스트 fail 전에 작업 2; the custom
token 테스트 passes against the current compatibility behavior.

### 작업 2: Minimal Shared-Substrate Adoption

**complexity:** 높음

**파일:**
- Modify: `lock/redis/mutex.go`

- [ ] **단계 1: 추가 the 공유 substrate import 및 lease field**

가져오기 `btredis "github.com/bluetape4k/bluetape-go/redis"` 및 add a private
optional `sharedLease *btredis.Lease` to `Lease`. 유지 `key` 및 `token` fields
및 모든 exported methods unchanged.

- [ ] **단계 2: 교체 만 default token generation**

교체 `randomToken()` 함께:

```go
ownerToken, err := btredis.NewOwnerToken()
if err != nil {
    return nil, fmt.Errorf("generate redis lock token: %w", err)
}
token = ownerToken.RedisValue()
```

After choosing either generated 또는 호출자 token, create the optional 공유
lease 만 for canonical values:

```go
var sharedLease *btredis.Lease
if ownerToken, err := btredis.ParseOwnerToken(token); err == nil {
    lease, err := btredis.NewLease(m.opts.key, ownerToken)
    if err != nil {
        return nil, err
    }
    sharedLease = &lease
}
```

다음을 하지 않는다: reject a non-canonical 호출자 token.

- [ ] **단계 3: Redact acquire provider failures**

Wrap a `SetNX` command failure as:

```go
return nil, btredis.NewOpError(
    btredis.OpLabels{Family: "lock", Operation: "acquire"},
    m.opts.key,
    err,
)
```

다음을 하지 않는다: wrap `ctx.Err()` 또는 `ErrNotAcquired`; their current 호출자-visible
identity remains unchanged.

- [ ] **단계 4: 라우팅 canonical unlocks through the 공유 helper**

After preserving the 기존 nil/canceled-context checks, use:

```go
if l.sharedLease != nil {
    return btredis.CompareAndDelete(ctx, l.mutex.client, *l.sharedLease, "lock")
}
```

For a custom non-canonical token, retain the private `unlockScript` execution
및 replace 만 its 오류 return 함께 `btredis.NewOpError` using family `lock`
및 operation `compare-delete`. Owner mismatch must still return `(false, nil)`.

- [ ] **단계 5: 보존 TTL compatibility explicitly**

Leave `options.normalize` validation as `o.TTL <= 0`. 다음을 하지 않는다: call
`btredis.ValidateTTL`: the 공유 패키지 rejects sub-millisecond durations 및
would change the current 공개 option 계약.

- [ ] **단계 6: Format 및 run focused 테스트**

실행:

```bash
gofmt -w lock/redis/mutex.go lock/redis/mutex_test.go
go test -p 1 -count=1 ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
```

예상: PASS. Redis Testcontainers execution remains serial (`-p 1`).

### 작업 3: Documentation And 검증

**complexity:** 낮음

**파일:**
- Modify: `lock/redis/README.md`
- Modify: `lock/redis/README.ko.md`

- [ ] **단계 1: 문서화 sanitized operational 오류**

추가 one behavior bullet in each locale: Redis command failures retain their
원인 for `errors.Is`/`errors.As`, while diagnostic messages redact raw keys
및 owner tokens. 다음을 하지 않는다: claim a new 공개 lock feature.

- [ ] **단계 2: 검증 source/document parity 및 full local 계약**

실행:

```bash
go test -p 1 -count=1 ./redis ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
go test -count=1 ./lock/redis -run Example
git diff --check
rg -n 'redact|redacted|오류|에러' lock/redis/README.md lock/redis/README.ko.md
```

- [ ] **단계 3: 실행 repository verification 전에 review**

실행: `make ci`

예상: PASS. No benchmark is run because this issue does 아님 alter an
algorithm 또는 provider throughput; issue #560 owns benchmark measurement,
table, chart, 및 analysis obligations.

### 작업 4: 리뷰, Lessons, And Publication

**complexity:** 보통

**파일:**
- 생성: `docs/review/2026-07-10-issue-579-redis-lock-substrate-review.md`
- 생성: `docs/lessons/2026-07-10-issue-579-redis-lock-substrate.md`

- [ ] **단계 1: 검증 implementation against this spec 및 plan**

Confirm each invariant is covered by an implementation assertion 및 fresh
테스트 evidence. 기록 any intentional 없음-op, especially local TTL validation.

- [ ] **단계 2: 실행 the mandatory six-perspective 7-Tier review**

리뷰 the `develop...HEAD` diff for 성능, 안정성, 보안,
운영자/Ops, 개발자/API, 및 사용자/호출자 concerns. Normalize findings 및
do 아님 publish 함께 P0/P1 findings. 기록 `P0=0 P1=0` in the review artifact.

- [ ] **단계 3: 커밋 함께 Lore trailers 및 create a PR closing #579**

사용 an 영문 intent-first commit message 함께 Constraint, Rejected,
Confidence, 범위-risk, Directive, Tested, 및 Not-tested trailers. The PR
body ends 함께 `## DoD Status` 및 includes the benchmark N/A rationale.

## 롤백

Revert the migration commit. The 공유 `redis` 패키지 remains independently
usable because it was introduced 및 merged in #578; reverting #579 restores
the original local token/script/오류 implementation without any data 또는 key
migration.
