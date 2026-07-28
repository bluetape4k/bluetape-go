package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// TemperatureUnit 패키지에서 공개하는 구조체다.
type TemperatureUnit struct {
	name   string
	suffix string
}

var (
	kelvinUnit     = TemperatureUnit{name: "kelvin", suffix: "K"}
	celsiusUnit    = TemperatureUnit{name: "celsius", suffix: "degC"}
	fahrenheitUnit = TemperatureUnit{name: "fahrenheit", suffix: "degF"}
)

var temperatureUnits = []TemperatureUnit{fahrenheitUnit, celsiusUnit, kelvinUnit}

// KelvinUnit Kelvin 단위를 반환한다.
func KelvinUnit() TemperatureUnit { return kelvinUnit }

// CelsiusUnit Celsius 단위를 반환한다.
func CelsiusUnit() TemperatureUnit { return celsiusUnit }

// FahrenheitUnit Fahrenheit 단위를 반환한다.
func FahrenheitUnit() TemperatureUnit { return fahrenheitUnit }

// Name 식별자 이름을 반환한다.
func (u TemperatureUnit) Name() string {
	return u.name
}

// Suffix 값에 사용할 suffix 문자열을 반환한다.
func (u TemperatureUnit) Suffix() string {
	return u.suffix
}

// Temperature 패키지에서 공개하는 구조체다.
type Temperature struct {
	kelvin float64
}

// TemperatureDelta 패키지에서 공개하는 구조체다.
type TemperatureDelta struct {
	kelvin float64
}

// Kelvin Kelvin 온도 값을 만든다.
//
// 매개변수:
//   - value: Kelvin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Kelvin(value float64) (Temperature, error) {
	if !finite(value) {
		return Temperature{}, fmt.Errorf("%w: temperature must be finite", ErrInvalidMeasure)
	}
	return Temperature{kelvin: value}, nil
}

// MustKelvin 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustKelvin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustKelvin(value float64) Temperature {
	temperature, err := Kelvin(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Celsius Celsius 온도 값을 만든다.
//
// 매개변수:
//   - value: Celsius에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Celsius(value float64) (Temperature, error) {
	return Kelvin(value + 273.15)
}

// MustCelsius 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustCelsius에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustCelsius(value float64) Temperature {
	temperature, err := Celsius(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Fahrenheit Fahrenheit 온도 값을 만든다.
//
// 매개변수:
//   - value: Fahrenheit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Fahrenheit(value float64) (Temperature, error) {
	return Kelvin((value-32)*5/9 + 273.15)
}

// MustFahrenheit 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustFahrenheit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustFahrenheit(value float64) Temperature {
	temperature, err := Fahrenheit(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// KelvinDelta 온도 값을 생성한다.
//
// 매개변수:
//   - value: KelvinDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func KelvinDelta(value float64) (TemperatureDelta, error) {
	if !finite(value) {
		return TemperatureDelta{}, fmt.Errorf("%w: temperature delta must be finite", ErrInvalidMeasure)
	}
	return TemperatureDelta{kelvin: value}, nil
}

// MustKelvinDelta 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustKelvinDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustKelvinDelta(value float64) TemperatureDelta {
	delta, err := KelvinDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// CelsiusDelta 온도 값을 생성한다.
//
// 매개변수:
//   - value: CelsiusDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func CelsiusDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value)
}

// MustCelsiusDelta 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustCelsiusDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustCelsiusDelta(value float64) TemperatureDelta {
	delta, err := CelsiusDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// FahrenheitDelta 온도 값을 생성한다.
//
// 매개변수:
//   - value: FahrenheitDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func FahrenheitDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value * 5 / 9)
}

// MustFahrenheitDelta 온도 값 생성에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustFahrenheitDelta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustFahrenheitDelta(value float64) TemperatureDelta {
	delta, err := FahrenheitDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// InKelvin 온도를 Kelvin 값으로 반환한다.
func (t Temperature) InKelvin() float64 {
	return t.kelvin
}

// InCelsius 온도를 Celsius 값으로 반환한다.
func (t Temperature) InCelsius() float64 {
	return t.kelvin - 273.15
}

// InFahrenheit 온도를 Fahrenheit 값으로 반환한다.
func (t Temperature) InFahrenheit() float64 {
	return (t.kelvin-273.15)*9/5 + 32
}

// InKelvin 온도를 Kelvin 값으로 반환한다.
func (d TemperatureDelta) InKelvin() float64 {
	return d.kelvin
}

// InCelsius 온도를 Celsius 값으로 반환한다.
func (d TemperatureDelta) InCelsius() float64 {
	return d.kelvin
}

