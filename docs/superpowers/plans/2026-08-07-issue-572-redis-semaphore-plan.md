# Redis Semaphore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Issue #572의 redis/semaphore package가 TTL-aware bounded permits와 owner-safe idempotent release를 제공하도록 구현한다.

**Architecture:** Semaphore는 caller-owned redis.Cmdable과 KeyBuilder가 만든 하나의 sorted-set lease key를 보관한다. acquire Lua script가 Redis TIME으로 만료 member를 정리하고 permit을 추가하며, release Lua script는 exact owner-token member만 제거한다. TryAcquire는 즉시형이고 Acquire는 context-aware bounded backoff다.

**Tech Stack:** Go 1.26, github.com/redis/go-redis/v9, github.com/bluetape4k/bluetape-go/redis, Redis 7 Testcontainers, Go race detector.

---

## 파일 지도

| 파일 | 책임 |
|---|---|
| redis/semaphore/doc.go | permit/TTL/overlap 경계 |
| redis/semaphore/errors.go | ErrNotAcquired sentinel |
| redis/semaphore/options.go | Options{Key, Permits, TTL} validation |
| redis/semaphore/keys.go | digest hash-tag sorted-set key layout |
| redis/semaphore/scripts.go | acquire/release Lua와 result parser |
| redis/semaphore/semaphore.go | Semaphore, Lease, acquire/release API |
| redis/semaphore/wait.go | context-aware bounded retry timer |
| redis/semaphore/semaphore_test.go | validation, key/parser, cancellation |
| redis/semaphore/integration_test.go | accounting, expiry, owner mismatch, stress |
| redis/semaphore/example_test.go | cleanup timeout, over-TTL examples |
| redis/semaphore/README.md | English API/operational guidance |
| redis/semaphore/README.ko.md | Korean parity |

## Task 1: 옵션·key·capacity validation을 TDD로 고정

**Files:**
- Create: redis/semaphore/semaphore_test.go
- Create: redis/semaphore/errors.go
- Create: redis/semaphore/options.go
- Create: redis/semaphore/keys.go
- Create: redis/semaphore/doc.go

- [ ] Step 1: 실패하는 validation/key 테스트를 작성한다.

