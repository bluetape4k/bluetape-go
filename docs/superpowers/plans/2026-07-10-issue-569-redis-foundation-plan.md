# Issue 569 Redis foundation 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 the 공개 `github.com/bluetape4k/bluetape-go/redis` 패키지 함께 safe Redis key, owner-token, lease script, TTL, 및 operational 오류 primitives.

**아키텍처:** 구현 a narrow 패키지 named `btredis` that reuses `go-redis/v9` 만 for script execution 및 does 아님 wrap the Redis client. Existing Redis-backed packages are 아님 migrated in this issue; #570 owns migration 후 parity 테스트 및 benchmarks.

**기술 스택:** `go.mod` declares Go 1.26.3, current local toolchain is `go1.26.5`, `github.com/redis/go-redis/v9`, 기존 `testcontainers/redis` fixture, table-driven `testing`, TDD, `go test`, `go test -race`, `make ci`.

---

## 입력

- Spec: `docs/superpowers/specs/2026-07-10-issue-569-redis-foundation-spec.md`
- Spec review: `docs/superpowers/reviews/2026-07-10-issue-569-redis-foundation-spec-review.md`
- Issue: #569
- Milestone: `0.19.0`

## 파일 지도

| Path | 책임 |
|---|---|
| `redis/doc.go` | Package overview 및 safety boundary. |
| `redis/errors.go` | Sentinels, `OpError`, redacted 오류 construction. |
| `redis/key.go` | `Key`, `KeyBuilder`, redacted key id helpers. |
| `redis/token.go` | Opaque `OwnerToken` generation, parsing, validation, redacted formatting. |
| `redis/ttl.go` | TTL validation 및 millisecond conversion. |
| `redis/lease.go` | Immutable `Lease` 및 validation. |
| `redis/script.go` | Package-level compare-delete / compare-extend Lua scripts 및 helpers. |
| `redis/*_test.go` | Unit, fake-scripter, integration, stress, 및 example. |
| `redis/README.md`, `redis/README.ko.md` | Public 패키지 docs 및 운영자 guidance. |
| `README.md`, `README.ko.md` | Root 패키지 index updates. |
| `CHANGELOG.md` | `[Unreleased]` 패키지 addition. |
| `docs/lessons/2026-07-10-issue-569-redis-foundation.md` | Session lesson 전에 PR. |
| `docs/review/2026-07-10-issue-569-redis-foundation-review.md` | 단계 6-R final review artifact. |

## Current-Code Assumptions To Recheck Before 단계 4

- `go.mod` still contains `github.com/redis/go-redis/v9`.
- `testcontainers/redis.Start(ctx, tb)` still returns a Redis address.
- No top-level `redis/` 패키지 exists.
- Existing Redis packages are 아님 edited except docs/root indexes in this issue.

## 작업 1: Package Scaffold And Token Contract

complexity: 높음
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `redis/doc.go`
- 생성: `redis/token.go`
- 생성: `redis/token_test.go`

- [ ] **단계 1: Write failing token 테스트**

생성 `redis/token_test.go`:

```go
package btredis

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"testing"
)

func TestNewOwnerTokenReturnsCanonicalRedactedToken(t *testing.T) {
	token, err := NewOwnerToken()
	if err != nil {
		t.Fatalf("NewOwnerToken() error = %v", err)
	}
	if err := token.Validate(); err != nil {
		t.Fatalf("token.Validate() error = %v", err)
	}
	raw := token.RedisValue()
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(raw) {
		t.Fatalf("RedisValue() length = %d, want 64 lowercase hex characters", len(raw))
	}
	if got := token.String(); got == raw || got == "" {
		t.Fatalf("String() = %q, want non-empty redacted value different from raw token", got)
	}
	if printed := fmt.Sprint(token); printed == raw {
		t.Fatal("fmt.Sprint(token) leaked raw token")
	}
	if printed := fmt.Sprintf("%#v", token); contains(printed, raw) {
		t.Fatal("debug formatting leaked raw token")
	}
	if printed := fmt.Sprintf("%+v", token); contains(printed, raw) {
		t.Fatal("verbose formatting leaked raw token")
	}
	if printed := token.GoString(); contains(printed, raw) {
		t.Fatal("GoString leaked raw token")
	}
	if value := token.LogValue(); value.Kind() != slog.KindString || contains(value.String(), raw) {
		t.Fatal("slog LogValue leaked raw token")
	}
}

func TestParseOwnerTokenRejectsNonCanonicalValues(t *testing.T) {
	valid := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if token, err := ParseOwnerToken(valid); err != nil || token.RedisValue() != valid {
		t.Fatalf("ParseOwnerToken(valid) raw length = %d, has error = %t", len(token.RedisValue()), err != nil)
	}
	invalid := []string{
		"",
		" ",
		"0123456789abcdef",
		valid + "00",
		"0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	}
	for i, value := range invalid {
		t.Run(fmt.Sprintf("case-%02d", i), func(t *testing.T) {
			token, err := ParseOwnerToken(value)
			if err == nil {
				t.Fatalf("ParseOwnerToken invalid case %d returned raw length %d, nil error", i, len(token.RedisValue()))
			}
			if !errors.Is(err, ErrInvalidOwnerToken) {
				t.Fatalf("ParseOwnerToken invalid case %d sentinel match = false, want ErrInvalidOwnerToken", i)
			}
			if value != "" && value != " " && contains(err.Error(), value) {
				t.Fatalf("error leaked invalid token case %d", i)
			}
		})
	}
}

func TestOwnerTokenZeroValueInvalid(t *testing.T) {
	var token OwnerToken
	if err := token.Validate(); !errors.Is(err, ErrInvalidOwnerToken) {
		t.Fatalf("zero token Validate() = %v, want ErrInvalidOwnerToken", err)
	}
	if token.RedisValue() != "" {
		t.Fatalf("zero token RedisValue() = %q, want empty", token.RedisValue())
	}
}

func TestNewOwnerTokenConcurrentBoundedUniqueness(t *testing.T) {
	const goroutines = 16
	const perGoroutine = 32

	var wg sync.WaitGroup
	seen := make(chan string, goroutines*perGoroutine)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGoroutine {
				token, err := NewOwnerToken()
				if err != nil {
					t.Errorf("NewOwnerToken() error = %v", err)
					return
				}
				seen <- token.RedisValue()
			}
		}()
	}
	wg.Wait()
	close(seen)

	unique := make(map[string]struct{}, goroutines*perGoroutine)
	for raw := range seen {
		if _, exists := unique[raw]; exists {
			t.Fatalf("unexpected probabilistic token collision for token length %d", len(raw))
		}
		unique[raw] = struct{}{}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && regexp.MustCompile(regexp.QuoteMeta(sub)).FindStringIndex(s) != nil
}
```

- [ ] **단계 2: 검증 token 테스트 fail for missing 패키지**

실행:

```bash
go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'
```

예상: FAIL because `./redis` does 아님 yet exist 또는 token symbols are undefined.

- [ ] **단계 3: 구현 minimal token 및 패키지 docs**

생성 `redis/doc.go`:

```go
// Package btredis provides small Redis safety primitives shared by Redis-backed
// bluetape-go packages.
//
// The package intentionally does not wrap the go-redis client and does not own
// Redis connections, logging, metrics, retries, or tenant isolation. Callers own
// Redis clients, deadlines, access control, and package-specific key contracts.
//
// Owner tokens are sensitive lease credentials. Do not log RedisValue output.
package btredis
```

생성 `redis/token.go`:

```go
package btredis

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrInvalidOwnerToken is returned when an owner token is empty or non-canonical.
var ErrInvalidOwnerToken = errors.New("redis: invalid owner token")

// OwnerToken is an opaque Redis lease credential.
type OwnerToken struct {
	value string
}

// NewOwnerToken returns a 256-bit random token encoded as lowercase hex.
func NewOwnerToken() (OwnerToken, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return OwnerToken{}, fmt.Errorf("redis owner token: %w", err)
	}
	return OwnerToken{value: hex.EncodeToString(data[:])}, nil
}

// ParseOwnerToken parses a canonical lowercase 256-bit hex owner token.
func ParseOwnerToken(value string) (OwnerToken, error) {
	token := OwnerToken{value: value}
	if err := token.Validate(); err != nil {
		return OwnerToken{}, err
	}
	return token, nil
}

// String returns a redacted display value.
func (t OwnerToken) String() string {
	if t.value == "" {
		return "redis-owner-token:<empty>"
	}
	return "redis-owner-token:<redacted>"
}

// GoString returns a redacted debug display value.
func (t OwnerToken) GoString() string {
	return t.String()
}

// LogValue returns a redacted structured logging value.
func (t OwnerToken) LogValue() slog.Value {
	return slog.StringValue(t.String())
}

// RedisValue returns the sensitive Redis comparison value.
func (t OwnerToken) RedisValue() string {
	return t.value
}

// Validate verifies that the token is canonical and non-empty.
func (t OwnerToken) Validate() error {
	if strings.TrimSpace(t.value) == "" || !tokenPattern.MatchString(t.value) {
		return fmt.Errorf("%w: expected 64 lowercase hex characters", ErrInvalidOwnerToken)
	}
	return nil
}
```