// InFahrenheit 온도를 Fahrenheit 값으로 반환한다.
func (d TemperatureDelta) InFahrenheit() float64 {
	return d.kelvin * 9 / 5
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - delta: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t Temperature) Add(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin + delta.kelvin}
}

// Sub 현재 값에서 입력 값을 뺀 결과를 반환한다.
//
// 매개변수:
//   - delta: Sub에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t Temperature) Sub(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin - delta.kelvin}
}

// Delta 두 온도 값의 차이를 반환한다.
//
// 매개변수:
//   - other: Delta에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t Temperature) Delta(other Temperature) TemperatureDelta {
	return TemperatureDelta{kelvin: t.kelvin - other.kelvin}
}

// Compare 두 값을 정렬 순서로 비교한다.
//
// 매개변수:
//   - other: Compare에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t Temperature) Compare(other Temperature) int {
	switch {
	case t.kelvin < other.kelvin:
		return -1
	case t.kelvin > other.kelvin:
		return 1
	default:
		return 0
	}
}

// Format 값을 지정한 형식의 문자열로 변환한다.
//
// 매개변수:
//   - unit: Format에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (t Temperature) Format(unit TemperatureUnit) (string, error) {
	if !finite(t.kelvin) {
		return "", fmt.Errorf("%w: temperature must be finite", ErrInvalidMeasure)
	}
	switch unit {
	case kelvinUnit:
		return renderNumber(t.InKelvin()) + " " + unit.suffix, nil
	case celsiusUnit:
		return renderNumber(t.InCelsius()) + " " + unit.suffix, nil
	case fahrenheitUnit:
		return renderNumber(t.InFahrenheit()) + " " + unit.suffix, nil
	default:
		return "", fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// Format 값을 지정한 형식의 문자열로 변환한다.
//
// 매개변수:
//   - unit: Format에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (d TemperatureDelta) Format(unit TemperatureUnit) (string, error) {
	if !finite(d.kelvin) {
		return "", fmt.Errorf("%w: temperature delta must be finite", ErrInvalidMeasure)
	}
	switch unit {
	case kelvinUnit:
		return renderNumber(d.InKelvin()) + " " + unit.suffix, nil
	case celsiusUnit:
		return renderNumber(d.InCelsius()) + " " + unit.suffix, nil
	case fahrenheitUnit:
		return renderNumber(d.InFahrenheit()) + " " + unit.suffix, nil
	default:
		return "", fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (t Temperature) String() string {
	text, err := t.Format(celsiusUnit)
	if err != nil {
		return "<invalid temperature>"
	}
	return text
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (d TemperatureDelta) String() string {
	text, err := d.Format(celsiusUnit)
	if err != nil {
		return "<invalid temperature delta>"
	}
	return text
}

// ParseTemperature 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseTemperature가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseTemperature(text string) (Temperature, error) {
	value, unit, err := parseTemperatureValue(text)
	if err != nil {
		return Temperature{}, err
	}
	switch unit {
	case kelvinUnit:
		return Kelvin(value)
	case celsiusUnit:
		return Celsius(value)
	case fahrenheitUnit:
		return Fahrenheit(value)
	default:
		return Temperature{}, fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// ParseTemperatureDelta 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseTemperatureDelta가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseTemperatureDelta(text string) (TemperatureDelta, error) {
	value, unit, err := parseTemperatureValue(text)
	if err != nil {
		return TemperatureDelta{}, err
	}
	switch unit {
	case kelvinUnit:
		return KelvinDelta(value)
	case celsiusUnit:
		return CelsiusDelta(value)
	case fahrenheitUnit:
		return FahrenheitDelta(value)
	default:
		return TemperatureDelta{}, fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

func parseTemperatureValue(text string) (float64, TemperatureUnit, error) {
	input := strings.TrimSpace(text)
	for _, unit := range temperatureUnits {
		if !strings.HasSuffix(input, unit.suffix) {
			continue
		}
		numberPart := strings.TrimSpace(strings.TrimSuffix(input, unit.suffix))
		if numberPart == "" {
			return 0, TemperatureUnit{}, ParseError{Input: text, Suffix: unit.suffix, Err: ErrInvalidParse}
		}
		value, err := strconv.ParseFloat(numberPart, 64)
		if err != nil || !finite(value) {
			return 0, TemperatureUnit{}, ParseError{Input: text, Suffix: unit.suffix, Err: ErrInvalidParse}
		}
		return value, unit, nil
	}
	return 0, TemperatureUnit{}, ParseError{Input: text, Err: fmt.Errorf("%w: unknown suffix", ErrInvalidUnit)}
}
