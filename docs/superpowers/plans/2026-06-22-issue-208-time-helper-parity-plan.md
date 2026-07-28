# Issue #208 Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

Issue: #208
Milestone: 0.6.3
Spec: `docs/superpowers/specs/2026-06-22-issue-208-time-helper-parity-design.md`
Spec review: `docs/superpowers/reviews/2026-06-22-issue-208-time-helper-parity-step-2r-spec-review.md`
Date: 2026-06-22

## 목표

Add a small Go-native time helper surface to `core` for quarter,
year-quarter, and calendar date iteration workflows. Preserve the issue's
constraint that this is not a Kotlin/JVM Java-time DSL clone.

## Current Fit

- `core` already hosts narrow shared helpers with exported Go doc comments,
  focused table tests, and README examples.
- Existing examples use external `core_test` packages and ordinary `Example`
  functions.
- Existing public behavior changes are documented in both `core/README.md` and
  `core/README.ko.md`.
- No current `core` helper imports `time` or `iter`; this change should keep
  both imports isolated to a new file.

## Implementation Tasks

### 1. Lock Tests First

Create failing tests in `core/time_test.go` before implementation.

- Quarter tests:
  - `NewQuarter(1..4)` succeeds.
  - `NewQuarter(0)` and `NewQuarter(5)` wrap `ErrInvalidQuarter`.
  - `QuarterOf(time.January) == Quarter1`, `QuarterOf(time.March) == Quarter1`,
    `QuarterOf(time.April) == Quarter2`, and
    `QuarterOf(time.December) == Quarter4`.
  - `QuarterOf(time.Month(0))` and `QuarterOf(time.Month(13))` wrap
    `ErrInvalidQuarter`.
  - `Quarter1.Add(4) == Quarter1`, `Quarter1.Add(-1) == Quarter4`, and a
    multi-year positive/negative offset stays in `1..4`.
  - `StartMonth`, `EndMonth`, `Number`, and `String` cover valid and invalid
    values.

- Year-quarter tests:
  - `NewYearQuarter(2026, Quarter2)` succeeds.
  - invalid quarter and year `0` fail with the expected sentinel.
  - `YearQuarterOf(time.Date(2026, time.October, 1, ...)) == 2026-Q4`.
  - `ParseYearQuarter("2026-Q3")` succeeds.
  - `ParseYearQuarter` rejects `2026Q3`, `2026-Q0`, `2026-Q5`, `abcd-Q1`,
    `26-Q1`, and `2026-q1`.
  - `Add` crosses boundaries: `2026-Q4 + 1 == 2027-Q1`,
    `2026-Q1 - 1 == 2025-Q4`, and result year `0` is rejected.
  - `Start` and `End` use the supplied location and reject `nil` locations.
  - `Contains` includes the first instant, excludes the end instant, and
    returns false for invalid receivers.
  - `String` returns canonical `YYYY-QN` for valid values and a diagnostic for
    invalid values.

- Date iteration tests:
  - `DatesUntil` returns an empty sequence when the end date is before the
    start date.
  - `DatesUntil` excludes the end date.
  - `DatesInclusive` includes the end date.
  - Non-midnight input times yield midnight dates in the start location.
  - End values in a different location are converted into the start location
    before date comparison.
  - `America/New_York` spring-forward and fall-back ranges produce the expected
    calendar dates without asserting fixed 24-hour elapsed durations.

### 2. Implement Time Helpers

Create `core/time.go` with standard-library-only code.

- Add exported sentinels to `core/errors.go`, matching the existing error
  location:

```go
var (
    ErrInvalidQuarter = errors.New("invalid quarter")
    ErrInvalidTime    = errors.New("invalid time")
)
```

- Implement `Quarter` as an `int` enum with Go doc comments on the type,
  constants, constructors, and exported methods.
- Use wrapped sentinel errors through `fmt.Errorf("%w: ...", sentinel)`.
- Use modulo arithmetic for `Quarter.Add`, handling negative offsets.
- Implement `YearQuarter` with:
  - constructor validation,
  - `YearQuarterOf`,
  - strict `YYYY-QN` parsing,
  - validated quarter arithmetic,
  - `Start`, `End`, `Contains`, `Valid`, and `String`.
