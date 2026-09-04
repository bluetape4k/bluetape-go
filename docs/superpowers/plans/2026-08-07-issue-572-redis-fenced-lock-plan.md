# Redis Fenced Lock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Issue #572의 redis/lock package가 Redis Lua 기반 owner-safe lease와 영속 fencing counter를 제공하도록 구현한다.

**Architecture:** FencedLock은 caller-owned redis.Cmdable과 KeyBuilder가 만든 동일 hash-tag owner/counter key를 보관한다. TryAcquire는 한 번만 시도하고 Acquire는 timer/select bounded backoff로 TryAcquire를 반복한다. Release는 shared btredis.Lease와 CompareAndDelete를 사용한다.

**Tech Stack:** Go 1.26, github.com/redis/go-redis/v9, github.com/bluetape4k/bluetape-go/redis, Redis 7 Testcontainers, Go race detector.

---

## 파일 지도

| 파일 | 책임 |
|---|---|
| redis/lock/doc.go | package documentation과 fencing/TTL 경계 |
| redis/lock/errors.go | ErrNotAcquired sentinel |
| redis/lock/options.go | Options와 client/key/TTL validation |
| redis/lock/keys.go | KeyBuilder owner/counter key layout |
| redis/lock/scripts.go | acquire Lua script와 결과 parser |
| redis/lock/lock.go | FencedLock, Lease, acquire/release, reconciliation |
| redis/lock/wait.go | bounded backoff timer |
| redis/lock/lock_test.go | unit validation, key-slot, parser, cancellation |
| redis/lock/integration_test.go | Testcontainers acquire/release/expiry/contention |
| redis/lock/example_test.go | fencing, cleanup timeout, over-TTL examples |
| redis/lock/README.md | English API와 safety caveat |
| redis/lock/README.ko.md | Korean parity |

## Task 1: 입력 검증과 key layout을 TDD로 고정

**Files:**
- Create: redis/lock/lock_test.go
- Create: redis/lock/errors.go
- Create: redis/lock/options.go
- Create: redis/lock/keys.go
- Create: redis/lock/doc.go

- [ ] Step 1: 실패하는 validation/key 테스트를 작성한다.

~~~go
func TestNewRejectsInvalidOptions(t *testing.T) {
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
    t.Cleanup(func() { _ = client.Close() })
    tests := []struct {
        name string
        client redis.Cmdable
        opts Options
        want error
    }{
        {name: "nil client", opts: Options{Key: "k", TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "blank key", client: client, opts: Options{Key: "  ", TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "zero ttl", client: client, opts: Options{Key: "k"}, want: btredis.ErrInvalidTTL},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := New(tt.client, tt.opts)
            if !errors.Is(err, tt.want) {
                t.Fatalf("New() error = %v, want %v", err, tt.want)
            }
        })
    }
}

func TestBuildKeysUsesSameRedactedHashTag(t *testing.T) {
    keys, err := buildKeys(" caller-owned:orders ")
    if err != nil { t.Fatal(err) }
    if !strings.Contains(keys.owner, "{redis-key:") || !strings.Contains(keys.counter, "{redis-key:") {
        t.Fatalf("keys must use redacted hash tags: %#v", keys)
    }
    ownerTag := keys.owner[strings.IndexByte(keys.owner, '{'):strings.IndexByte(keys.owner, '}')+1]
    counterTag := keys.counter[strings.IndexByte(keys.counter, '{'):strings.IndexByte(keys.counter, '}')+1]
    if ownerTag != counterTag { t.Fatalf("hash tags differ") }
}
~~~

- [ ] Step 2: 새 package 테스트를 실행해 함수 부재를 확인한다.

실행: go test -count=1 ./redis/lock -run 'TestNewRejectsInvalidOptions|TestBuildKeysUsesSameRedactedHashTag'

예상: Options, New, buildKeys undefined 실패.

- [ ] Step 3: validation과 key builder를 구현한다.

~~~go
var ErrNotAcquired = errors.New("redis fenced lock not acquired")

type Options struct {
    Key string
    TTL time.Duration
}

type options struct {
    key string
    ttl time.Duration
}

