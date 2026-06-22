package core_test

import (
	"errors"
	"iter"
	"reflect"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestNewQuarter(t *testing.T) {
	for i := 1; i <= 4; i++ {
		quarter, err := core.NewQuarter(i)
		if err != nil {
			t.Fatalf("NewQuarter(%d) error = %v", i, err)
		}
		if quarter.Number() != i {
			t.Fatalf("NewQuarter(%d).Number() = %d", i, quarter.Number())
		}
	}

	for _, value := range []int{0, 5, -1} {
		if _, err := core.NewQuarter(value); !errors.Is(err, core.ErrInvalidQuarter) {
			t.Fatalf("NewQuarter(%d) error = %v, want ErrInvalidQuarter", value, err)
		}
	}
}

func TestQuarterOf(t *testing.T) {
	tests := []struct {
		name  string
		month time.Month
		want  core.Quarter
	}{
		{name: "january", month: time.January, want: core.Quarter1},
		{name: "march", month: time.March, want: core.Quarter1},
		{name: "april", month: time.April, want: core.Quarter2},
		{name: "december", month: time.December, want: core.Quarter4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := core.QuarterOf(tt.month)
			if err != nil {
				t.Fatalf("QuarterOf(%v) error = %v", tt.month, err)
			}
			if got != tt.want {
				t.Fatalf("QuarterOf(%v) = %v, want %v", tt.month, got, tt.want)
			}
		})
	}

	for _, month := range []time.Month{0, 13} {
		if _, err := core.QuarterOf(month); !errors.Is(err, core.ErrInvalidQuarter) {
			t.Fatalf("QuarterOf(%v) error = %v, want ErrInvalidQuarter", month, err)
		}
	}
}

func TestQuarterMethods(t *testing.T) {
	tests := []struct {
		quarter core.Quarter
		start   time.Month
		end     time.Month
		text    string
	}{
		{quarter: core.Quarter1, start: time.January, end: time.March, text: "Q1"},
		{quarter: core.Quarter2, start: time.April, end: time.June, text: "Q2"},
		{quarter: core.Quarter3, start: time.July, end: time.September, text: "Q3"},
		{quarter: core.Quarter4, start: time.October, end: time.December, text: "Q4"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if !tt.quarter.Valid() {
				t.Fatalf("%v.Valid() = false", tt.quarter)
			}
			if got, err := tt.quarter.StartMonth(); err != nil || got != tt.start {
				t.Fatalf("%v.StartMonth() = %v, %v; want %v, nil", tt.quarter, got, err, tt.start)
			}
			if got, err := tt.quarter.EndMonth(); err != nil || got != tt.end {
				t.Fatalf("%v.EndMonth() = %v, %v; want %v, nil", tt.quarter, got, err, tt.end)
			}
			if got := tt.quarter.String(); got != tt.text {
				t.Fatalf("%v.String() = %q, want %q", tt.quarter, got, tt.text)
			}
		})
	}

	invalid := core.Quarter(0)
	if invalid.Valid() {
		t.Fatalf("Quarter(0).Valid() = true")
	}
	if got := invalid.Number(); got != 0 {
		t.Fatalf("Quarter(0).Number() = %d, want 0", got)
	}
	if _, err := invalid.StartMonth(); !errors.Is(err, core.ErrInvalidQuarter) {
		t.Fatalf("Quarter(0).StartMonth() error = %v, want ErrInvalidQuarter", err)
	}
	if _, err := invalid.EndMonth(); !errors.Is(err, core.ErrInvalidQuarter) {
		t.Fatalf("Quarter(0).EndMonth() error = %v, want ErrInvalidQuarter", err)
	}
	if _, err := invalid.Add(1); !errors.Is(err, core.ErrInvalidQuarter) {
		t.Fatalf("Quarter(0).Add(1) error = %v, want ErrInvalidQuarter", err)
	}
	if got := invalid.String(); got != "Quarter(0)" {
		t.Fatalf("Quarter(0).String() = %q, want Quarter(0)", got)
	}
}

func TestQuarterAdd(t *testing.T) {
	tests := []struct {
		name    string
		quarter core.Quarter
		offset  int
		want    core.Quarter
	}{
		{name: "full cycle", quarter: core.Quarter1, offset: 4, want: core.Quarter1},
		{name: "previous", quarter: core.Quarter1, offset: -1, want: core.Quarter4},
		{name: "large positive", quarter: core.Quarter2, offset: 11, want: core.Quarter1},
		{name: "large negative", quarter: core.Quarter3, offset: -10, want: core.Quarter1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.quarter.Add(tt.offset)
			if err != nil {
				t.Fatalf("%v.Add(%d) error = %v", tt.quarter, tt.offset, err)
			}
			if got != tt.want {
				t.Fatalf("%v.Add(%d) = %v, want %v", tt.quarter, tt.offset, got, tt.want)
			}
		})
	}
}

