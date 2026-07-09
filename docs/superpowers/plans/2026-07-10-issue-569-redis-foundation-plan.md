# Issue 569 Redis Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the public `github.com/bluetape4k/bluetape-go/redis` package with safe Redis key, owner-token, lease script, TTL, and operational error primitives.

**Architecture:** Implement a narrow package named `btredis` that reuses `go-redis/v9` only for script execution and does not wrap the Redis client. Existing Redis-backed packages are not migrated in this issue; #570 owns migration after parity tests and benchmarks.

**Tech Stack:** `go.mod` declares Go 1.26.3, current local toolchain is `go1.26.5`, `github.com/redis/go-redis/v9`, existing `testcontainers/redis` fixture, table-driven `testing`, TDD, `go test`, `go test -race`, `make ci`.

---

## Inputs

- Spec: `docs/superpowers/specs/2026-07-10-issue-569-redis-foundation-spec.md`
- Spec review: `docs/superpowers/reviews/2026-07-10-issue-569-redis-foundation-spec-review.md`
- Issue: #569
- Milestone: `0.19.0`

## File Map

| Path | Responsibility |
|---|---|
| `redis/doc.go` | Package overview and safety boundary. |
| `redis/errors.go` | Sentinels, `OpError`, redacted error construction. |
| `redis/key.go` | `Key`, `KeyBuilder`, redacted key id helpers. |
| `redis/token.go` | Opaque `OwnerToken` generation, parsing, validation, redacted formatting. |
| `redis/ttl.go` | TTL validation and millisecond conversion. |
| `redis/lease.go` | Immutable `Lease` and validation. |
| `redis/script.go` | Package-level compare-delete / compare-extend Lua scripts and helpers. |
| `redis/*_test.go` | Unit, fake-scripter, integration, stress, and examples. |
| `redis/README.md`, `redis/README.ko.md` | Public package docs and operator guidance. |
| `README.md`, `README.ko.md` | Root package index updates. |
| `CHANGELOG.md` | `[Unreleased]` package addition. |
| `docs/lessons/2026-07-10-issue-569-redis-foundation.md` | Session lesson before PR. |
| `docs/review/2026-07-10-issue-569-redis-foundation-review.md` | Step 6-R final review artifact. |

## Current-Code Assumptions To Recheck Before Step 4

- `go.mod` still contains `github.com/redis/go-redis/v9`.
- `testcontainers/redis.Start(ctx, tb)` still returns a Redis address.
- No top-level `redis/` package exists.
- Existing Redis packages are not edited except docs/root indexes in this issue.

## Task 1: Package Scaffold And Token Contract

complexity: high
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `redis/doc.go`
- Create: `redis/token.go`
- Create: `redis/token_test.go`

- [ ] **Step 1: Write failing token tests**

Create `redis/token_test.go`:

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

- [ ] **Step 2: Verify token tests fail for missing package**

Run:

```bash
go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'
```

Expected: FAIL because `./redis` does not yet exist or token symbols are undefined.

- [ ] **Step 3: Implement minimal token and package docs**

Create `redis/doc.go`:

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

Create `redis/token.go`:

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

- [ ] **Step 4: Verify token tests pass**

Run:

```bash
go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'
```

Expected: PASS.

## Task 2: Key Builder, Redaction, TTL, And OpError

complexity: high
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `redis/key.go`
- Create: `redis/ttl.go`
- Create: `redis/errors.go`
- Create: `redis/key_test.go`
- Create: `redis/ttl_test.go`
- Create: `redis/errors_test.go`

- [ ] **Step 1: Write failing key, TTL, and error tests**

Create `redis/key_test.go`, `redis/ttl_test.go`, and `redis/errors_test.go` with table-driven tests covering:

