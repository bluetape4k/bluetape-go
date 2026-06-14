# Locale Currency Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `money.CurrencyByLocale` from a small hand map to deterministic CLDR-backed current-region currency lookup.

**Architecture:** Keep the public API unchanged. Parse BCP47 tags with `golang.org/x/text/language`, extract only an explicit region from the source tag, enumerate current legal tender currencies with `golang.org/x/text/currency.Query`, and return a currency only when the result is exactly one current tender unit.

**Tech Stack:** Go 1.25, `golang.org/x/text/language`, `golang.org/x/text/currency`, `github.com/govalues/money`, repo-local `testing/concurrency.GoroutineStressTester`, `$bluetape-go-patterns`, `$bluetape4k-diagram`.

---

## File Structure

- Modify `money/currency.go`
  - Replace `localeRegion` and `regionCurrencies` with BCP47 parsing and CLDR tender lookup helpers.
  - Keep `CurrencyByLocale(tag string) (Currency, error)` as the only public locale API.
- Modify `money/currency_test.go`
  - Expand table-driven locale tests.
  - Add explicit no-tender and multi-tender rejection cases.
- Create `money/currency_concurrency_test.go`
  - Add `GoroutineStressTester` coverage for concurrent success and failure lookup paths.
- Modify `money/README.md`
  - Replace #179 deferred text with active `CurrencyByLocale` behavior.
  - Embed or reference the locale mapping diagram near locale behavior.
- Modify `money/README.ko.md`
  - Keep Korean README parity with the English README.
- Modify `README.md` and `README.ko.md` only if root package summary needs the fuller locale mapping wording.
- Modify `CHANGELOG.md`
  - Add an Unreleased `money` bullet for CLDR-backed locale currency mapping.
- Keep existing diagram files:
  - `scripts/generate-money-locale-currency-diagram.mjs`
  - `docs/images/readme-diagrams/money-locale-currency-resolution-flow.*`
- Create review/lesson artifacts as required:
  - `docs/superpowers/reviews/2026-06-14-issue-179-locale-currency-mapping-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-179-locale-currency-mapping-step-6r-code-review.md`
  - `docs/lessons/2026-06-14-issue-179-locale-currency-mapping.md`

## Task 1: Write Locale Contract Tests First

**Files:**
- Modify: `money/currency_test.go`

- [x] **Step 1: Replace `TestCurrencyByLocale` table with current-region coverage**

Use this test shape:

```go
func TestCurrencyByLocale(t *testing.T) {
	tests := []struct {
		tag  string
		want Currency
	}{
		{tag: "ko-KR", want: KRW},
		{tag: "en_US", want: USD},
		{tag: "ko_KR", want: KRW},
		{tag: "en-us", want: USD},
		{tag: "ja-JP", want: JPY},
		{tag: "zh-CN", want: CNY},
		{tag: "de-DE", want: EUR},
		{tag: "fr_FR", want: EUR},
		{tag: "it-IT", want: EUR},
		{tag: "es-ES", want: EUR},
		{tag: "nl-NL", want: EUR},
		{tag: "pt-PT", want: EUR},
		{tag: "fi-FI", want: EUR},
		{tag: "ie-IE", want: EUR},
		{tag: "at-AT", want: EUR},
		{tag: "be-BE", want: EUR},
		{tag: "en-GB", want: MustParseCurrency("GBP")},
		{tag: "fr-CA", want: MustParseCurrency("CAD")},
		{tag: "en-AU", want: MustParseCurrency("AUD")},
		{tag: "pt-BR", want: MustParseCurrency("BRL")},
		{tag: "hi-IN", want: MustParseCurrency("INR")},
		{tag: "es-MX", want: MustParseCurrency("MXN")},
		{tag: "zh-Hant-TW", want: MustParseCurrency("TWD")},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := CurrencyByLocale(tt.tag)
			if err != nil {
				t.Fatalf("CurrencyByLocale(%q) failed: %v", tt.tag, err)
			}
			if got != tt.want {
				t.Fatalf("CurrencyByLocale(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}
```

- [x] **Step 2: Replace unsupported test with explicit policy cases**

Use this test shape:

```go
func TestCurrencyByLocaleRejectsUnsupportedTags(t *testing.T) {
	for _, tag := range []string{
		"ko",
		"",
		"und",
		"en-001",
		"en-QM",
		"en-AQ",
		"es-PA",
		"en-u-cu-usd",
	} {
		t.Run(tag, func(t *testing.T) {
			_, err := CurrencyByLocale(tag)
			if !errors.Is(err, ErrInvalidCurrency) {
				t.Fatalf("expected ErrInvalidCurrency for %q, got %v", tag, err)
			}
		})
	}
}
```

