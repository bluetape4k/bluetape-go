# Issue #208 Go Time Helper Parity Design

Issue: #208
Milestone: 0.6.3
Worktree: `issue-208-time-helper-parity`
Date: 2026-06-22

## Objective

Add a narrow Go-native time helper slice to `core` that covers repeated backend
date/quarter workflows without cloning the Kotlin/JVM Java-time DSL.

## Current Evidence

- Baseline worktree is based on `origin/develop` at `2c6de4a`.
- Baseline `go test ./...` passed in the #208 worktree.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` marks time
  helpers as missing but says only repeated Go `time` helpers should be added;
  Java Time DSL, `Period`, and JVM temporal type mirroring are explicit
  non-goals.
- `docs/research/2026-06-01-issue-8-core-support-inventory.md` deferred
  time/date helpers until repeated workflows emerged.
- Kotlin source candidates:
  - `Quarter.kt` provides quarter validation, month mapping, and modulo
    quarter movement.
  - `YearQuarter.kt` stores `(year, quarter)` and derives it from temporal
    values.
  - `DateIterator.kt` and `TemporalIterator.kt` are iterator base classes, not
    a direct Go API shape.
  - `DurationSupport.kt` mostly wraps Java/Kotlin `Duration` DSL operations.
  - `utils/javatimes` contains a much broader interval/calendar framework and
    is outside this issue's first slice.
- Existing Go usage:
  - `money.parseIMFPeriod` has package-local monthly, quarterly, and annual
    period-end logic.
  - `money`, `jwt`, `id`, `ratelimit`, and workflow code already use
    `time.Time`, `time.Duration`, and package-local injectable clocks directly.
- Go standard-library behavior:
  - `time.Date` normalizes out-of-range month/day values and has undefined
    zone choice for skipped/repeated DST wall-clock times.
  - `time.Time.AddDate` uses the time's location and can shift by 23 or 25
    hours across DST.
  - `time.ParseDuration` already covers signed `ns`, `us`/`µs`, `ms`, `s`,
    `m`, and `h`.
  - `time.Duration.String` already formats canonical duration strings.

## Selected First Slice

Implement only helpers that are small, repeated, and easy to read in Go:

1. Quarter values and month mapping.
2. Year-quarter values and quarter arithmetic.
3. Calendar date iteration using `iter.Seq[time.Time]` and `time.AddDate`.
4. README examples that show scheduler/reporting/audit style usage.

Do not add a separate package. Keep the helpers in `core` because #204 groups
core foundation parity and existing quarter-like code is already in utility
call sites.

## Public API Contract

### Quarter

```go
type Quarter int

const (
    Quarter1 Quarter = 1
    Quarter2 Quarter = 2
    Quarter3 Quarter = 3
    Quarter4 Quarter = 4
)

var ErrInvalidQuarter = errors.New("invalid quarter")

func NewQuarter(number int) (Quarter, error)
func QuarterOf(month time.Month) (Quarter, error)
func (q Quarter) Valid() bool
func (q Quarter) Number() int
func (q Quarter) StartMonth() (time.Month, error)
func (q Quarter) EndMonth() (time.Month, error)
func (q Quarter) Add(n int) (Quarter, error)
func (q Quarter) String() string
```

Contract:

- Valid quarter numbers are `1..4`.
- Zero-value `Quarter(0)` is invalid. Methods that need a valid value return
  `ErrInvalidQuarter`.
- `QuarterOf` accepts only `time.January..time.December`.
- `Add` uses modulo quarter arithmetic for positive and negative offsets.
- `String` returns `Q1`, `Q2`, `Q3`, `Q4`, or `Quarter(<n>)` for invalid
  values so logs do not hide invalid state.

### YearQuarter

```go
type YearQuarter struct {
    Year    int
    Quarter Quarter
}

func NewYearQuarter(year int, quarter Quarter) (YearQuarter, error)
func YearQuarterOf(t time.Time) YearQuarter
func ParseYearQuarter(value string) (YearQuarter, error)
func (yq YearQuarter) Valid() bool
func (yq YearQuarter) Add(n int) (YearQuarter, error)
func (yq YearQuarter) Start(loc *time.Location) (time.Time, error)
func (yq YearQuarter) End(loc *time.Location) (time.Time, error)
func (yq YearQuarter) Contains(t time.Time) bool
func (yq YearQuarter) String() string
```

Contract:

- `NewYearQuarter` rejects invalid quarters and year `0`.
- `YearQuarterOf` derives from `t.Year()` and `t.Month()` in the time's own
  location.
- `ParseYearQuarter` accepts only canonical `YYYY-QN`, for example `2026-Q3`.
- `Start` returns local midnight at the first day of the quarter.
- `End` returns the exclusive start of the next quarter.
- `Start` and `End` reject `nil` locations instead of letting `time.Date`
  panic.
- `Contains` compares against `[Start(t.Location()), End(t.Location()))` in
  the candidate time's location.
- `Contains` returns `false` for invalid `YearQuarter` values because it cannot
  report validation errors.