- structural segments reject `""`, `" "`, `"a:b"`, `"{bad}"`;
- `NewKeyBuilder("bluetape:probabilistic:bloom:v1")` accepts a colon-delimited package prefix by validating each prefix part as a structural segment;
- invalid prefixes such as `""`, `" "`, `"bad::prefix"`, `"bad:{tag}"`, and `"bad: part"` return `ErrInvalidKey`;
- `LogicalKey(" lock key ")`, `LogicalKey("tenant:user:{caller}:a:b")`, and `LogicalKey("line\nkey")` preserve exact bytes;
- `WithHashTag("test:package:case")` preserves the colon-bearing hash tag so #570 can keep existing `probabilistic/redis` namespace keys;
- `WithHashTag("")`, `WithHashTag("{bad}")`, and `WithHashTag("bad{tag}")` return `ErrInvalidHashTag`;
- `StructuralKey("bits")` and `StructuralKey("config")` preserve same hash tag;
- `KeyBuilder{}` methods return `ErrInvalidKey` or `ErrInvalidHashTag` and do not panic;
- `RedactedKeyID("tenant:secret")` is deterministic, matches `^redis-key:[0-9a-f]{24}$`, and does not contain `tenant:secret`;
- `TTLMillis("lease ttl", time.Millisecond)` returns `1`;
- zero, negative, and sub-millisecond TTLs return `ErrInvalidTTL`;
- `NewOpError(OpLabels{Family: "redis lock", Operation: "release"}, "raw:key", context.DeadlineExceeded)` returns an `OpError`, wraps the cause, and does not print `raw:key`;
- `NewOpErrorWithRedactedKey` preserves a valid already-redacted key id;
- `NewOpErrorWithRedactedKey` rejects raw-key-looking values and malformed IDs with `ErrInvalidKey` without echoing the rejected value;
- `NewOpError` and `NewOpErrorWithRedactedKey` do not include raw keys or owner tokens in `Error()` even when the wrapped cause message contains them;
- `OpLabels` rejects empty, overlong, or delimiter-heavy family/operation labels without echoing rejected label text;
- `Key.String()` and `Key.GoString()` return `RedactedID`, not `Value`.

Use this exact command:

```bash
go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'
```

Expected: FAIL because the new symbols are not implemented.

- [ ] **Step 2: Implement key builder, TTL helpers, and errors**

Implement:

```go
var (
	ErrInvalidKey     = errors.New("redis: invalid key")
	ErrInvalidHashTag = errors.New("redis: invalid hash tag")
	ErrInvalidTTL     = errors.New("redis: invalid ttl")
)
```

Implementation constraints:

- `RedactedKeyID` uses SHA-256 and the first 12 bytes encoded as hex.
- `ValidateRedactedKeyID` accepts only `redis-key:<24 lowercase hex>`.
- `KeyBuilder` stores prefix parts, optional hash tag, and structural parts.
- `NewKeyBuilder(prefix)` splits a colon-delimited package prefix, validates each prefix part as a structural segment, and stores the original colon-delimited prefix shape through its parts.
- `Structural(parts...)` returns a copied builder with appended structural parts.
- `WithHashTag(tag)` validates non-empty and no braces, then stores tag verbatim; `:` is allowed because existing `probabilistic/redis` namespaces use colon-bearing hash tags.
- `StructuralKey(parts...)` appends structural suffixes.
- `LogicalKey(logicalKey)` validates non-blank by `strings.TrimSpace` but appends the original string.
- `Key.String()` and `Key.GoString()` return `RedactedID`; `Key.Value` remains sensitive Redis command input.
- `ValidateTTL` rejects `ttl < time.Millisecond` and `ttl <= 0`.
- `TTLMillis` calls `ValidateTTL` and returns `ttl.Milliseconds()`.
- `OpLabels` groups low-cardinality `Family` and `Operation` values to avoid adjacent positional key/label strings. Label validation rejects empty, overlong, colon/braces, and whitespace-only values.
- `NewOpError` computes `RedactedKeyID(rawKey)` and never stores raw key.
- `NewOpErrorWithRedactedKey` validates the redacted key id before storing it.
- `OpError.Error()` prints family, operation, redacted key id, and cause type/category only; it must not include the wrapped cause text because provider errors may contain raw keys or tokens.
- `OpError.Is` delegates to the wrapped cause.

- [ ] **Step 3: Verify key, TTL, and error tests pass**

Run:

```bash
go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'
```

Expected: PASS.

## Task 3: Lease And Script Helpers With Fake Scripter No-Dispatch Tests

complexity: high
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `redis/lease.go`
- Create: `redis/script.go`
- Create: `redis/lease_test.go`
- Create: `redis/script_test.go`

- [ ] **Step 1: Write failing lease and fake-scripter tests**

Tests must prove:

- `NewLease("", validToken)` and `NewLease("key", OwnerToken{})` return sentinel-compatible errors.
- `Lease{}` is invalid.
- `CompareAndDelete(nil, fake, lease, "redis test")` returns a validation error and fake call count remains 0.
- `CompareAndDelete(ctx, nil, lease, "redis test")` and `CompareAndExtend(ctx, nil, lease, ttl, "redis test")` return validation errors and do not panic.
- a typed nil client returns a validation error when detectable without unsafe reflection.
- pre-canceled context returns `context.Canceled` and fake call count remains 0.
- invalid lease returns validation error and fake call count remains 0.
- invalid TTL for `CompareAndExtend` returns `ErrInvalidTTL` and fake call count remains 0.
- fake success returning integer `1` maps to `true`.
- fake success returning integer `0` maps to `false`.
- fake Redis error wraps as `OpError` and preserves the cause.
- fake Redis error with raw key/token text in the cause still produces an `OpError.Error()` string that contains only the redacted key id and no raw key/token.

Run:

```bash
go test -count=1 ./redis -run 'Lease|CompareAnd'
```

Expected: FAIL until `lease.go` and `script.go` exist.

- [ ] **Step 2: Implement immutable lease and package-level scripts**

Implement constraints:

- `Lease` has unexported `key string` and `token OwnerToken`.
- `NewLease` validates non-blank key without trimming stored key.
- `Lease.Key`, `Lease.RedactedKeyID`, `Lease.Token`, and `Lease.Validate` are value methods.
- `compareDeleteScript := redis.NewScript(...)` and `compareExtendScript := redis.NewScript(...)` are package-level vars.
- Script helpers validate in this order before Redis dispatch:
  1. `ctx != nil`
  2. `ctx.Err()`
  3. `client != nil` and detectable typed nil client rejection
  4. `lease.Validate()`
  5. for extend only, `TTLMillis("lease ttl", ttl)`
- Script helpers call `script.Run(ctx, client, []string{lease.Key()}, lease.Token().RedisValue(), ...)`.
- Script helpers build `OpLabels{Family: family, Operation: "compare-delete"}` and `OpLabels{Family: family, Operation: "compare-extend"}` for error construction.
- Result integer `1` is true, `0` is false; any parse error is wrapped in `OpError`.

- [ ] **Step 3: Verify fake-scripter tests pass**

Run:

```bash
go test -count=1 ./redis -run 'Lease|CompareAnd'
```

Expected: PASS.

## Task 4: Redis Testcontainers Integration And Stress

complexity: high
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `redis/integration_test.go`
- Create or update: `redis/script_test.go`

- [ ] **Step 1: Write failing Redis integration tests**

Use `redistestcontainer.Start(ctx, t)` from `github.com/bluetape4k/bluetape-go/testcontainers/redis` and `redis.NewClient`.

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

Each test must:

- use `context.WithTimeout`;
- use unique keys from `t.Name()`;
- call `t.Cleanup` to delete keys and close client;
- not call `t.Parallel` in Testcontainers-backed tests or in parent subtests;
- run Testcontainers-backed Redis tests serially through package command, not parallel external invocations.
- assert first-run helper execution succeeds against a fresh real client, relying on go-redis `Script.Run` to handle `EVALSHA`/`EVAL` fallback instead of reimplementing fallback in this package.
- stress tests use at least 8 workers and 32 iterations per worker on a shared key, interleave stale and later owners, and assert the later owner value remains intact; extend stress also asserts stale owners do not increase the later owner's TTL beyond the expected owner TTL window.

Run:

```bash
go test -p 1 -count=1 ./redis -run 'CompareAnd(Delete|Extend)'
```

Expected: FAIL until integration support is complete.

- [ ] **Step 2: Implement missing integration support**

Only adjust production code if integration tests expose a real contract gap. Do not add retry loops or provider migration code.

- [ ] **Step 3: Verify integration and race**

Run:

```bash
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
```

Expected: PASS.

## Task 5: Examples, README Locale Set, Root Index, And Changelog