func TestNewYearQuarter(t *testing.T) {
	got, err := core.NewYearQuarter(2026, core.Quarter2)
	if err != nil {
		t.Fatalf("NewYearQuarter error = %v", err)
	}
	if got.Year != 2026 || got.Quarter != core.Quarter2 {
		t.Fatalf("NewYearQuarter = %+v", got)
	}

	if _, err := core.NewYearQuarter(2026, core.Quarter(5)); !errors.Is(err, core.ErrInvalidQuarter) {
		t.Fatalf("NewYearQuarter invalid quarter error = %v, want ErrInvalidQuarter", err)
	}
	if _, err := core.NewYearQuarter(0, core.Quarter1); !errors.Is(err, core.ErrInvalidTime) {
		t.Fatalf("NewYearQuarter year zero error = %v, want ErrInvalidTime", err)
	}
}

func TestYearQuarterOf(t *testing.T) {
	got := core.YearQuarterOf(time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC))
	want := core.YearQuarter{Year: 2026, Quarter: core.Quarter4}
	if got != want {
		t.Fatalf("YearQuarterOf = %+v, want %+v", got, want)
	}
}

func TestParseYearQuarter(t *testing.T) {
	got, err := core.ParseYearQuarter("2026-Q3")
	if err != nil {
		t.Fatalf("ParseYearQuarter error = %v", err)
	}
	want := core.YearQuarter{Year: 2026, Quarter: core.Quarter3}
	if got != want {
		t.Fatalf("ParseYearQuarter = %+v, want %+v", got, want)
	}

	for _, value := range []string{"2026Q3", "2026-Q0", "2026-Q5", "abcd-Q1", "26-Q1", "2026-q1"} {
		if _, err := core.ParseYearQuarter(value); !errors.Is(err, core.ErrInvalidTime) && !errors.Is(err, core.ErrInvalidQuarter) {
			t.Fatalf("ParseYearQuarter(%q) error = %v, want ErrInvalidTime or ErrInvalidQuarter", value, err)
		}
	}
}

func TestYearQuarterAdd(t *testing.T) {
	tests := []struct {
		name   string
		value  core.YearQuarter
		offset int
		want   core.YearQuarter
	}{
		{
			name:   "next year",
			value:  core.YearQuarter{Year: 2026, Quarter: core.Quarter4},
			offset: 1,
			want:   core.YearQuarter{Year: 2027, Quarter: core.Quarter1},
		},
		{
			name:   "previous year",
			value:  core.YearQuarter{Year: 2026, Quarter: core.Quarter1},
			offset: -1,
			want:   core.YearQuarter{Year: 2025, Quarter: core.Quarter4},
		},
		{
			name:   "multi year",
			value:  core.YearQuarter{Year: 2026, Quarter: core.Quarter2},
			offset: 11,
			want:   core.YearQuarter{Year: 2029, Quarter: core.Quarter1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.Add(tt.offset)
			if err != nil {
				t.Fatalf("%v.Add(%d) error = %v", tt.value, tt.offset, err)
			}
			if got != tt.want {
				t.Fatalf("%v.Add(%d) = %+v, want %+v", tt.value, tt.offset, got, tt.want)
			}
		})
	}

	if _, err := (core.YearQuarter{Year: 1, Quarter: core.Quarter1}).Add(-4); !errors.Is(err, core.ErrInvalidTime) {
		t.Fatalf("YearQuarter crossing year zero error = %v, want ErrInvalidTime", err)
	}
}

func TestYearQuarterStartEndContains(t *testing.T) {
	loc := time.FixedZone("KST", 9*60*60)
	yq := core.YearQuarter{Year: 2026, Quarter: core.Quarter2}

	start, err := yq.Start(loc)
	if err != nil {
		t.Fatalf("Start error = %v", err)
	}
	wantStart := time.Date(2026, time.April, 1, 0, 0, 0, 0, loc)
	if !start.Equal(wantStart) || start.Location() != loc {
		t.Fatalf("Start = %v (%v), want %v (%v)", start, start.Location(), wantStart, loc)
	}

	end, err := yq.End(loc)
	if err != nil {
		t.Fatalf("End error = %v", err)
	}
	wantEnd := time.Date(2026, time.July, 1, 0, 0, 0, 0, loc)
	if !end.Equal(wantEnd) || end.Location() != loc {
		t.Fatalf("End = %v (%v), want %v (%v)", end, end.Location(), wantEnd, loc)
	}

	if _, err := yq.Start(nil); !errors.Is(err, core.ErrInvalidTime) {
		t.Fatalf("Start(nil) error = %v, want ErrInvalidTime", err)
	}
	if _, err := yq.End(nil); !errors.Is(err, core.ErrInvalidTime) {
		t.Fatalf("End(nil) error = %v, want ErrInvalidTime", err)
	}

	if !yq.Contains(start) {
		t.Fatalf("Contains(start) = false")
	}
	if yq.Contains(end) {
		t.Fatalf("Contains(end) = true")
	}
	if (core.YearQuarter{Year: 0, Quarter: core.Quarter2}).Contains(start) {
		t.Fatalf("invalid YearQuarter.Contains(start) = true")
	}
}