- `String` returns canonical `YYYY-QN` for valid values and an explicit
  diagnostic string for invalid values.

### Date iteration

```go
func DatesUntil(startInclusive, endExclusive time.Time) iter.Seq[time.Time]
func DatesInclusive(startInclusive, endInclusive time.Time) iter.Seq[time.Time]
```

Contract:

- Iteration is date-only and lexical in the `startInclusive.Location()`.
- Each yielded value is midnight for that date in the start location.
- End values are converted into the start location before comparing date
  boundaries.
- `DatesUntil` yields `[start date, end date)`.
- `DatesInclusive` yields `[start date, end date]`.
- If the end date is before the start date, the sequence is empty.
- Iteration advances with `AddDate(0, 0, 1)`, preserving wall-clock date
  behavior across DST instead of assuming every day is 24 hours.

## Explicit Exclusions

- No Kotlin-style numeric DSL such as `3.days`, `2.hours`, or unary duration
  operators.
- No broad duration parser/formatter wrapper in this PR because
  `time.ParseDuration` and `time.Duration.String` already cover the practical
  Go contract.
- No `Period`, interval tree, calendar visitor, business calendar, week-year,
  holiday, or range collection framework.
- No JVM `Date`, `Timestamp`, `Temporal`, `LocalDate`, `LocalDateTime`,
  `OffsetDateTime`, or `ZonedDateTime` mirroring.
- No injectable clock abstraction in `core` yet. Existing packages already use
  `func() time.Time`; future packages should continue that pattern unless
  repeated public API pressure proves a shared type is needed.
- No timezone database dependency or `time/tzdata` import in production code.

## Error Contract

- `ErrInvalidQuarter` wraps invalid quarter and invalid month cases through
  `fmt.Errorf("%w: ...", ErrInvalidQuarter)`.
- Add `ErrInvalidTime = errors.New("invalid time")` as the shared sentinel for
  time helper validation.
- `ErrInvalidTime` wraps invalid year-quarter parse, year `0`, and nil
  location cases through `fmt.Errorf("%w: ...", ErrInvalidTime)`.
- `YearQuarter` methods return `ErrInvalidQuarter` for invalid quarter values
  and `ErrInvalidTime` for invalid year/location/parse values.
- No method panics for caller-supplied invalid values.

## Tests

Add table-driven tests under `core`:

- Quarter:
  - `NewQuarter(1..4)` succeeds.
  - `NewQuarter(0)` and `NewQuarter(5)` wrap `ErrInvalidQuarter`.
  - `QuarterOf(time.January) == Quarter1`, `QuarterOf(time.December) == Quarter4`.
  - `QuarterOf(time.Month(0))` and `QuarterOf(time.Month(13))` wrap
    `ErrInvalidQuarter`.
  - `Quarter1.Add(4) == Quarter1`, `Quarter1.Add(-1) == Quarter4`.
  - invalid quarter method calls return errors.
- YearQuarter:
  - `NewYearQuarter(2026, Quarter2)` succeeds.
  - invalid quarter and year `0` fail.
  - `YearQuarterOf(time.Date(2026, time.October, 1, ...)) == 2026-Q4`.
  - `ParseYearQuarter("2026-Q3")` succeeds.
  - malformed values such as `2026Q3`, `2026-Q0`, `2026-Q5`, and `abcd-Q1`
    fail.
  - `Add` crosses year boundaries: `2026-Q4 + 1 == 2027-Q1` and
    `2026-Q1 - 1 == 2025-Q4`.
  - `Start` and `End` use the supplied location and reject nil locations.
  - `Contains` includes the first instant and excludes the end instant.
  - `Contains` returns false for invalid `YearQuarter` values.
- Date iteration:
  - Empty range when end is before start.
  - Until range excludes the end date.
  - Inclusive range includes the end date.
  - Start/end times with non-midnight clocks still yield date midnights.
  - Different end location is converted into the start location before
    comparing.
  - DST spring-forward and fall-back ranges in `America/New_York` produce the
    expected calendar dates without asserting fixed 24-hour durations.
- README examples compile through ordinary tests or example tests.

## Documentation

Update `core/README.md` and `core/README.ko.md`:

- Add a concise quarter/year-quarter example.
- Add a scheduler/reporting/audit-style date iteration example.
- State that helpers are calendar-date helpers, not fixed-duration day math.
- State DST and timezone behavior explicitly.
- State excluded DSL/framework features.

## Validation Plan

- `go test -count=1 ./core`
- `go test -race -count=1 ./core`
- `go test ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `git diff --check`

## Step DoD

| Requirement | Status |
|---|---|
| API rejects Kotlin-style DSL cloning | Specified in exclusions. |
| Time zone behavior | Specified for `Start`, `End`, `Contains`, and date iteration. |
| DST behavior | Specified via `AddDate` and `America/New_York` tests. |
| Zero-value behavior | Invalid `Quarter(0)` and year `0` specified. |
| Boundary behavior | Quarter/month ranges, `[start, end)`, inclusive/until iteration, and year crossings specified. |
| README examples | Required for English and Korean README files. |
