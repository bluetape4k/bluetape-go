package core

import (
	"fmt"
	"iter"
	"strconv"
	"time"
)

// Quarter int 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// NewQuarter NewQuarter 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - number: NewQuarter 동작에 필요한 number 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewQuarter(number int) (Quarter, error) {
	q := Quarter(number)
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, number)
	}
	return q, nil
}

// QuarterOf QuarterOf 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - month: QuarterOf 동작에 필요한 month 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func QuarterOf(month time.Month) (Quarter, error) {
	if month < time.January || month > time.December {
		return 0, fmt.Errorf("%w: month %d", ErrInvalidQuarter, month)
	}
	return Quarter((int(month)-1)/3 + 1), nil
}

// Valid Valid 공개 API의 동작을 수행한다.
func (q Quarter) Valid() bool {
	return q >= Quarter1 && q <= Quarter4
}

// Number Number 공개 API의 동작을 수행한다.
func (q Quarter) Number() int {
	return int(q)
}

// StartMonth StartMonth 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (q Quarter) StartMonth() (time.Month, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	return time.Month((int(q)-1)*3 + 1), nil
}

// EndMonth EndMonth 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (q Quarter) EndMonth() (time.Month, error) {
	start, err := q.StartMonth()
	if err != nil {
		return 0, err
	}
	return start + 2, nil
}

// Add Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - n: Add 동작에 필요한 n 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (q Quarter) Add(n int) (Quarter, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	offset := floorMod(int(q)-1+n, 4)
	return Quarter(offset + 1), nil
}

// String String 공개 API의 동작을 수행한다.
func (q Quarter) String() string {
	if !q.Valid() {
		return fmt.Sprintf("Quarter(%d)", q)
	}
	return fmt.Sprintf("Q%d", q)
}

// YearQuarter struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type YearQuarter struct {
	Year    int
	Quarter Quarter
}

// NewYearQuarter NewYearQuarter 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - year: NewYearQuarter 동작에 필요한 year 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - quarter: NewYearQuarter 동작에 필요한 quarter 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewYearQuarter(year int, quarter Quarter) (YearQuarter, error) {
	yq := YearQuarter{Year: year, Quarter: quarter}
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	return yq, nil
}

// YearQuarterOf YearQuarterOf 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - t: YearQuarterOf 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func YearQuarterOf(t time.Time) YearQuarter {
	quarter, _ := QuarterOf(t.Month())
	return YearQuarter{Year: t.Year(), Quarter: quarter}
}

// ParseYearQuarter ParseYearQuarter 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ParseYearQuarter가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Valid Valid 공개 API의 동작을 수행한다.
func (yq YearQuarter) Valid() bool {
	return yq.Year != 0 && yq.Quarter.Valid()
}

// Add Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - n: Add 동작에 필요한 n 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (yq YearQuarter) Add(n int) (YearQuarter, error) {
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	total := yq.Year*4 + int(yq.Quarter) - 1 + n
	year := floorDiv(total, 4)
	quarter := Quarter(total - year*4 + 1)
	return NewYearQuarter(year, quarter)
}

// Start Start 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - loc: Start 동작에 필요한 loc 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// End End 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - loc: End 동작에 필요한 loc 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (yq YearQuarter) End(loc *time.Location) (time.Time, error) {
	start, err := yq.Start(loc)
	if err != nil {
		return time.Time{}, err
	}
	return start.AddDate(0, 3, 0), nil
}

// Contains Contains 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - t: Contains 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// String String 공개 API의 동작을 수행한다.
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

// DatesUntil DatesUntil 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - startInclusive: DatesUntil 동작에 필요한 startInclusive 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - endExclusive: DatesUntil 동작에 필요한 endExclusive 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// DatesInclusive DatesInclusive 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - startInclusive: DatesInclusive 동작에 필요한 startInclusive 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - endInclusive: DatesInclusive 동작에 필요한 endInclusive 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