~~~go
func TestNewRejectsInvalidSemaphoreOptions(t *testing.T) {
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
    t.Cleanup(func() { _ = client.Close() })
    tests := []struct {
        name string
        client redis.Cmdable
        opts Options
        want error
    }{
        {name: "nil client", opts: Options{Key: "k", Permits: 1, TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "blank key", client: client, opts: Options{Key: " ", Permits: 1, TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "zero permits", client: client, opts: Options{Key: "k", TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "negative permits", client: client, opts: Options{Key: "k", Permits: -1, TTL: time.Second}, want: btredis.ErrInvalidKey},
        {name: "zero ttl", client: client, opts: Options{Key: "k", Permits: 1}, want: btredis.ErrInvalidTTL},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := New(tt.client, tt.opts)
            if !errors.Is(err, tt.want) { t.Fatalf("New() error = %v, want %v", err, tt.want) }
        })
    }
}

func TestBuildKeyUsesStableRedactedHashTag(t *testing.T) {
    keys, err := buildKeys(" caller-owned:permits ")
    if err != nil { t.Fatal(err) }
    if !strings.Contains(keys.leases, "{redis-key:") || !strings.HasSuffix(keys.leases, ":leases") {
        t.Fatalf("unexpected leases key: %q", keys.leases)
    }
}
~~~

- [ ] Step 2: package test를 실행해 함수 부재 실패를 확인한다.

실행: go test -count=1 ./redis/semaphore -run 'TestNewRejectsInvalidSemaphoreOptions|TestBuildKeyUsesStableRedactedHashTag'

예상: Options, New, buildKeys undefined.

- [ ] Step 3: options/key/sentinel을 구현한다.

~~~go
var ErrNotAcquired = errors.New("redis semaphore permit not acquired")

type Options struct {
    Key string
    Permits int
    TTL time.Duration
}

type options struct {
    key string
    permits int
    ttl time.Duration
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
    if strings.TrimSpace(o.Key) == "" { return options{}, fmt.Errorf("%w: semaphore key", btredis.ErrInvalidKey) }
    if o.Permits <= 0 { return options{}, fmt.Errorf("%w: permits", btredis.ErrInvalidKey) }
    if err := btredis.ValidateTTL("semaphore", o.TTL); err != nil { return options{}, err }
    return options{key: o.Key, permits: o.Permits, ttl: o.TTL}, nil
}

type keySet struct {
    leases string
    keyID string
}

func buildKeys(logicalKey string) (keySet, error) {
    if logicalKey == "" { return keySet{}, fmt.Errorf("%w: semaphore key", btredis.ErrInvalidKey) }
    builder, err := btredis.NewKeyBuilder("bluetape:redis:semaphore")
    if err != nil { return keySet{}, err }
    keyID := btredis.RedactedKeyID(logicalKey)
    builder, err = builder.WithHashTag(keyID)
    if err != nil { return keySet{}, err }
    leases, err := builder.StructuralKey("leases")
    if err != nil { return keySet{}, err }
    return keySet{leases: leases.Value, keyID: keyID}, nil
}
~~~

doc.go에는 bounded permit, lease expiry, no fencing token, over-TTL overlap을
명시한다.

- [ ] Step 4: 포맷·focused validation을 실행하고 작은 커밋을 만든다.

~~~bash
gofmt -w redis/semaphore/*.go
go test -count=1 ./redis/semaphore -run 'TestNewRejectsInvalidSemaphoreOptions|TestBuildKeyUsesStableRedactedHashTag'
git add redis/semaphore
git commit -m "Add Redis semaphore validation and key contract" -m "Constraint: Keep semaphore ownership explicit and independent from redis/lock.
Rejected: Shared generic coordination state was rejected to keep permit semantics narrow.
Confidence: high
Scope-risk: narrow
Directive: Use exact owner-token members for every permit.
Tested: Focused validation and key-layout tests.
Not-tested: Redis sorted-set scripts are pending."
~~~

## Task 2: sorted-set scripts와 lease API 구현

**Files:**
- Create: redis/semaphore/scripts.go
- Create: redis/semaphore/semaphore.go
- Modify: redis/semaphore/semaphore_test.go

- [ ] Step 1: parser 실패 테스트를 작성한다.

~~~go
func TestParseAcquireResult(t *testing.T) {
    tests := []struct {
        name string
        value any
        want bool
        wantErr bool
    }{
        {name: "busy", value: int64(0)},
        {name: "acquired", value: int64(1), want: true},
        {name: "unexpected", value: int64(2), wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := redis.NewCmd(context.Background(), "eval")
            cmd.SetVal(tt.value)
            got, err := parseAcquireResult(cmd)
            if (err != nil) != tt.wantErr { t.Fatalf("parse error = %v", err) }
            if err == nil && got != tt.want { t.Fatalf("parse result = %t, want %t", got, tt.want) }
        })
    }
}
~~~

- [ ] Step 2: acquire/release Lua script와 parser를 추가한다.

scripts.go의 acquire source는 Redis server time으로 만료 member를 먼저
지우고, ZCARD가 permits 미만일 때만 token을 ZADD한다.

~~~lua
local now = redis.call("TIME")
local nowMillis = now[1] * 1000 + math.floor(now[2] / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", nowMillis)
if redis.call("ZCARD", KEYS[1]) >= tonumber(ARGV[1]) then
    return 0
end
redis.call("ZADD", KEYS[1], nowMillis + tonumber(ARGV[2]), ARGV[3])
return 1
~~~

release source는 ZREM의 반환값을 그대로 돌려주며, parser는 0/1 외의
결과를 validation error로 처리한다.

- [ ] Step 3: Semaphore, Lease, constructor, TryAcquire, Release를 구현한다.

~~~go
type Semaphore struct {
    client redis.Cmdable
    opts options
    keys keySet
}

type Lease struct {
    semaphore *Semaphore
    key string
    owner btredis.OwnerToken
}

func New(client redis.Cmdable, opts Options) (*Semaphore, error) {
    normalized, err := opts.normalize(client)
    if err != nil { return nil, err }
    keys, err := buildKeys(normalized.key)
    if err != nil { return nil, err }
    return &Semaphore{client: client, opts: normalized, keys: keys}, nil
}

func (s *Semaphore) TryAcquire(ctx context.Context) (*Lease, error) {
    ctx = normalizeContext(ctx)
    if err := ctx.Err(); err != nil { return nil, err }
    owner, err := btredis.NewOwnerToken()
    if err != nil { return nil, fmt.Errorf("generate semaphore owner token: %w", err) }
    ttlMillis, err := btredis.TTLMillis("semaphore", s.opts.ttl)
    if err != nil { return nil, err }
    cmd := acquireScript.Run(ctx, s.client, []string{s.keys.leases}, s.opts.permits, ttlMillis, owner.RedisValue())
    acquired, err := parseAcquireResult(cmd)
    if err != nil { return nil, operationError("acquire", s.keys.keyID, err) }
    if !acquired { return nil, ErrNotAcquired }
    return &Lease{semaphore: s, key: s.opts.key, owner: owner}, nil
}

func (l *Lease) Release(ctx context.Context) (bool, error) {
    if l == nil || l.semaphore == nil { return false, nil }
    ctx = normalizeContext(ctx)
    if err := ctx.Err(); err != nil { return false, err }
    cmd := releaseScript.Run(ctx, l.semaphore.client, []string{l.semaphore.keys.leases}, l.owner.RedisValue())
    result, err := cmd.Int64()
    if err != nil { return false, operationError("release", l.semaphore.keys.keyID, err) }
    return result == 1, nil
}

func (l *Lease) Key() string {
    if l == nil { return "" }
    return l.key
}

func (l *Lease) OwnerToken() btredis.OwnerToken {
    if l == nil { return btredis.OwnerToken{} }
    return l.owner
}

func operationError(operation, keyID string, err error) error {
    wrapped := btredis.NewOpError(btredis.OpLabels{Family: "redis semaphore", Operation: operation}, keyID, err)
    return errors.Join(wrapped, btredis.ErrCommitUnknown)
}

func normalizeContext(ctx context.Context) context.Context {
    if ctx == nil { return context.Background() }
    return ctx
}
~~~

operationError는 btredis.OpError와 btredis.ErrCommitUnknown을 함께 보존하고,
provider 오류는 Acquire loop가 재시도하지 않도록 한다.

- [ ] Step 4: parser/accessor 단위 테스트와 vet을 실행한다.

~~~bash
gofmt -w redis/semaphore/*.go
go test -count=1 ./redis/semaphore -run 'TestParseAcquireResult|TestLeaseAccessor'
go vet ./redis/semaphore
~~~

## Task 3: blocking Acquire와 cancellation 추가

**Files:**
- Create: redis/semaphore/wait.go
- Modify: redis/semaphore/semaphore.go
- Modify: redis/semaphore/semaphore_test.go

- [ ] Step 1: permit exhaustion과 deadline 테스트를 작성한다.

~~~go
func TestAcquirePreservesDeadlineWithoutLeakingPermit(t *testing.T) {
    client := redisClient(t)
    sem, err := New(client, Options{Key: testKey(t), Permits: 1, TTL: time.Second})
    if err != nil { t.Fatal(err) }
    held, err := sem.TryAcquire(context.Background())
    if err != nil { t.Fatal(err) }
    defer func() { _, _ = held.Release(context.Background()) }()
    ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
    defer cancel()
    if _, err := sem.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
        t.Fatalf("Acquire() error = %v, want deadline", err)
    }
    if _, err := sem.TryAcquire(context.Background()); !errors.Is(err, ErrNotAcquired) {
        t.Fatalf("canceled waiter leaked or stole permit: %v", err)
    }
}
~~~

- [ ] Step 2: timer/select backoff와 Acquire loop를 구현한다.

wait.go에 initialRetryDelay 5ms, maxRetryDelay 100ms, waitForRetry,
nextRetryDelay를 정의한다. Acquire는 다음 exact loop로 ErrNotAcquired만
재시도한다.

~~~go
const (
    initialRetryDelay = 5 * time.Millisecond
    maxRetryDelay = 100 * time.Millisecond
)

func waitForRetry(ctx context.Context, delay time.Duration) error {
    timer := time.NewTimer(delay)
    defer timer.Stop()
    select {
    case <-ctx.Done(): return ctx.Err()
    case <-timer.C: return nil
    }
}

func nextRetryDelay(delay time.Duration) time.Duration {
    if delay >= maxRetryDelay { return maxRetryDelay }
    next := delay * 2
    if next > maxRetryDelay { return maxRetryDelay }
    return next
}

func (s *Semaphore) Acquire(ctx context.Context) (*Lease, error) {
    ctx = normalizeContext(ctx)
    delay := initialRetryDelay
    for {
        lease, err := s.TryAcquire(ctx)
        if !errors.Is(err, ErrNotAcquired) { return lease, err }
        if err := waitForRetry(ctx, delay); err != nil { return nil, err }
        delay = nextRetryDelay(delay)
    }
}
~~~

goroutine, ticker, watchdog를 생성하지 않는다.

- [ ] Step 3: blocking/race 검증을 실행한다.

~~~bash
gofmt -w redis/semaphore/*.go
go test -p 1 -count=1 ./redis/semaphore -run Acquire
go test -p 1 -race -count=1 ./redis/semaphore
~~~

## Task 4: permit expiry, owner mismatch, idempotency, contention 검증

**Files:**
- Create: redis/semaphore/integration_test.go
- Modify: redis/semaphore/semaphore_test.go

- [ ] Step 1: Testcontainers client와 unique key cleanup을 추가한다.

redistestcontainer.Start(ctx, t)로 Redis를 시작하고
bluetape:test:semaphore:<test-name> logical key를 사용한다. client close와
DEL cleanup은 t.Cleanup으로 등록하며 package 실행은 -p 1로 유지한다.

- [ ] Step 2: permit accounting과 idempotent release를 테스트한다.

~~~go
sem, _ := New(client, Options{Key: key, Permits: 2, TTL: time.Second})
first, _ := sem.TryAcquire(ctx)
second, _ := sem.TryAcquire(ctx)
if _, err := sem.TryAcquire(ctx); !errors.Is(err, ErrNotAcquired) { t.Fatal(err) }
if released, err := first.Release(ctx); err != nil || !released { t.Fatal(err) }
if released, err := first.Release(ctx); err != nil || released { t.Fatal("release must be idempotent") }
third, err := sem.TryAcquire(ctx)
if err != nil || third == nil { t.Fatalf("permit after release = %v", err) }
_, _ = second.Release(ctx)
_, _ = third.Release(ctx)
~~~

- [ ] Step 3: expiry cleanup과 stale member 보호를 테스트한다.

TTL을 40ms로 두고 첫 lease가 만료된 뒤 두 번째 TryAcquire가 성공하는지
확인한다. 첫 lease Release가 두 번째 member를 제거하지 않는지 확인하고,
두 번째 release 뒤 ZCARD가 0인지 확인한다.

- [ ] Step 4: cancellation path permit leakage를 테스트한다.

한 permit을 held 상태로 두고 deadline context로 Acquire를 호출한다.
deadline 오류 후 held lease를 release하고 새 TryAcquire가 성공하는지
확인해 canceled waiter가 member를 남기지 않았음을 증명한다.

- [ ] Step 5: same-key concurrent stress를 추가한다.

concurrencytest.NewGoroutineStressTester로 16 workers/64 rounds를 실행하고
atomic active counter가 항상 Permits 이하인지 검증한다. 성공 lease는 모두
cleanup context로 release한다.

## Task 5: examples와 README 작성

**Files:**
- Create: redis/semaphore/example_test.go
- Create: redis/semaphore/README.md
- Create: redis/semaphore/README.ko.md
- Modify: redis/semaphore/doc.go

- [ ] Step 1: cleanup timeout example을 추가한다.

~~~go
cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
released, err := lease.Release(cleanupCtx)
_ = released
_ = errors.Is(err, btredis.ErrCommitUnknown)
~~~

- [ ] Step 2: English/Korean README에 no-fencing/over-TTL 경계를 추가한다.

Semaphore lease TTL은 permit 자동 회수 경계다. TTL 이후 실행 중인 작업은
새 permit과 overlap할 수 있고 semaphore는 fencing token을 제공하지 않는다.
외부 resource version/ownership 검증이나 짧은 critical section이 필요하다.

- [ ] Step 3: docs/examples parity를 검증한다.

~~~bash
gofmt -w redis/semaphore/example_test.go
go test -count=1 ./redis/semaphore -run Example
git diff --check
~~~

## Task 6: semaphore 최종 검증과 commit

- [ ] Step 1: targeted, race, dependency tests를 순차 실행한다.

~~~bash
go test -p 1 -count=1 ./redis/semaphore
go test -p 1 -race -count=1 ./redis/semaphore
go test -p 1 -count=1 ./redis
~~~

- [ ] Step 2: repository quality checks를 실행한다.

~~~bash
make fmt-check
make tidy-check
make vet
make lint
~~~

- [ ] Step 3: semaphore 구현을 Lore 커밋으로 기록한다.

~~~bash
git add redis/semaphore
git commit -m "Add Redis bounded semaphore primitive" -m "Constraint: Permit ownership is an exact owner-token sorted-set member with TTL.
Rejected: Fencing tokens, watchdog renewal, and FIFO ordering remain out of scope.
Confidence: high
Scope-risk: moderate
Directive: Cleanup expired members atomically before every acquire.
Tested: Permit accounting, expiry, cancellation, contention, race, repository Redis tests, fmt, tidy, vet, and lint.
Not-tested: Full make ci is recorded at issue integration gate."
~~~

## Semaphore plan self-review

- Spec의 bounded permits, Redis server-time expiry cleanup, exact-member release,
  idempotency, cancellation/deadline, no-fencing claim, overlap caveat, README,
  examples, stress/race 검증을 Task 1–6에 매핑했다.
- Options, Semaphore, Lease, ErrNotAcquired, acquireScript, releaseScript,
  parser와 wait helper 명칭을 모든 task에서 일관되게 사용한다.
- 모든 단계에 실제 파일·테스트·명령을 지정했으며 미정 placeholder를 남기지 않았다.
