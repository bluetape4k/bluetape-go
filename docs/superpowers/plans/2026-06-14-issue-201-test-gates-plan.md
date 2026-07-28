# Issue #201 Test Gate Upgrade Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade issue #201 verification gates by adding bounded Testcontainers cleanup tests and helper code, strengthening `testing/concurrency` edge coverage, and recording the resulting stress/race/CI evidence.

**Architecture:** Add one small internal helper package under `testcontainers/internal/cleanup` so all Testcontainers wrappers share bounded termination logic without exposing a new public API. Keep `testing/concurrency` behavior unchanged unless tests expose a defect; use additional tests to lock invalid option, nil task, caller cancellation, timeout, and bounded parallelism contracts.

**Tech Stack:** Go 1.26.3, `testing`, `context`, `time`, repo-local `testing/concurrency.GoroutineStressTester`, Testcontainers module wrappers, `$bluetape4k-workflow`, `$bluetape-go-patterns`, `$bluetape4k-diagram`.

---

## File Structure

- Create:
  - `testcontainers/internal/cleanup/cleanup.go`
  - `testcontainers/internal/cleanup/cleanup_test.go`
  - `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`
  - `docs/lessons/2026-06-14-issue-201-test-gates.md`
  - `docs/superpowers/pr/2026-06-14-issue-201-test-gates-pr-body.md`
- Modify:
  - `testcontainers/kafka/kafka.go`
  - `testcontainers/mysql/mysql.go`
  - `testcontainers/nats/nats.go`
  - `testcontainers/postgres/postgres.go`
  - `testcontainers/redis/redis.go`
  - `testing/concurrency/testers_test.go`

## Task 1: RED - Add Bounded Cleanup Tests

**Files:**
- Create: `testcontainers/internal/cleanup/cleanup_test.go`

- [ ] **Step 1: Write failing cleanup tests**

Create `testcontainers/internal/cleanup/cleanup_test.go`:

```go
package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTerminateUsesBoundedContextEvenWhenParentIsCancelled(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "kept"))
	cancel()

	terminator := &capturingTerminator{}
	if err := Terminate(parent, 50*time.Millisecond, terminator); err != nil {
		t.Fatalf("Terminate failed: %v", err)
	}

	if terminator.ctx == nil {
		t.Fatal("expected terminator context")
	}
	if err := terminator.ctx.Err(); err != nil {
		t.Fatalf("cleanup context should ignore parent cancellation, got %v", err)
	}
	if got := terminator.ctx.Value(contextKey{}); got != "kept" {
		t.Fatalf("expected context value to be preserved, got %v", got)
	}
	deadline, ok := terminator.ctx.Deadline()
	if !ok {
		t.Fatal("expected cleanup context deadline")
	}
	if time.Until(deadline) > time.Second {
		t.Fatalf("expected bounded cleanup deadline, got %s", time.Until(deadline))
	}
}

func TestTerminateReturnsTimeoutWhenContainerDoesNotStop(t *testing.T) {
	terminator := blockingTerminator{}

	err := Terminate(context.Background(), 10*time.Millisecond, terminator)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestTerminateRejectsNilTerminator(t *testing.T) {
	err := Terminate(context.Background(), time.Second, nil)

	if err == nil {
		t.Fatal("expected nil terminator error")
	}
}

type contextKey struct{}

type capturingTerminator struct {
	ctx context.Context
}

func (t *capturingTerminator) Terminate(ctx context.Context) error {
	t.ctx = ctx
	return nil
}

type blockingTerminator struct{}

func (blockingTerminator) Terminate(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
go test -count=1 ./testcontainers/internal/cleanup
```

Expected:

```text
FAIL: undefined: Terminate
```

## Task 2: GREEN - Implement Internal Cleanup Helper

**Files:**
- Create: `testcontainers/internal/cleanup/cleanup.go`

- [ ] **Step 1: Implement minimal cleanup helper**

Create `testcontainers/internal/cleanup/cleanup.go`:

```go
package cleanup

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const DefaultTerminateTimeout = 10 * time.Second

// Terminator is the subset of Testcontainers containers used during cleanup.
type Terminator interface {
	Terminate(context.Context) error
}

// Terminate stops a container with a bounded context that ignores parent
// cancellation while preserving parent context values.
func Terminate(parent context.Context, timeout time.Duration, terminator Terminator) error {
	if terminator == nil {
		return fmt.Errorf("terminator must not be nil")
	}
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = DefaultTerminateTimeout
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	return terminator.Terminate(ctx)
}

// Register wires bounded Testcontainers cleanup to tb.Cleanup.
func Register(tb testing.TB, parent context.Context, name string, terminator Terminator) {
	tb.Helper()
	tb.Cleanup(func() {
		tb.Helper()
		if err := Terminate(parent, DefaultTerminateTimeout, terminator); err != nil {
			tb.Fatalf("terminate %s container: %v", name, err)
		}
	})
}
```