- Add a small integer floor-division helper only if needed for negative
  quarter offsets. Keep it unexported and covered through public tests.
- Implement `DatesUntil` and `DatesInclusive` with `iter.Seq[time.Time]`,
  normalizing both boundaries to dates in the start location and advancing via
  `AddDate(0, 0, 1)`.

### 3. Add Example Tests

Create `core/time_example_test.go` or extend existing example tests.

- Quarter/year-quarter example:
  - parse or derive a quarter,
  - print `YYYY-QN`,
  - print start/end dates in UTC or a fixed location.
- Date iteration example:
  - iterate a small reporting window,
  - print calendar dates only,
  - avoid non-deterministic local timezone output.

### 4. Update Documentation

Update both README files.

- `core/README.md`:
  - mention quarter/year-quarter/date iteration helpers in the intro.
  - add a concise usage example.
  - document DST/timezone behavior and exclusions in `Behavior`.
- `core/README.ko.md`:
  - mirror the English behavior and example changes.
  - keep terminology consistent with existing mixed Korean/English style.

### 5. Format and Local Validation

Run validation after implementation:

1. `go test -count=1 ./core`
2. `go test -race -count=1 ./core`
3. `go test ./...`
4. `make fmt-check`
5. `make tidy-check`
6. `make vet`
7. `make lint`
8. `make ci`
9. `git diff --check`

If a later broad command repeats an earlier narrow command exactly, record that
as cumulative evidence rather than rerunning unnecessary duplicates.

### 6. Review, Commit, and PR

- Run Step 6-R 7-tier review:
  - performance,
  - stability,
  - security,
  - operator/Ops,
  - developer/API,
  - user/caller,
  - main-session integration.
- Record any native subagent timeout or fallback explicitly.
- Fix all P0/P1 findings before PR creation.
- Commit with Lore trailers.
- Open PR linked to #208.
- Set PR assignee to `debop`.
- Copy issue milestone `0.6.3` and labels from #208.
- Ensure PR body ends with `## DoD Status`.

## Requirement Traceability

| Spec requirement | Plan coverage |
|---|---|
| Reject Kotlin-style DSL cloning | Tasks 2 and 4 keep only enum/value/iterator helpers and document exclusions. |
| Time zone behavior | Tasks 1, 2, and 4 cover supplied locations, candidate locations, and date boundary conversion. |
| DST behavior | Tasks 1, 2, and 4 require `AddDate` behavior and New York DST tests/docs. |
| Zero-value behavior | Tasks 1 and 2 cover invalid `Quarter(0)`, invalid `YearQuarter`, and year `0`. |
| Boundary behavior | Tasks 1 and 2 cover quarter/month ranges, inclusive/exclusive dates, and quarter year crossings. |
| README examples | Tasks 3 and 4 add compiling examples plus bilingual README updates. |
| No panics for invalid caller values | Tasks 1 and 2 require sentinel errors or false return values for invalid inputs. |
| No new dependency | Task 2 restricts implementation to the standard library. |

## 위험 and Mitigations

| Risk | Mitigation |
|---|---|
| Date iteration accidentally becomes fixed-duration day math. | Use `AddDate(0, 0, 1)` and DST tests that assert dates, not elapsed hours. |
| Strict parser is too narrow. | Keep first slice strict by spec; future relaxed parser can be additive. |
| `YearQuarter.Add` crosses to year `0`. | Reject result year `0` with `ErrInvalidTime` and cover in tests. |
| README diverges across languages. | Update English and Korean files in the same patch and verify with diff review. |

## Stop Condition

Stop after PR creation only when:

- implementation and docs match the spec,
- validation commands have fresh passing evidence or an explicit recorded gap,
- Step 6-R P0/P1 findings are zero,
- PR is assigned to `debop`,
- PR milestone and labels mirror #208,
- PR body ends with `## DoD Status`.