- [ ] **단계 4: 검증 token 테스트 pass**

실행:

```bash
go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'
```

예상: PASS.

## 작업 2: Key Builder, Redaction, TTL, And OpError

complexity: 높음
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `redis/key.go`
- 생성: `redis/ttl.go`
- 생성: `redis/errors.go`
- 생성: `redis/key_test.go`
- 생성: `redis/ttl_test.go`
- 생성: `redis/errors_test.go`

- [ ] **단계 1: Write failing key, TTL, 및 오류 테스트**

생성 `redis/key_test.go`, `redis/ttl_test.go`, 및 `redis/errors_test.go` 함께 table-driven 테스트 covering:

- structural segments reject `""`, `" "`, `"a:b"`, `"{bad}"`;
- `NewKeyBuilder("bluetape:probabilistic:bloom:v1")` accepts a colon-delimited 패키지 prefix by validating each prefix part as a structural segment;
- invalid prefixes such as `""`, `" "`, `"bad::prefix"`, `"bad:{tag}"`, 및 `"bad: part"` return `ErrInvalidKey`;
- `LogicalKey(" lock key ")`, `LogicalKey("tenant:user:{caller}:a:b")`, 및 `LogicalKey("line\nkey")` preserve exact bytes;
- `WithHashTag("test:package:case")` preserves the colon-bearing hash tag so #570 can keep 기존 `probabilistic/redis` namespace keys;
- `WithHashTag("")`, `WithHashTag("{bad}")`, 및 `WithHashTag("bad{tag}")` return `ErrInvalidHashTag`;
- `StructuralKey("bits")` 및 `StructuralKey("config")` preserve same hash tag;
- `KeyBuilder{}` methods return `ErrInvalidKey` 또는 `ErrInvalidHashTag` 및 do 아님 panic;
- `RedactedKeyID("tenant:secret")` is deterministic, matches `^redis-key:[0-9a-f]{24}$`, 및 does 아님 contain `tenant:secret`;
- `TTLMillis("lease ttl", time.Millisecond)` returns `1`;
- zero, negative, 및 sub-millisecond TTLs return `ErrInvalidTTL`;
- `NewOpError(OpLabels{Family: "redis lock", Operation: "release"}, "raw:key", context.DeadlineExceeded)` returns an `OpError`, wraps the 원인, 및 does 아님 print `raw:key`;
- `NewOpErrorWithRedactedKey` preserves a valid already-redacted key id;
- `NewOpErrorWithRedactedKey` rejects raw-key-looking values 및 malformed IDs 함께 `ErrInvalidKey` without echoing the rejected value;
- `NewOpError` 및 `NewOpErrorWithRedactedKey` do 아님 include raw keys 또는 owner tokens in `Error()` even when the wrapped 원인 message contains them;
- `OpLabels` rejects empty, overlong, 또는 delimiter-heavy family/operation labels without echoing rejected label text;
- `Key.String()` 및 `Key.GoString()` return `RedactedID`, 아님 `Value`.

사용 this exact command:

```bash
go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'
```

예상: FAIL because the new symbols are 아님 implemented.

- [ ] **단계 2: 구현 key builder, TTL helpers, 및 오류**

구현:

```go
var (
	ErrInvalidKey     = errors.New("redis: invalid key")
	ErrInvalidHashTag = errors.New("redis: invalid hash tag")
	ErrInvalidTTL     = errors.New("redis: invalid ttl")
)
```

Implementation constraints:

- `RedactedKeyID` uses SHA-256 및 the first 12 bytes encoded as hex.
- `ValidateRedactedKeyID` accepts 만 `redis-key:<24 낮음ercase hex>`.
- `KeyBuilder` stores prefix parts, optional hash tag, 및 structural parts.
- `NewKeyBuilder(prefix)` splits a colon-delimited 패키지 prefix, validates each prefix part as a structural segment, 및 stores the original colon-delimited prefix shape through its parts.
- `Structural(parts...)` returns a copied builder 함께 appended structural parts.
- `WithHashTag(tag)` validates non-empty 및 없음 braces, then stores tag verbatim; `:` is al낮음ed because 기존 `probabilistic/redis` namespaces use colon-bearing hash tags.
- `StructuralKey(parts...)` appends structural suffixes.
- `LogicalKey(logicalKey)` validates non-blank by `strings.TrimSpace` but appends the original string.
- `Key.String()` 및 `Key.GoString()` return `RedactedID`; `Key.Value` remains sensitive Redis command input.
- `ValidateTTL` rejects `ttl < time.Millisecond` 및 `ttl <= 0`.
- `TTLMillis` calls `ValidateTTL` 및 returns `ttl.Milliseconds()`.
- `OpLabels` groups 낮음-cardinality `Family` 및 `Operation` values to avoid adjacent positional key/label strings. Label validation rejects empty, overlong, colon/braces, 및 whitespace-만 values.
- `NewOpError` computes `RedactedKeyID(rawKey)` 및 never stores raw key.
- `NewOpErrorWithRedactedKey` validates the redacted key id 전에 storing it.
- `OpError.Error()` prints family, operation, redacted key id, 및 원인 type/category 만; it must 아님 include the wrapped 원인 text because provider 오류 may contain raw keys 또는 tokens.
- `OpError.Is` delegates to the wrapped 원인.

- [ ] **단계 3: 검증 key, TTL, 및 오류 테스트 pass**

실행:

```bash
go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'
```

예상: PASS.

## 작업 3: Lease And Script Helpers With Fake Scripter No-Dispatch Tests

complexity: 높음
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `redis/lease.go`
- 생성: `redis/script.go`
- 생성: `redis/lease_test.go`
- 생성: `redis/script_test.go`

- [ ] **단계 1: Write failing lease 및 fake-scripter 테스트**

Tests must prove:

- `NewLease("", validToken)` 및 `NewLease("key", OwnerToken{})` return sentinel-compatible 오류.
- `Lease{}` is invalid.
- `CompareAndDelete(nil, fake, lease, "redis test")` returns a validation 오류 및 fake call count remains 0.
- `CompareAndDelete(ctx, nil, lease, "redis test")` 및 `CompareAndExtend(ctx, nil, lease, ttl, "redis test")` return validation 오류 및 do 아님 panic.
- a typed nil client returns a validation 오류 when detectable without unsafe reflection.
- pre-canceled context returns `context.Canceled` 및 fake call count remains 0.
- invalid lease returns validation 오류 및 fake call count remains 0.
- invalid TTL for `CompareAndExtend` returns `ErrInvalidTTL` 및 fake call count remains 0.
- fake success returning integer `1` maps to `true`.
- fake success returning integer `0` maps to `false`.
- fake Redis 오류 wraps as `OpError` 및 preserves the 원인.
- fake Redis 오류 함께 raw key/token text in the 원인 still produces an `OpError.Error()` string that contains 만 the redacted key id 및 없음 raw key/token.

실행:

```bash
go test -count=1 ./redis -run 'Lease|CompareAnd'
```

예상: FAIL until `lease.go` 및 `script.go` exist.

- [ ] **단계 2: 구현 immutable lease 및 패키지-level scripts**

구현 constraints:

- `Lease` has unexported `key string` 및 `token OwnerToken`.
- `NewLease` validates non-blank key without trimming stored key.
- `Lease.Key`, `Lease.RedactedKeyID`, `Lease.Token`, 및 `Lease.Validate` are value methods.
- `compareDeleteScript := redis.NewScript(...)` 및 `compareExtendScript := redis.NewScript(...)` are 패키지-level vars.
- Script helpers validate in this order 전에 Redis dispatch:
  1. `ctx != nil`
  2. `ctx.Err()`
  3. `client != nil` 및 detectable typed nil client rejection
  4. `lease.Validate()`
  5. for extend 만, `TTLMillis("lease ttl", ttl)`