complexity: medium
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `redis/example_test.go`
- Create: `redis/README.md`
- Create: `redis/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add compile-checked examples**

Create examples:

- `ExampleNewOwnerToken`
- `ExampleKeyBuilder_LogicalKey`
- `ExampleKeyBuilder_StructuralKey`
- `ExampleCompareAndDelete`
- `ExampleOpError`

Examples must not print raw owner token values. Use `String()` or redacted key IDs only.
`ExampleCompareAndDelete` must show `context.WithTimeout` and document that
`(false, nil)` means ownership drift. `ExampleOpError` must use `OpLabels` and
must not pass raw key material as family or operation labels.

Run:

```bash
go test -count=1 ./redis -run Example
```

Expected: PASS after examples compile.

- [ ] **Step 2: Write package READMEs**

`redis/README.md` and `redis/README.ko.md` must cover:

- import path and package name;
- non-goals: no generic Redis facade, no migration of existing packages in #569;
- key preservation and structural/logical split;
- hash-tags are same-slot helpers, not tenant isolation;
- owner-token secrecy and redacted formatting;
- `context.WithTimeout` examples and post-dispatch cancellation indeterminate state;
- `(false, nil)` means ownership drift, not an infrastructure error;
- `OpError` and redacted diagnostics;
- Redis script/client error triage and the fact that `OpError.Error()` is
  sanitized while the cause remains available through `errors.Is` / `errors.As`;
- cleanup guidance for partial failures;
- rollback/no-migration behavior for #569;
- test commands.

- [ ] **Step 3: Update root README and changelog**

Add a concise `redis` package index row to `README.md` and `README.ko.md`.
Add `[Unreleased]` changelog bullet:

```markdown
- Add `redis` foundation package with key, owner-token, lease script, TTL, and redacted Redis operation error primitives.
```

- [ ] **Step 4: Verify docs against source**

Run:

```bash
go test -count=1 ./redis -run Example
rg -n "github.com/bluetape4k/bluetape-go/redis|btredis|OwnerToken|KeyBuilder|CompareAndDelete|CompareAndExtend|OpError|0.19.0|#569" redis README.md README.ko.md CHANGELOG.md
rg -n "context.WithTimeout|false, nil|ownership drift|indeterminate|cleanup|no migration|rollback|OpLabels|redis-key:" redis/README.md redis/README.ko.md redis/example_test.go
git diff --check
```

Expected: PASS / matching docs. Also manually checklist both locale READMEs for
the same boundary, non-goals, key preservation, token secrecy, timeout,
cancellation, ownership drift, script/client error, cleanup, rollback, and
no-migration sections; keyword presence alone is not sufficient.

## Task 6: Final Verification, Lessons, And Review Evidence

complexity: medium
Required skill: `bluetape-go-patterns`

**Files:**
- Create: `docs/lessons/2026-07-10-issue-569-redis-foundation.md`
- Create: `docs/review/2026-07-10-issue-569-redis-foundation-review.md`

- [ ] **Step 1: Run targeted verification**

Run:

```bash
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
go test -count=1 ./redis -run Example
git diff --check
```

Expected: PASS.

- [ ] **Step 2: Run full repo verification**

Run:

```bash
make ci
```

Expected: PASS.

- [ ] **Step 3: Verify no existing Redis package migration occurred**

Run:

```bash
git diff --name-only origin/develop...HEAD
rg -n 'github.com/bluetape4k/bluetape-go/redis' lock/redis leader/redis ratelimit/redis probabilistic/redis cache/rediscoord cache/redisnear jwt || true
```

Expected: changed files do not include existing Redis package implementation files, and `rg` prints no imports in those packages.

- [ ] **Step 4: Record lesson and code review artifact**

Lesson must capture:

- public Redis foundation should reject nil contexts for external IO;
- owner tokens must be opaque/redacted by default;
- structural key segments and caller-owned logical keys need separate API surfaces;
- post-dispatch Redis script cancellation has indeterminate commit state.

Review artifact must include:

- reviewed scope and baseline commit;
- verification commands and results;
- 7-Tier P0/P1/P2/P3 table;
- explicit `P0=0 P1=0`;
- remaining risks or follow-up issues.

- [ ] **Step 5: Verify issue metadata before PR**

Run:

```bash
gh issue view 569 --json assignees,milestone,labels,state,title,url
```

Expected:

- state `OPEN`;
- assignee `debop`;
- milestone `0.19.0`;
- labels match issue scope.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-07-10-issue-569-redis-foundation-plan.md`. |
| All tasks have complexity labels | Done | Tasks 1-4 high, Tasks 5-6 medium. |
| `bluetape-go-patterns` applied to every code-bearing task | Done | Each task declares required skill. |
| Plan code/test snippets conform to Go patterns | Done | Context, `%w`, errors.Is/As, race, and Testcontainers requirements included. |
| Thread/coroutine helper applicability | N/A | Go repo; race/stress uses Go `testing` and `go test -race`, not bluetape4k JUnit helpers. |
| Tests and verification tasks included | Done | Unit, fake-scripter, Testcontainers, race, examples, `make ci`. |
| Multilingual README and contributor docs included | Done | README locale set, CHANGELOG, lessons, review artifact. |
| Risky ordering/dependency assumptions explicit | Done | Current-code assumptions and no-migration check included. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R plan review convergence. |