type keySet struct {
    owner string
    counter string
    keyID string
}

func isNilClient(client redis.Cmdable) bool {
    if client == nil { return true }
    value := reflect.ValueOf(client)
    switch value.Kind() {
    case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
        return value.IsNil()
    default:
        return false
    }
}

func (o Options) normalize(client redis.Cmdable) (options, error) {
    if isNilClient(client) { return options{}, fmt.Errorf("%w: redis client", btredis.ErrInvalidKey) }
    if strings.TrimSpace(o.Key) == "" { return options{}, fmt.Errorf("%w: lock key", btredis.ErrInvalidKey) }
    if err := btredis.ValidateTTL("lock", o.TTL); err != nil { return options{}, err }
    return options{key: o.Key, ttl: o.TTL}, nil
}

func buildKeys(logicalKey string) (keySet, error) {
    if logicalKey == "" { return keySet{}, fmt.Errorf("%w: lock key", btredis.ErrInvalidKey) }
    builder, err := btredis.NewKeyBuilder("bluetape:redis:lock")
    if err != nil { return keySet{}, err }
    builder, err = builder.WithHashTag(btredis.RedactedKeyID(logicalKey))
    if err != nil { return keySet{}, err }
    owner, err := builder.StructuralKey("owner")
    if err != nil { return keySet{}, err }
    counter, err := builder.StructuralKey("counter")
    if err != nil { return keySet{}, err }
    return keySet{owner: owner.Value, counter: counter.Value, keyID: btredis.RedactedKeyID(logicalKey)}, nil
}
~~~

doc.go에는 single-instance lock이며 external resource가 fencing token을
비교해야 한다는 경계를 기록한다.

- [ ] Step 4: 포맷·focused test를 통과시킨다.