- Script helpers call `script.Run(ctx, client, []string{lease.Key()}, lease.Token().RedisValue(), ...)`.
- Script helpers build `OpLabels{Family: family, Operation: "compare-delete"}` 및 `OpLabels{Family: family, Operation: "compare-extend"}` for 오류 construction.
- Result integer `1` is true, `0` is false; any parse 오류 is wrapped in `OpError`.

- [ ] **단계 3: 검증 fake-scripter 테스트 pass**

실행:

```bash
go test -count=1 ./redis -run 'Lease|CompareAnd'
```

예상: PASS.

## 작업 4: Redis Testcontainers Integration And Stress

complexity: 높음
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `redis/integration_test.go`
- 생성 또는 update: `redis/script_test.go`

- [ ] **단계 1: Write failing Redis integration 테스트**

사용 `redistestcontainer.Start(ctx, t)` from `github.com/bluetape4k/bluetape-go/testcontainers/redis` 및 `redis.NewClient`.

Tests:

- `TestCompareAndDeleteRemovesOnlyMatchingOwner`
- `TestCompareAndDeleteDoesNotRemoveLaterOwner`
- `TestCompareAndExtendUpdatesOnlyMatchingOwner`
- `TestCompareAndExtendDoesNotExtendLaterOwner`
- `TestCompareAndDeleteCanceledContextReturnsContextError`
- `TestCompareAndExtendDeadlineReturnsContextError`
- `TestCompareAndDeleteFirstRunUsesGoRedisScriptFallback`
- `TestCompareAndDeleteInterleavedOwnersStress`
- `TestCompareAndExtendInterleavedOwnersStress`

Each 테스트 must:

- use `context.WithTimeout`;
- use unique keys from `t.Name()`;
- call `t.Cleanup` to delete keys 및 close client;
- 아님 call `t.Parallel` in Testcontainers-backed 테스트 또는 in parent subtests;
- run Testcontainers-backed Redis 테스트 serially through 패키지 command, 아님 parallel external invocations.
- assert first-run helper execution succeeds against a fresh real client, relying on go-redis `Script.Run` to handle `EVALSHA`/`EVAL` fallback instead of reimplementing fallback in this 패키지.
- stress 테스트 use at least 8 workers 및 32 iterations per worker on a 공유 key, interleave stale 및 later owners, 및 assert the later owner value remains intact; extend stress also asserts stale owners do 아님 increase the later owner's TTL beyond the expected owner TTL window.

실행:

```bash
go test -p 1 -count=1 ./redis -run 'CompareAnd(Delete|Extend)'
```

예상: FAIL until integration support is complete.

- [ ] **단계 2: 구현 missing integration support**

Only adjust production code if integration 테스트 expose a real 계약 gap. 다음을 하지 않는다: add retry loops 또는 provider migration code.

- [ ] **단계 3: 검증 integration 및 race**

실행:

```bash
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
```

예상: PASS.

## 작업 5: Examples, README Locale Set, Root Index, And Changelog