func TestYearQuarterString(t *testing.T) {
	if got := (core.YearQuarter{Year: 2026, Quarter: core.Quarter3}).String(); got != "2026-Q3" {
		t.Fatalf("String() = %q, want 2026-Q3", got)
	}
	if got := (core.YearQuarter{Year: 0, Quarter: core.Quarter3}).String(); got != "YearQuarter(0,Q3)" {
		t.Fatalf("invalid year String() = %q, want YearQuarter(0,Q3)", got)
	}
	if got := (core.YearQuarter{Year: 2026, Quarter: core.Quarter(5)}).String(); got != "YearQuarter(2026,Quarter(5))" {
		t.Fatalf("invalid quarter String() = %q, want YearQuarter(2026,Quarter(5))", got)
	}
}

func TestDatesUntil(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	start := time.Date(2026, time.January, 1, 15, 30, 0, 0, loc)
	end := time.Date(2026, time.January, 4, 8, 0, 0, 0, loc)

	got := collectDateKeys(core.DatesUntil(start, end))
	want := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DatesUntil keys = %v, want %v", got, want)
	}

	for _, date := range collectDates(core.DatesUntil(start, end)) {
		if date.Location() != loc {
			t.Fatalf("date location = %v, want %v", date.Location(), loc)
		}
		if date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
			t.Fatalf("date = %v, want midnight", date)
		}
	}
}

func TestDatesUntilEmptyWhenEndBeforeStart(t *testing.T) {
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 4, 0, 0, 0, 0, time.UTC)

	got := collectDateKeys(core.DatesUntil(start, end))
	if len(got) != 0 {
		t.Fatalf("DatesUntil reversed = %v, want empty", got)
	}
}

func TestDatesInclusive(t *testing.T) {
	start := time.Date(2026, time.January, 1, 23, 59, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 3, 1, 0, 0, 0, time.UTC)

	got := collectDateKeys(core.DatesInclusive(start, end))
	want := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DatesInclusive keys = %v, want %v", got, want)
	}
}

func TestDatesUntilConvertsEndIntoStartLocation(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)
	start := time.Date(2026, time.January, 1, 23, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 3, 8, 0, 0, 0, tokyo)

	got := collectDateKeys(core.DatesUntil(start, end))
	want := []string{"2026-01-01"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DatesUntil mixed location keys = %v, want %v", got, want)
	}
}

func TestDateIterationDSTCalendarDates(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York unavailable: %v", err)
	}

	springStart := time.Date(2026, time.March, 7, 12, 0, 0, 0, loc)
	springEnd := time.Date(2026, time.March, 10, 12, 0, 0, 0, loc)
	gotSpring := collectDateKeys(core.DatesInclusive(springStart, springEnd))
	wantSpring := []string{"2026-03-07", "2026-03-08", "2026-03-09", "2026-03-10"}
	if !reflect.DeepEqual(gotSpring, wantSpring) {
		t.Fatalf("spring DST keys = %v, want %v", gotSpring, wantSpring)
	}

	fallStart := time.Date(2026, time.October, 31, 12, 0, 0, 0, loc)
	fallEnd := time.Date(2026, time.November, 2, 12, 0, 0, 0, loc)
	gotFall := collectDateKeys(core.DatesInclusive(fallStart, fallEnd))
	wantFall := []string{"2026-10-31", "2026-11-01", "2026-11-02"}
	if !reflect.DeepEqual(gotFall, wantFall) {
		t.Fatalf("fall DST keys = %v, want %v", gotFall, wantFall)
	}
}

func collectDates(seq iter.Seq[time.Time]) []time.Time {
	var dates []time.Time
	for date := range seq {
		dates = append(dates, date)
	}
	return dates
}

func collectDateKeys(seq iter.Seq[time.Time]) []string {
	dates := collectDates(seq)
	keys := make([]string, 0, len(dates))
	for _, date := range dates {
		keys = append(keys, date.Format(time.DateOnly))
	}
	return keys
}
