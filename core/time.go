package core

import (
	"fmt"
	"iter"
	"strconv"
	"time"
)

// Quarter int 공개 타입이다.
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

// NewQuarter Quarter 인스턴스를 생성한다.
//
// 매개변수:
//   - number: NewQuarter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewQuarter(number int) (Quarter, error) {
	q := Quarter(number)
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, number)
	}
	return q, nil
}

// QuarterOf 입력 값에서 도메인 값을 계산한다.
//
// 매개변수:
//   - month: QuarterOf에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func QuarterOf(month time.Month) (Quarter, error) {
	if month < time.January || month > time.December {
		return 0, fmt.Errorf("%w: month %d", ErrInvalidQuarter, month)
	}
	return Quarter((int(month)-1)/3 + 1), nil
}

// Valid 값이 유효한지 반환한다.
func (q Quarter) Valid() bool {
	return q >= Quarter1 && q <= Quarter4
}

// Number numeric constraint에 포함되는 타입 집합이다.
func (q Quarter) Number() int {
	return int(q)
}

// StartMonth 분기 시작 월을 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (q Quarter) StartMonth() (time.Month, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	return time.Month((int(q)-1)*3 + 1), nil
}

// EndMonth 분기 종료 월을 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (q Quarter) EndMonth() (time.Month, error) {
	start, err := q.StartMonth()
	if err != nil {
		return 0, err
	}
	return start + 2, nil
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - n: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (q Quarter) Add(n int) (Quarter, error) {
	if !q.Valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidQuarter, q)
	}
	offset := floorMod(int(q)-1+n, 4)
	return Quarter(offset + 1), nil
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (q Quarter) String() string {
	if !q.Valid() {
		return fmt.Sprintf("Quarter(%d)", q)
	}
	return fmt.Sprintf("Q%d", q)
}

// YearQuarter 패키지에서 공개하는 구조체다.
type YearQuarter struct {
	Year    int
	Quarter Quarter
}

// NewYearQuarter YearQuarter 인스턴스를 생성한다.
//
// 매개변수:
//   - year: NewYearQuarter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - quarter: NewYearQuarter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewYearQuarter(year int, quarter Quarter) (YearQuarter, error) {
	yq := YearQuarter{Year: year, Quarter: quarter}
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	return yq, nil
}

// YearQuarterOf 입력 값에서 도메인 값을 계산한다.
//
// 매개변수:
//   - t: YearQuarterOf에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func YearQuarterOf(t time.Time) YearQuarter {
	quarter, _ := QuarterOf(t.Month())
	return YearQuarter{Year: t.Year(), Quarter: quarter}
}

// ParseYearQuarter 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - value: ParseYearQuarter가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Valid 값이 유효한지 반환한다.
func (yq YearQuarter) Valid() bool {
	return yq.Year != 0 && yq.Quarter.Valid()
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - n: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (yq YearQuarter) Add(n int) (YearQuarter, error) {
	if err := yq.validate(); err != nil {
		return YearQuarter{}, err
	}
	total := yq.Year*4 + int(yq.Quarter) - 1 + n
	year := floorDiv(total, 4)
	quarter := Quarter(total - year*4 + 1)
	return NewYearQuarter(year, quarter)
}

// Start 병렬 작업 실행 흐름을 제어한다.
//
// 매개변수:
//   - loc: Start에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// End 기간의 종료 시각을 반환한다.
//
// 매개변수:
//   - loc: End에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (yq YearQuarter) End(loc *time.Location) (time.Time, error) {
	start, err := yq.Start(loc)
	if err != nil {
		return time.Time{}, err
	}
	return start.AddDate(0, 3, 0), nil
}

// Contains 값이 포함되어 있는지 반환한다.
//
// 매개변수:
//   - t: Contains에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
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

// DatesUntil 종료일 전까지의 날짜 목록을 반환한다.
//
// 매개변수:
//   - startInclusive: DatesUntil에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - endExclusive: DatesUntil에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// DatesInclusive 시작일과 종료일을 포함한 날짜 목록을 반환한다.
//
// 매개변수:
//   - startInclusive: DatesInclusive에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - endInclusive: DatesInclusive에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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