complexity: 보통
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `redis/example_test.go`
- 생성: `redis/README.md`
- 생성: `redis/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **단계 1: 추가 compile-checked example**

생성 example:

- `ExampleNewOwnerToken`
- `ExampleKeyBuilder_LogicalKey`
- `ExampleKeyBuilder_StructuralKey`
- `ExampleCompareAndDelete`
- `ExampleOpError`

Examples must 아님 print raw owner token values. 사용 `String()` 또는 redacted key IDs 만.
`ExampleCompareAndDelete` must show `context.WithTimeout` 및 document that
`(false, nil)` means ownership drift. `ExampleOpError` must use `OpLabels` 및
must 아님 pass raw key material as family 또는 operation labels.

실행:

```bash
go test -count=1 ./redis -run Example
```

예상: PASS 후 example compile.

- [ ] **단계 2: Write 패키지 READMEs**

`redis/README.md` 및 `redis/README.ko.md` must cover:

- import path 및 패키지 name;
- non-goals: 없음 generic Redis facade, 없음 migration of 기존 packages in #569;
- key preservation 및 structural/logical split;
- hash-tags are same-slot helpers, 아님 tenant isolation;
- owner-token secrecy 및 redacted formatting;
- `context.WithTimeout` example 및 post-dispatch cancellation indeterminate state;
- `(false, nil)` means ownership drift, 아님 an infrastructure 오류;
- `OpError` 및 redacted 진단;
- Redis script/client 오류 triage 및 the fact that `OpError.Error()` is
  sanitized while the 원인 remains available through `errors.Is` / `errors.As`;
- cleanup guidance for partial failures;
- rollback/없음-migration behavior for #569;
- 테스트 commands.

- [ ] **단계 3: 업데이트 root README 및 changelog**

추가 a concise `redis` 패키지 index row to `README.md` 및 `README.ko.md`.
추가 `[Unreleased]` changelog bullet:

```markdown
- Add `redis` foundation package with key, owner-token, lease script, TTL, and redacted Redis operation error primitives.
```

- [ ] **단계 4: 검증 docs against source**

실행:

```bash
go test -count=1 ./redis -run Example
rg -n "github.com/bluetape4k/bluetape-go/redis|btredis|OwnerToken|KeyBuilder|CompareAndDelete|CompareAndExtend|OpError|0.19.0|#569" redis README.md README.ko.md CHANGELOG.md
rg -n "context.WithTimeout|false, nil|ownership drift|indeterminate|cleanup|no migration|rollback|OpLabels|redis-key:" redis/README.md redis/README.ko.md redis/example_test.go
git diff --check
```

예상: PASS / matching docs. Also manually checklist both locale READMEs for
the same boundary, non-goals, key preservation, token secrecy, timeout,
cancellation, ownership drift, script/client 오류, cleanup, rollback, 및
없음-migration sections; keyword presence alone is 아님 sufficient.

## 작업 6: Final 검증, Lessons, And 리뷰 증거

complexity: 보통
Required skill: `bluetape-go-patterns`

**파일:**
- 생성: `docs/lessons/2026-07-10-issue-569-redis-foundation.md`
- 생성: `docs/review/2026-07-10-issue-569-redis-foundation-review.md`

- [ ] **단계 1: 실행 targeted verification**

실행:

```bash
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
go test -count=1 ./redis -run Example
git diff --check
```

예상: PASS.

- [ ] **단계 2: 실행 full repo verification**

실행:

```bash
make ci
```

예상: PASS.

- [ ] **단계 3: 검증 없음 기존 Redis 패키지 migration occurred**

실행:

```bash
git diff --name-only origin/develop...HEAD
rg -n 'github.com/bluetape4k/bluetape-go/redis' lock/redis leader/redis ratelimit/redis probabilistic/redis cache/rediscoord cache/redisnear jwt || true
```

예상: changed files do 아님 include 기존 Redis 패키지 implementation files, 및 `rg` prints 없음 imports in those packages.

- [ ] **단계 4: 기록 lesson 및 code review artifact**

Lesson must capture:

- 공개 Redis foundation should reject nil contexts for external IO;
- owner tokens must be opaque/redacted by default;
- structural key segments 및 호출자-owned logical keys need separate API surfaces;
- post-dispatch Redis script cancellation has indeterminate commit state.

리뷰 artifact must include:

- reviewed scope 및 baseline commit;
- verification commands 및 results;
- 7-Tier P0/P1/P2/P3 table;
- explicit `P0=0 P1=0`;
- remaining risks 또는 fol낮음-up issues.

- [ ] **단계 5: 검증 issue metadata 전에 PR**

실행:

```bash
gh issue view 569 --json assignees,milestone,labels,state,title,url
```

예상:

- state `OPEN`;
- assignee `debop`;
- milestone `0.19.0`;
- labels match issue scope.

## 단계 3 Checklist Completion Report

| Item | 상태 | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-07-10-issue-569-redis-foundation-plan.md`. |
| All tasks have complexity labels | Done | Tasks 1-4 높음, Tasks 5-6 보통. |
| `bluetape-go-patterns` applied to every code-bearing task | Done | Each task declares required skill. |
| Plan code/테스트 snippets conform to Go patterns | Done | Context, `%w`, 오류.Is/As, race, 및 Testcontainers requirements included. |
| Thread/coroutine helper applicability | N/A | Go repo; race/stress uses Go `testing` 및 `go test -race`, 아님 bluetape4k JUnit helpers. |
| Tests 및 verification tasks included | Done | Unit, fake-scripter, Testcontainers, race, example, `make ci`. |
| Multilingual README 및 contributor docs included | Done | README locale set, CHANGELOG, lessons, review artifact. |
| Risky ordering/dependency assumptions explicit | Done | Current-code assumptions 및 없음-migration check included. |
| Spec + plan committed 전에 implementation | Pending | 커밋 후 단계 3-R plan review convergence. |