~~~bash
gofmt -w redis/lock/*.go
go test -count=1 ./redis/lock -run 'TestNewRejectsInvalidOptions|TestBuildKeysUsesSameRedactedHashTag'
~~~

예상: 두 테스트 PASS, Redis dispatch 없음.

- [ ] Step 5: 작은 커밋을 만든다.

~~~bash
git add redis/lock
git commit -m "Add fenced lock validation and key contract" -m "Constraint: Keep the new package separate from lock/redis.Mutex.
Rejected: Raw logical keys in Redis storage were rejected to preserve redacted diagnostics.
Confidence: high
Scope-risk: narrow
Directive: Keep the counter key persistent and hash-slot aligned with the owner key.
Tested: Focused validation and key-layout tests.
Not-tested: Redis mutation scripts are pending."
~~~

## Task 2: acquire script와 fencing parser 구현

**Files:**
- Create: redis/lock/scripts.go
- Modify: redis/lock/lock_test.go
- Create: redis/lock/lock.go

- [ ] Step 1: parser 실패 테스트를 작성한다.

~~~go
func TestParseAcquireResult(t *testing.T) {
    tests := []struct {
        name string
        value any
        acquired bool
        fence uint64
        wantErr bool
    }{
        {name: "busy", value: []int64{0, 0}},
        {name: "acquired", value: []int64{1, 42}, acquired: true, fence: 42},
        {name: "wrong length", value: []int64{1}, wantErr: true},
        {name: "zero fence", value: []int64{1, 0}, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := redis.NewCmd(context.Background(), "eval")
            cmd.SetVal(tt.value)
            acquired, fence, err := parseAcquireResult(cmd)
            if (err != nil) != tt.wantErr { t.Fatalf("parse error = %v", err) }
            if err == nil && (acquired != tt.acquired || fence != tt.fence) {
                t.Fatalf("parse result = %t/%d", acquired, fence)
            }
        })
    }
}
~~~

- [ ] Step 2: parser 부재 실패를 확인한다.

실행: go test -count=1 ./redis/lock -run TestParseAcquireResult

예상: parseAcquireResult undefined.

- [ ] Step 3: atomic script와 parser를 구현한다.

scripts.go에 acquireScript와 다음 Lua source를 그대로 둔다.

~~~lua
if redis.call("EXISTS", KEYS[1]) == 1 then
    return {0, 0}
end
local fencing = redis.call("INCR", KEYS[2])
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
return {1, fencing}
~~~

parser는 Int64Slice의 길이와 양수 fencing token을 검증하고 busy
{0, 0}은 ErrNotAcquired 경로로 보낸다.

- [ ] Step 4: FencedLock, constructor, TryAcquire를 구현한다.

~~~go
type FencedLock struct {
    client redis.Cmdable
    opts options
    keys keySet
}

type Lease struct {
    lock *FencedLock
    key string
    owner btredis.OwnerToken
    fencing uint64
    shared btredis.Lease
}

func New(client redis.Cmdable, opts Options) (*FencedLock, error) {
    normalized, err := opts.normalize(client)
    if err != nil { return nil, err }
    keys, err := buildKeys(normalized.key)
    if err != nil { return nil, err }
    return &FencedLock{client: client, opts: normalized, keys: keys}, nil
}

func (l *FencedLock) TryAcquire(ctx context.Context) (*Lease, error) {
    ctx = normalizeContext(ctx)
    if err := ctx.Err(); err != nil { return nil, err }
    owner, err := btredis.NewOwnerToken()
    if err != nil { return nil, fmt.Errorf("generate fenced lock owner token: %w", err) }
    millis, err := btredis.TTLMillis("lock", l.opts.ttl)
    if err != nil { return nil, err }
    cmd := acquireScript.Run(ctx, l.client, []string{l.keys.owner, l.keys.counter}, owner.RedisValue(), millis)
    acquired, fence, err := parseAcquireResult(cmd)
    if err != nil { return l.reconcileAcquire(ctx, owner, err) }
    if !acquired { return nil, ErrNotAcquired }
    shared, err := btredis.NewLease(l.keys.owner, owner)
    if err != nil { return nil, err }
    return &Lease{lock: l, key: l.opts.key, owner: owner, fencing: fence, shared: shared}, nil
}

func operationError(operation, keyID string, err error) error {
    wrapped := btredis.NewOpError(btredis.OpLabels{Family: "redis fenced lock", Operation: operation}, keyID, err)
    return errors.Join(wrapped, btredis.ErrCommitUnknown)
}

func (l *FencedLock) reconcileAcquire(ctx context.Context, owner btredis.OwnerToken, cause error) (*Lease, error) {
    probeCtx, cancel := context.WithTimeout(context.Background(), min(l.opts.ttl, 250*time.Millisecond))
    defer cancel()
    value, err := l.client.Get(probeCtx, l.keys.owner).Result()
    if err == nil && value == owner.RedisValue() {
        rawFence, fenceErr := l.client.Get(probeCtx, l.keys.counter).Result()
        fence, parseErr := strconv.ParseUint(rawFence, 10, 64)
        if fenceErr == nil && parseErr == nil && fence > 0 {
            shared, leaseErr := btredis.NewLease(l.keys.owner, owner)
            if leaseErr == nil {
                return &Lease{lock: l, key: l.opts.key, owner: owner, fencing: fence, shared: shared}, nil
            }
        }
    }
    return nil, operationError("acquire", l.keys.keyID, errors.Join(cause, err))
}

func normalizeContext(ctx context.Context) context.Context {
    if ctx == nil { return context.Background() }
    return ctx
}
~~~

reconcileAcquire는 짧은 background probe로 owner/counter를 읽고 commit을
확정할 수 있을 때만 lease를 복원한다. 확정할 수 없으면 redacted
OpError와 ErrCommitUnknown을 함께 반환한다.

- [ ] Step 5: parser와 vet을 실행한다.

~~~bash
gofmt -w redis/lock/*.go
go test -count=1 ./redis/lock -run TestParseAcquireResult
go vet ./redis/lock
~~~

## Task 3: lease API, owner-safe release, ambiguity 검증

**Files:**
- Modify: redis/lock/lock.go
- Modify: redis/lock/lock_test.go
- Create: redis/lock/integration_test.go

- [ ] Step 1: nil/zero accessor 테스트를 작성한다.

~~~go
func TestNilLeaseIsIdempotent(t *testing.T) {
    var lease *Lease
    released, err := lease.Release(context.Background())
    if released || err != nil { t.Fatalf("nil Release() = %t, %v", released, err) }
}

func TestLeaseAccessorZeroValues(t *testing.T) {
    var lease Lease
    if lease.Key() != "" || lease.OwnerToken().RedisValue() != "" || lease.FencingToken() != 0 {
        t.Fatal("zero Lease accessors returned non-zero values")
    }
}
~~~

- [ ] Step 2: accessor와 shared compare-delete release를 구현한다.

~~~go
func (l *Lease) Key() string {
    if l == nil { return "" }
    return l.key
}

func (l *Lease) OwnerToken() btredis.OwnerToken {
    if l == nil { return btredis.OwnerToken{} }
    return l.owner
}

func (l *Lease) FencingToken() uint64 {
    if l == nil { return 0 }
    return l.fencing
}

func (l *Lease) Release(ctx context.Context) (bool, error) {
    if l == nil || l.lock == nil { return false, nil }
    ctx = normalizeContext(ctx)
    if err := ctx.Err(); err != nil { return false, err }
    released, err := btredis.CompareAndDelete(ctx, l.lock.client, l.shared, "redis fenced lock")
    if err != nil { return false, operationError("release", l.lock.keys.keyID, err) }
    return released, nil
}
~~~

- [ ] Step 3: Testcontainers expiry/monotonicity/owner mismatch를 추가한다.

redistestcontainer.Start(ctx, t)로 client를 만들고 cleanup을 등록한다. 첫
lease를 TTL 만료시킨 뒤 새 lease의 fencing token이 증가하는지, stale
release가 fresh owner를 삭제하지 않는지, fresh release가 true인지 확인한다.

테스트 helper는 Redis provider를 polling하되 고정 sleep에 의존하지 않는다.

~~~go
func waitForOwnerExpiry(t *testing.T, client redis.Cmdable, key string) {
    t.Helper()
    deadline := time.Now().Add(time.Second)
    for time.Now().Before(deadline) {
        if client.Exists(context.Background(), key).Val() == 0 { return }
        time.Sleep(5 * time.Millisecond)
    }
    t.Fatalf("owner key %q did not expire", btredis.RedactedKeyID(key))
}
~~~

~~~go
stale, err := first.TryAcquire(ctx)
if err != nil { t.Fatal(err) }
firstFence := stale.FencingToken()
waitForOwnerExpiry(t, client, first.keys.owner)
fresh, err := second.TryAcquire(ctx)
if err != nil { t.Fatal(err) }
if fresh.FencingToken() <= firstFence { t.Fatal("fencing token did not increase") }
if released, err := stale.Release(ctx); err != nil || released {
    t.Fatalf("stale release = %t, %v", released, err)
}
if released, err := fresh.Release(ctx); err != nil || !released {
    t.Fatalf("fresh release = %t, %v", released, err)
}
~~~

- [ ] Step 4: cancellation와 closed-client redaction을 검증한다.

이미 취소된 context는 Redis dispatch와 key 생성이 없어야 한다. closed
client 오류는 redis.ErrClosed와 ErrCommitUnknown을 보존하고 raw logical
key/owner token을 error string에 포함하지 않아야 한다.

- [ ] Step 5: focused integration/race test를 실행한다.

~~~bash
gofmt -w redis/lock/*.go
go test -p 1 -count=1 ./redis/lock
go test -p 1 -race -count=1 ./redis/lock
~~~

## Task 4: blocking Acquire를 context-first로 추가

**Files:**
- Create: redis/lock/wait.go
- Modify: redis/lock/lock.go
- Modify: redis/lock/lock_test.go

- [ ] Step 1: release 후 waiter acquire와 deadline 테스트를 작성한다.

~~~go
func TestAcquirePreservesDeadline(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
    defer cancel()
    client := redisClient(t)
    first, err := New(client, Options{Key: testKey(t), TTL: time.Second})
    if err != nil { t.Fatal(err) }
    held, err := first.TryAcquire(context.Background())
    if err != nil { t.Fatal(err) }
    defer func() { _, _ = held.Release(context.Background()) }()
    second, err := New(client, Options{Key: first.opts.key, TTL: time.Second})
    if err != nil { t.Fatal(err) }
    if _, err := second.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
        t.Fatalf("Acquire() error = %v, want deadline", err)
    }
}
~~~

- [ ] Step 2: bounded timer/select backoff를 구현한다.

~~~go
const (
    initialRetryDelay = 5 * time.Millisecond
    maxRetryDelay = 100 * time.Millisecond
)

func waitForRetry(ctx context.Context, delay time.Duration) error {
    timer := time.NewTimer(delay)
    defer timer.Stop()
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        return nil
    }
}

func (l *FencedLock) Acquire(ctx context.Context) (*Lease, error) {
    ctx = normalizeContext(ctx)
    delay := initialRetryDelay
    for {
        lease, err := l.TryAcquire(ctx)
        if !errors.Is(err, ErrNotAcquired) { return lease, err }
        if err := waitForRetry(ctx, delay); err != nil { return nil, err }
        if delay < maxRetryDelay {
            delay *= 2
            if delay > maxRetryDelay { delay = maxRetryDelay }
        }
    }
}
~~~

- [ ] Step 3: blocking/race 검증을 실행한다.

~~~bash
gofmt -w redis/lock/*.go
go test -p 1 -count=1 ./redis/lock -run Acquire
go test -p 1 -race -count=1 ./redis/lock
~~~

## Task 5: acceptance examples와 README 작성

**Files:**
- Create: redis/lock/example_test.go
- Create: redis/lock/README.md
- Create: redis/lock/README.ko.md
- Modify: redis/lock/doc.go

- [ ] Step 1: external resource fencing example을 작성한다.

~~~go
type fencedResource struct{ last uint64 }

func (r *fencedResource) Write(token uint64) error {
    if token <= r.last { return errors.New("stale fencing token") }
    r.last = token
    return nil
}
~~~

example은 Redis에 연결하지 않고 낮은 token rejection, cleanup context,
TTL 이후 stale/fresh overlap caveat를 compile-check한다.

- [ ] Step 2: English/Korean README에 운영 경계를 추가한다.

FencingToken은 보호 resource가 마지막 token을 비교·저장할 때만 stale
holder를 거부한다. lease TTL 이후 실행 중인 holder는 새 holder와 overlap할
수 있다. 업무 context가 취소되면 별도 cleanup context를 만들고
ErrCommitUnknown이면 실제 상태를 재확인한다.

- [ ] Step 3: examples/documentation 검증을 실행한다.

~~~bash
gofmt -w redis/lock/example_test.go
go test -count=1 ./redis/lock -run Example
git diff --check
~~~

## Task 6: lock package 최종 검증과 commit

- [ ] Step 1: targeted와 dependency tests를 순차 실행한다.

~~~bash
go test -p 1 -count=1 ./redis/lock
go test -p 1 -race -count=1 ./redis/lock
go test -p 1 -count=1 ./redis ./lock/redis
~~~

- [ ] Step 2: repository quality checks를 실행한다.

~~~bash
make fmt-check
make tidy-check
make vet
make lint
~~~

- [ ] Step 3: lock 구현을 Lore 커밋으로 기록한다.

~~~bash
git add redis/lock
git commit -m "Add Redis fenced lock primitive" -m "Constraint: Preserve lock/redis.Mutex and require external resource fencing.
Rejected: Watchdog renewal and Redlock quorum remain outside Issue #572.
Confidence: high
Scope-risk: moderate
Directive: Never expire or reset the fencing counter.
Tested: Targeted integration, race, repository Redis tests, fmt, tidy, vet, and lint.
Not-tested: Full make ci is recorded at issue integration gate."
~~~

## Lock plan self-review

- Spec의 API, persistent counter, same-slot key, owner-safe release,
  Acquire/TryAcquire, context error, ErrCommitUnknown, stale-resource
  example, cleanup timeout, over-TTL caveat, README, race 검증을 Task 1–6에
  매핑했다.
- ErrNotAcquired, Options, FencedLock, Lease, keySet, parser와 retry
  constants 명칭을 모든 task에서 일관되게 사용한다.
- 모든 단계에 파일·테스트·명령을 지정했으며 미정 placeholder를 남기지 않았다.
