package core

import (
	"fmt"
	"iter"
	"strconv"
	"time"
)

// Quarter represents one calendar quarter.
//
// The zero value is invalid. Use NewQuarter or QuarterOf when accepting
// caller-provided values.
type Quarter int

const (
	// Quarter1 is January through March.
	Quarter1 Quarter = 1
	// Quarter2 is April through June.
	Quarter2 Quarter = 2
	// Quarter3 is July through September.
	Quarter3 Quarter = 3
	// Quarter4 is October through December.
	Quarter4 Quarter = 4
)

// NewQuarter returns a validated quarter for number 1 through 4.
func NewQuarter(number int) (Quarter, error) {
	q := Quarter(number)
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, number)
	}
	return q, nil
}

// QuarterOf returns the quarter containing month.
func QuarterOf(month time.Month) (Quarter, error) {
	if month < time.January || month > time.December {
		return 0, fmt.Errorf("%w: month %d", ErrInvalidQuarter, month)
	}
	return Quarter((int(month)-1)/3 + 1), nil
}

// Valid reports whether q is Q1 through Q4.
func (q Quarter) Valid() bool {
	return q >= Quarter1 && q <= Quarter4
}

// Number returns the numeric quarter value.
func (q Quarter) Number() int {
	return int(q)
}

// StartMonth returns the first month in q.
func (q Quarter) StartMonth() (time.Month, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	return time.Month((int(q)-1)*3 + 1), nil
}

// EndMonth returns the last month in q.
func (q Quarter) EndMonth() (time.Month, error) {
	start, err := q.StartMonth()
	if err != nil {
		return 0, err
	}
	return start + 2, nil
}

// Add returns the quarter n quarters after q.
func (q Quarter) Add(n int) (Quarter, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	offset := floorMod(int(q)-1+n, 4)
	return Quarter(offset + 1), nil
}

// String returns Q1, Q2, Q3, Q4, or a diagnostic for invalid values.
func (q Quarter) String() string {
	if !q.Valid() {
		return fmt.Sprintf("Quarter(%d)", q)
	}
	return fmt.Sprintf("Q%d", q)
}

// YearQuarter represents a calendar quarter in a specific year.
//
// Year zero is invalid. Use NewYearQuarter or ParseYearQuarter when accepting
// caller-provided values.
type YearQuarter struct {
	Year    int
	Quarter Quarter
}

// NewYearQuarter returns a validated YearQuarter.
func NewYearQuarter(year int, quarter Quarter) (YearQuarter, error) {
	yq := YearQuarter{Year: year, Quarter: quarter}
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	return yq, nil
}

// YearQuarterOf returns the year and quarter containing t in t's location.
func YearQuarterOf(t time.Time) YearQuarter {
	quarter, _ := QuarterOf(t.Month())
	return YearQuarter{Year: t.Year(), Quarter: quarter}
}

// ParseYearQuarter parses a canonical value such as 2026-Q3.
func ParseYearQuarter(value string) (YearQuarter, error) {
	if len(value) != len("2006-Q1") || value[4] != '-' || value[5] != 'Q' {
		return YearQuarter{}, fmt.Errorf("%w: expected YYYY-QN", ErrInvalidTime)
	}
	for _, digit := range value[:4] {
		if digit < '0' || digit > '9' {
			return YearQuarter{}, fmt.Errorf("%w: expected four digit year", ErrInvalidTime)
		}
	}
	year, _ := strconv.Atoi(value[:4])
	quarter, err := NewQuarter(int(value[6] - '0'))
	if err != nil {
		return YearQuarter{}, err
	}
	return NewYearQuarter(year, quarter)
}

// Valid reports whether yq has a non-zero year and a valid quarter.
func (yq YearQuarter) Valid() bool {
	return yq.Year != 0 && yq.Quarter.Valid()
}

// Add returns the YearQuarter n quarters after yq.
func (yq YearQuarter) Add(n int) (YearQuarter, error) {
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	total := yq.Year*4 + int(yq.Quarter) - 1 + n
	year := floorDiv(total, 4)
	quarter := Quarter(total - year*4 + 1)
	return NewYearQuarter(year, quarter)
}

// Start returns local midnight on the first day of yq in loc.
func (yq YearQuarter) Start(loc *time.Location) (time.Time, error) {
	if loc == nil {
		return time.Time{}, fmt.Errorf("%w: nil location", ErrInvalidTime)
	}
	if err := yq.validate(); err != nil {
		return time.Time{}, err
	}
	month, err := yq.Quarter.StartMonth()
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(yq.Year, month, 1, 0, 0, 0, 0, loc), nil
}

// End returns local midnight at the exclusive start of the next quarter.
func (yq YearQuarter) End(loc *time.Location) (time.Time, error) {
	start, err := yq.Start(loc)
	if err != nil {
		return time.Time{}, err
	}
	return start.AddDate(0, 3, 0), nil
}

// Contains reports whether t is in yq in t's location.
func (yq YearQuarter) Contains(t time.Time) bool {
	if !yq.Valid() {
		return false
	}
	loc := t.Location()
	start, err := yq.Start(loc)
	if err != nil {
		return false
	}
	end, err := yq.End(loc)
	if err != nil {
		return false
	}
	return !t.Before(start) && t.Before(end)
}

// String returns canonical YYYY-QN text or a diagnostic for invalid values.
func (yq YearQuarter) String() string {
	if !yq.Valid() {
		return fmt.Sprintf("YearQuarter(%d,%s)", yq.Year, yq.Quarter)
	}
	return fmt.Sprintf("%04d-%s", yq.Year, yq.Quarter)
}

func (yq YearQuarter) validate() error {
	if !yq.Quarter.Valid() {
		return fmt.Errorf("%w: %d", ErrInvalidQuarter, yq.Quarter)
	}
	if yq.Year == 0 {
		return fmt.Errorf("%w: year 0", ErrInvalidTime)
	}
	return nil
}

// DatesUntil returns midnight dates from startInclusive through endExclusive.
//
// Boundaries are compared as calendar dates in startInclusive's location.
func DatesUntil(startInclusive, endExclusive time.Time) iter.Seq[time.Time] {
	loc := startInclusive.Location()
	start := dateOnly(startInclusive, loc)
	end := dateOnly(endExclusive, loc)
	return func(yield func(time.Time) bool) {
		for current := start; current.Before(end); current = current.AddDate(0, 0, 1) {
			if !yield(current) {
				return
			}
		}
	}
}

// DatesInclusive returns midnight dates from startInclusive through endInclusive.
//
// Boundaries are compared as calendar dates in startInclusive's location.
func DatesInclusive(startInclusive, endInclusive time.Time) iter.Seq[time.Time] {
	loc := startInclusive.Location()
	start := dateOnly(startInclusive, loc)
	end := dateOnly(endInclusive, loc)
	return func(yield func(time.Time) bool) {
		for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
			if !yield(current) {
				return
			}
		}
	}
}

func dateOnly(value time.Time, loc *time.Location) time.Time {
	local := value.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func floorMod(value, divisor int) int {
	mod := value % divisor
	if mod < 0 {
		mod += divisor
	}
	return mod
}

func floorDiv(value, divisor int) int {
	quotient := value / divisor
	remainder := value % divisor
	if remainder != 0 && ((remainder < 0) != (divisor < 0)) {
		quotient--
	}
	return quotient
}