- [x] **Step 3: Run failing targeted tests**

Run:

```bash
go test -count=1 ./money -run 'TestCurrencyByLocale'
```

Expected before implementation:

- New common regions fail because they are not in `regionCurrencies`.
- No-tender/multi-tender policy cases may fail because the old helper only checks a small map and does not query CLDR tender counts.

## Task 2: Implement Explicit-Region CLDR Lookup

**Files:**
- Modify: `money/currency.go`

- [x] **Step 1: Update imports**

Change the import block to include x/text packages:

```go
import (
	"errors"
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
	xcurrency "golang.org/x/text/currency"
	"golang.org/x/text/language"
)
```

- [x] **Step 2: Replace `CurrencyByLocale` implementation**

Use this implementation:

```go
// CurrencyByLocale 은 BCP47 locale tag의 명시적 현재 지역 통화를 반환합니다.
func CurrencyByLocale(tag string) (Currency, error) {
	normalized := normalizeLocaleTag(tag)
	if normalized == "" {
		return Currency{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, tag)
	}
	region, ok := explicitLocaleRegion(normalized)
	if !ok {
		return Currency{}, fmt.Errorf("%w: locale %q has no explicit region", ErrInvalidCurrency, tag)
	}
	if _, err := language.Parse(normalized); err != nil {
		var valueErr language.ValueError
		if !errors.As(err, &valueErr) {
			return Currency{}, fmt.Errorf("%w: invalid locale %q: %w", ErrInvalidCurrency, tag, err)
		}
	}
	return currencyByRegion(region, tag)
}
```

- [x] **Step 3: Add helper functions under `CurrencyByLocale`**

Use these helpers:

```go
func normalizeLocaleTag(tag string) string {
	return strings.ReplaceAll(strings.TrimSpace(tag), "_", "-")
}

func explicitLocaleRegion(tag string) (language.Region, bool) {
	parts := strings.Split(tag, "-")
	if len(parts) < 2 {
		return language.Region{}, false
	}
	for _, part := range parts[1:] {
		if len(part) == 1 {
			return language.Region{}, false
		}
		if len(part) != 2 && len(part) != 3 {
			continue
		}
		region, err := language.ParseRegion(part)
		if err == nil {
			return region, true
		}
	}
	return language.Region{}, false
}

func currencyByRegion(region language.Region, originalTag string) (Currency, error) {
	iter := xcurrency.Query(xcurrency.Region(region))
	var code string
	count := 0
	for iter.Next() {
		if !iter.IsTender() {
			continue
		}
		count++
		if count == 1 {
			code = iter.Unit().String()
		}
	}
	if count != 1 {
		return Currency{}, fmt.Errorf("%w: locale %q maps to %d current tender currencies", ErrInvalidCurrency, originalTag, count)
	}
	curr, err := ParseCurrency(code)
	if err != nil {
		return Currency{}, fmt.Errorf("%w: locale %q maps to invalid currency %q: %w", ErrInvalidCurrency, originalTag, code, err)
	}
	return curr, nil
}
```

- [x] **Step 4: Remove old helper and map**

Delete:

```go
func localeRegion(tag string) (string, bool) { ... }

var regionCurrencies = map[string]string{ ... }
```

- [x] **Step 5: Run targeted tests**

Run:

```bash
go test -count=1 ./money -run 'TestCurrencyByLocale'
```

Expected: PASS.

## Task 3: Add Goroutine Stress Coverage

**Files:**
- Create: `money/currency_concurrency_test.go`

- [x] **Step 1: Create stress test file**

Use this file:

```go
package money

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestCurrencyByLocaleUsesGoroutineStressTester(t *testing.T) {
	const rounds = 256
	var operations atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       max(32, runtime.GOMAXPROCS(0)*4),
		RoundsPerTask: rounds,
		Timeout:       10 * time.Second,
	})

	report, err := tester.Run(context.Background(),
		func(context.Context) error {
			got, err := CurrencyByLocale("ko-KR")
			if err != nil {
				return err
			}
			if got != KRW {
				return errors.New("ko-KR currency mismatch")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			got, err := CurrencyByLocale("en-GB")
			if err != nil {
				return err
			}
			if got != MustParseCurrency("GBP") {
				return errors.New("en-GB currency mismatch")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			if _, err := CurrencyByLocale("es-PA"); !errors.Is(err, ErrInvalidCurrency) {
				return err
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			if _, err := CurrencyByLocale("en-u-cu-usd"); !errors.Is(err, ErrInvalidCurrency) {
				return err
			}
			operations.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != rounds*4 || report.Failures != 0 {
		t.Fatalf("unexpected stress report: %+v", report)
	}
	if operations.Load() != int64(rounds*4) {
		t.Fatalf("expected %d operations, got %d", rounds*4, operations.Load())
	}
}
```