- [ ] **Step 2: Verify GREEN**

Run:

```bash
go test -count=1 ./testcontainers/internal/cleanup
```

Expected:

```text
ok  	github.com/bluetape4k/bluetape-go/testcontainers/internal/cleanup
```

## Task 3: Wire Testcontainers Wrappers To Bounded Cleanup

**Files:**
- Modify: `testcontainers/kafka/kafka.go`
- Modify: `testcontainers/mysql/mysql.go`
- Modify: `testcontainers/nats/nats.go`
- Modify: `testcontainers/postgres/postgres.go`
- Modify: `testcontainers/redis/redis.go`

- [ ] **Step 1: Replace unbounded cleanup in Redis**

In `testcontainers/redis/redis.go`, add the import:

```go
cleanup "github.com/bluetape4k/bluetape-go/testcontainers/internal/cleanup"
```

Replace:

```go
tb.Cleanup(func() {
	if err := container.Terminate(context.Background()); err != nil {
		tb.Fatalf("terminate redis container: %v", err)
	}
})
```

with:

```go
cleanup.Register(tb, ctx, "redis", container)
```

- [ ] **Step 2: Replace unbounded cleanup in Kafka**

In `testcontainers/kafka/kafka.go`, add the import:

```go
cleanup "github.com/bluetape4k/bluetape-go/testcontainers/internal/cleanup"
```

Replace the `t.Cleanup` block that calls `container.Terminate(context.Background())` with:

```go
cleanup.Register(t, ctx, "kafka", container)
```

- [ ] **Step 3: Replace unbounded cleanup in MySQL**

In `testcontainers/mysql/mysql.go`, add the same cleanup import and replace the
`t.Cleanup` block with:

```go
cleanup.Register(t, ctx, "mysql", container)
```

- [ ] **Step 4: Replace unbounded cleanup in NATS**

In `testcontainers/nats/nats.go`, add the same cleanup import and replace the
`t.Cleanup` block with:

```go
cleanup.Register(t, ctx, "nats", container)
```

- [ ] **Step 5: Replace unbounded cleanup in Postgres**

In `testcontainers/postgres/postgres.go`, add the same cleanup import and
replace the `t.Cleanup` block with:

```go
cleanup.Register(t, ctx, "postgres", container)
```

- [ ] **Step 6: Run targeted Testcontainers tests**

Run:

```bash
go test -count=1 ./testcontainers/internal/cleanup ./testcontainers/kafka ./testcontainers/mysql ./testcontainers/nats ./testcontainers/postgres ./testcontainers/redis
```

Expected:

```text
ok  	github.com/bluetape4k/bluetape-go/testcontainers/internal/cleanup
ok  	github.com/bluetape4k/bluetape-go/testcontainers/kafka
ok  	github.com/bluetape4k/bluetape-go/testcontainers/mysql
ok  	github.com/bluetape4k/bluetape-go/testcontainers/nats
ok  	github.com/bluetape4k/bluetape-go/testcontainers/postgres
ok  	github.com/bluetape4k/bluetape-go/testcontainers/redis
```

## Task 4: Add `testing/concurrency` Edge Coverage

**Files:**
- Modify: `testing/concurrency/testers_test.go`

- [ ] **Step 1: Add edge-case tests for invalid options and caller cancellation**

Append these tests to `testing/concurrency/testers_test.go`:

```go
func TestGoroutineStressTesterRejectsInvalidOptionsAndTasks(t *testing.T) {
	t.Run("missing task", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{})
		if _, err := tester.Run(context.Background()); err == nil {
			t.Fatal("expected missing task error")
		}
	})

	t.Run("nil task", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{})
		if _, err := tester.Run(context.Background(), nil); err == nil {
			t.Fatal("expected nil task error")
		}
	})

	t.Run("negative workers", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: -1})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative workers error")
		}
	})

	t.Run("negative rounds", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{RoundsPerTask: -1})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative rounds error")
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Timeout: -time.Second})
		if _, err := tester.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
			t.Fatal("expected negative timeout error")
		}
	})
}

func TestGoroutineStressTesterPropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 1})
	report, err := tester.Run(ctx, func(context.Context) error {
		t.Fatal("task should not run after caller cancellation")
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if report.Started != 0 {
		t.Fatalf("expected no started tasks, got %+v", report)
	}
}
```