- [x] **Step 2: Run stress and race tests**

Run:

```bash
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
```

Expected: PASS.

## Task 4: Update Documentation

**Files:**
- Modify: `money/README.md`
- Modify: `money/README.ko.md`
- Modify: `CHANGELOG.md`

- [x] **Step 1: Update `money/README.md` selection guide**

Replace the locale row with:

```markdown
| Locale-to-currency convenience | `CurrencyByLocale` | Uses explicit-region BCP47 tags and CLDR current legal tender data. Ambiguous or no-tender regions are rejected. |
```

- [x] **Step 2: Add locale section to `money/README.md` after the exchange-rate provider example**

Use this section:

````markdown
Locale currency mapping is a current-region convenience:

![money locale currency resolution flow](../docs/images/readme-diagrams/money-locale-currency-resolution-flow.png)

```go
currency, err := money.CurrencyByLocale("en-GB")
if err != nil {
    return err
}
_ = currency // GBP
```
````

- [x] **Step 3: Update English behavior bullets**

Add these bullets near existing currency behavior:

```markdown
- `CurrencyByLocale` requires an explicit BCP47 region and uses CLDR current
  legal tender data through `golang.org/x/text/currency`.
- Locale mapping is a current-region convenience, not an accounting, trading,
  tax, settlement, or legal-tender authority. Regions with no current tender or
  multiple current tender currencies return `ErrInvalidCurrency` so callers can
  choose explicitly.
```

- [x] **Step 4: Mirror the same content in `money/README.ko.md`**

Use Korean prose but keep code and diagram label in English:

````markdown
Locale currency mapping은 current-region convenience입니다.

![money locale currency resolution flow](../docs/images/readme-diagrams/money-locale-currency-resolution-flow.png)

```go
currency, err := money.CurrencyByLocale("en-GB")
if err != nil {
    return err
}
_ = currency // GBP
```
````

Add Korean behavior bullets:

```markdown
- `CurrencyByLocale`는 명시적 BCP47 region을 요구하고
  `golang.org/x/text/currency`의 CLDR current legal tender data를 사용합니다.
- Locale mapping은 current-region convenience이며 accounting, trading, tax,
  settlement, legal-tender 권위를 대체하지 않습니다. 현재 tender가 없거나 여러
  개인 region은 `ErrInvalidCurrency`를 반환하므로 caller가 명시적으로 선택해야
  합니다.
```

- [x] **Step 5: Update `CHANGELOG.md`**

Add under `[Unreleased]` / `### Added`:

```markdown
- `money.CurrencyByLocale` now resolves explicit-region BCP47 locale tags from
  CLDR current legal tender data, rejects missing/no-tender/multi-tender regions,
  and documents the locale mapping boundary with stress/race coverage and a
  diagram-backed README update.
```

## Task 5: Go Module And Formatting Cleanup

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify touched Go files

- [x] **Step 1: Format Go files**

Run:

```bash
gofmt -w money/currency.go money/currency_test.go money/currency_concurrency_test.go
```

- [x] **Step 2: Tidy module metadata**

Run:

```bash
go mod tidy
```

Expected:

- `golang.org/x/text` may move from indirect to direct in `go.mod`.
- No unexpected dependency additions beyond required direct import metadata.

- [x] **Step 3: Run whitespace check**

Run:

```bash
git diff --check
```

Expected: PASS.

## Task 6: Validation Gate

**Files:**
- All changed files

- [x] **Step 1: Diagram validation**

Run:

```bash
node scripts/generate-money-locale-currency-diagram.mjs
xmllint --noout docs/images/readme-diagrams/money-locale-currency-resolution-flow.svg
test -f docs/images/readme-diagrams/money-locale-currency-resolution-flow.png
```

Expected:

```text
money-locale-currency-resolution-flow: nodes=9 routes=8 segments=10 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 margins=L48/R48/T48/B48 titleGap=58
```

- [x] **Step 2: Targeted tests**

Run:

```bash
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
```

Expected: PASS.

- [x] **Step 3: Full repository tests**

Run:

```bash
go test -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Observed:

- `make fmt-check`, `make vet`, and `make lint` passed.
- `go test -count=1 -timeout=2m ./...` failed only in unchanged `jwt` cached-provider tests under local `go1.26.4 darwin/arm64`; PR CI remains the full-suite gate.
- `make tidy-check` must be run after the implementation commit because it compares module metadata against `HEAD`.

## Task 7: Review, Lessons, PR

**Files:**
- Create: `docs/superpowers/reviews/2026-06-14-issue-179-locale-currency-mapping-step-6r-code-review.md`
- Create: `docs/lessons/2026-06-14-issue-179-locale-currency-mapping.md`
- Create after PR: `docs/superpowers/reviews/2026-06-14-issue-179-locale-currency-mapping-step-7r-pr-review.md`

- [x] **Step 1: Run Step 6-R local 7-Tier review**

Review perspectives:

- Performance: local CLDR query cost, no IO, no avoidable per-call heavy work.
- Stability: explicit-region behavior, no likely-region inference, no multi-tender silent choice.
- Security: parse-only data path, no trust boundary.
- Operator/Ops: CLDR source/update docs, no runtime config.
- Developer/API: public API unchanged, sentinel errors preserved.
- User/Caller: README caveats and examples are clear.
- Main integration: normalize P0/P1/P2/P3.

Expected review verdict:

```text
P0=0 P1=0
```

- [x] **Step 2: Add lessons file**

Use this shape:

```markdown
# Issue #179 Locale Currency Mapping Lessons

## What changed

- `CurrencyByLocale` now uses explicit-region BCP47 parsing plus CLDR current
  legal tender data.

## What to repeat

- Reject likely-region inference for money helpers unless the public contract
  explicitly allows guesses.
- For locale-to-currency APIs, count current tender units and reject ambiguous
  multi-tender regions instead of picking the first value.

## Evidence

- `go test -race -count=1 ./money ./testing/concurrency`
- `make ci`
```

- [ ] **Step 3: Commit implementation**

Use a Lore commit:

```bash
git add money README.md README.ko.md CHANGELOG.md go.mod go.sum docs scripts
git commit -m "Resolve locale currencies from CLDR tender data" \
  -m "Constraint: Issue #179 requires source-backed locale mapping while preserving #35 sentinel errors and explicit-region behavior." \
  -m "Rejected: Likely-region currency inference | it guesses currencies for language-only tags." \
  -m "Rejected: Silent multi-tender selection | callers must choose when a region has multiple current tender units." \
  -m "Confidence: high" \
  -m "Scope-risk: moderate" \
  -m "Directive: Keep money locale helpers deterministic and reject ambiguous legal-tender cases." \
  -m "Tested: node scripts/generate-money-locale-currency-diagram.mjs; go test -count=1 ./money ./testing/concurrency; go test -race -count=1 ./money ./testing/concurrency; go test -count=1 ./...; make ci" \
  -m "Not-tested: live production locale data beyond the bundled golang.org/x/text CLDR snapshot"
```

- [ ] **Step 4: Push and create PR**

Use `--body-file` and make the final section `## DoD Status`.

PR title:

```text
Add CLDR-backed locale currency mapping
```

PR body must include:

- Closes #179.
- Summary of explicit-region CLDR lookup.
- Diagram evidence.
- Goroutine stress evidence.
- Step 6-R `P0=0 P1=0`.
- Validation command list.
- Final `## DoD Status` section.

- [ ] **Step 5: Run Step 7-R PR review**

After PR creation:

```bash
gh pr view <number> --json body,statusCheckRollup,mergeStateStatus
```

Verify:

- PR body is non-empty.
- Last `##` heading is `## DoD Status`.
- CI is pending or passing.

Record Step 7-R with `P0=0 P1=0`.

- [ ] **Step 6: Wait for CI within bounded SLA**

Use:

```bash
gh pr checks <number> --watch --interval 10
```

Stop on pass/fail. If long-running, report every 2-3 minutes and do not block beyond the agreed wait policy.

## Completion Criteria

- Issue #179 acceptance criteria are satisfied.
- `CurrencyByLocale` is CLDR-backed and deterministic.
- README locale set is in sync.
- Diagram assets exist, render, and are inspected.
- Goroutine stress and race tests pass.
- Step 6-R and Step 7-R record `P0=0 P1=0`.
- PR body ends with `## DoD Status`.