- [ ] **Step 2: Run the package tests**

Run:

```bash
go test -count=1 ./testing/concurrency
go test -race -count=1 ./testing/concurrency
```

Expected:

```text
ok  	github.com/bluetape4k/bluetape-go/testing/concurrency
```

## Task 5: Review Stress Helper Usage And Record Evidence

**Files:**
- Create: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-6r-code-review.md`

- [ ] **Step 1: Capture stress helper usage**

Run:

```bash
rg -n "NewGoroutineStressTester|NewAsyncJobTester" batch cache concurrency id jwt leader lock measure money probabilistic ratelimit state testing workflow workreport
```

Expected:

```text
Matches include batch, cache, id, jwt, leader/redis, lock/redis, measure, money, probabilistic, ratelimit, state, testing/concurrency, workflow, and workreport.
```

- [ ] **Step 2: Record Step 6-R review after implementation**

Write the Step 6-R artifact after code changes and validation. The review must
use the six-lane plus main integration shape and include exact:

```text
P0=0 P1=0
```

## Task 6: Full Verification Gates

**Files:**
- No source changes.

- [ ] **Step 1: Run targeted tests**

Run:

```bash
go test -count=1 ./testing/concurrency ./testcontainers/internal/cleanup ./testcontainers/kafka ./testcontainers/mysql ./testcontainers/nats ./testcontainers/postgres ./testcontainers/redis
```

Expected: all listed packages pass.

- [ ] **Step 2: Run targeted race tests**

Run:

```bash
go test -race -count=1 ./testing/concurrency ./testcontainers/internal/cleanup
```

Expected: both packages pass under the race detector.

- [ ] **Step 3: Run repository tests**

Run:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
make ci
git diff --check
```

Expected: all commands exit 0. If Docker or CI tooling fails from the local
environment, capture the exact failing package/command and rerun the strongest
next-best targeted gate before reporting.

## Task 7: Lessons, Commit, PR, And PR Review

**Files:**
- Create: `docs/lessons/2026-06-14-issue-201-test-gates.md`
- Create: `docs/superpowers/pr/2026-06-14-issue-201-test-gates-pr-body.md`
- Create: `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`

- [ ] **Step 1: Write lesson**

Create `docs/lessons/2026-06-14-issue-201-test-gates.md` with:

```markdown
# Issue #201 Test Gate Lessons

## What changed

- Testcontainers cleanup now uses a bounded context through an internal helper.
- `testing/concurrency` edge cases are explicitly covered.

## What to repeat

- Use `GoroutineStressTester` for shared-state/goroutine stress claims.
- Use `AsyncJobTester` for context cancellation/deadline claims.
- Keep Testcontainers-backed packages serial when Docker resources are shared.

## Evidence

- Targeted tests:
- Targeted race:
- Repo tests:
- CI:
```

- [ ] **Step 2: Commit implementation**

Use Lore protocol. The commit message intent line should explain why the gate
was hardened, not just list files changed.

- [ ] **Step 3: Create PR body file**

Write `docs/superpowers/pr/2026-06-14-issue-201-test-gates-pr-body.md` with
English sections for background, work done, validation, review notes, and final
`## DoD Status`.

- [ ] **Step 4: Create PR and verify live body**

Run:

```bash
gh pr create --repo bluetape4k/bluetape-go --base develop --head issue-201-test-gates --title "test: upgrade failure cancellation race and cleanup gates" --body-file docs/superpowers/pr/2026-06-14-issue-201-test-gates-pr-body.md
pr_number=$(gh pr view --repo bluetape4k/bluetape-go --head issue-201-test-gates --json number --jq '.number')
gh pr view "$pr_number" --repo bluetape4k/bluetape-go --json body --jq '.body'
```

Expected: live PR body is non-empty and the final Markdown `##` heading is:

```markdown
## DoD Status
```

- [ ] **Step 5: Run Step 7-R PR review**

Write `docs/superpowers/reviews/2026-06-14-issue-201-test-gates-step-7r-pr-review.md`
with the same six-lane plus main integration shape and `P0=0 P1=0`. Add a PR
comment or formal review entry when GitHub permissions allow it.
