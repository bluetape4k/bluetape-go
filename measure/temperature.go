package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// TemperatureUnit struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// KelvinUnit KelvinUnit 공개 API의 동작을 수행한다.
func KelvinUnit() TemperatureUnit { return kelvinUnit }

// CelsiusUnit CelsiusUnit 공개 API의 동작을 수행한다.
func CelsiusUnit() TemperatureUnit { return celsiusUnit }

// FahrenheitUnit FahrenheitUnit 공개 API의 동작을 수행한다.
func FahrenheitUnit() TemperatureUnit { return fahrenheitUnit }

// Name Name 공개 API의 동작을 수행한다.
func (u TemperatureUnit) Name() string {
	return u.name
}

// Suffix Suffix 공개 API의 동작을 수행한다.
func (u TemperatureUnit) Suffix() string {
	return u.suffix
}

// Temperature struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Temperature struct {
	kelvin float64
}

// TemperatureDelta struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TemperatureDelta struct {
	kelvin float64
}

// Kelvin Kelvin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Kelvin 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Kelvin(value float64) (Temperature, error) {
	if !finite(value) {
		return Temperature{}, fmt.Errorf("%w: temperature must be finite", ErrInvalidMeasure)
	}
	return Temperature{kelvin: value}, nil
}

// MustKelvin MustKelvin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustKelvin 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustKelvin(value float64) Temperature {
	temperature, err := Kelvin(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Celsius Celsius 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Celsius 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Celsius(value float64) (Temperature, error) {
	return Kelvin(value + 273.15)
}

// MustCelsius MustCelsius 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustCelsius 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustCelsius(value float64) Temperature {
	temperature, err := Celsius(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Fahrenheit Fahrenheit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Fahrenheit 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Fahrenheit(value float64) (Temperature, error) {
	return Kelvin((value-32)*5/9 + 273.15)
}

// MustFahrenheit MustFahrenheit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustFahrenheit 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustFahrenheit(value float64) Temperature {
	temperature, err := Fahrenheit(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// KelvinDelta KelvinDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: KelvinDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func KelvinDelta(value float64) (TemperatureDelta, error) {
	if !finite(value) {
		return TemperatureDelta{}, fmt.Errorf("%w: temperature delta must be finite", ErrInvalidMeasure)
	}
	return TemperatureDelta{kelvin: value}, nil
}

// MustKelvinDelta MustKelvinDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustKelvinDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustKelvinDelta(value float64) TemperatureDelta {
	delta, err := KelvinDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// CelsiusDelta CelsiusDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: CelsiusDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CelsiusDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value)
}

// MustCelsiusDelta MustCelsiusDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustCelsiusDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustCelsiusDelta(value float64) TemperatureDelta {
	delta, err := CelsiusDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// FahrenheitDelta FahrenheitDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: FahrenheitDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func FahrenheitDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value * 5 / 9)
}

// MustFahrenheitDelta MustFahrenheitDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustFahrenheitDelta 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustFahrenheitDelta(value float64) TemperatureDelta {
	delta, err := FahrenheitDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// InKelvin InKelvin 공개 API의 동작을 수행한다.
func (t Temperature) InKelvin() float64 {
	return t.kelvin
}

// InCelsius InCelsius 공개 API의 동작을 수행한다.
func (t Temperature) InCelsius() float64 {
	return t.kelvin - 273.15
}

// InFahrenheit InFahrenheit 공개 API의 동작을 수행한다.
func (t Temperature) InFahrenheit() float64 {
	return (t.kelvin-273.15)*9/5 + 32
}

// InKelvin InKelvin 공개 API의 동작을 수행한다.
func (d TemperatureDelta) InKelvin() float64 {
	return d.kelvin
}

// InCelsius InCelsius 공개 API의 동작을 수행한다.
func (d TemperatureDelta) InCelsius() float64 {
	return d.kelvin
}

// InFahrenheit InFahrenheit 공개 API의 동작을 수행한다.
func (d TemperatureDelta) InFahrenheit() float64 {
	return d.kelvin * 9 / 5
}

// Add Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - delta: Add 동작에 필요한 delta 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (t Temperature) Add(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin + delta.kelvin}
}

// Sub Sub 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - delta: Sub 동작에 필요한 delta 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (t Temperature) Sub(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin - delta.kelvin}
}

// Delta Delta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Delta 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (t Temperature) Delta(other Temperature) TemperatureDelta {
	return TemperatureDelta{kelvin: t.kelvin - other.kelvin}
}

// Compare Compare 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Compare 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// Format Format 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: Format 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Format Format 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: Format 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// String String 공개 API의 동작을 수행한다.
func (t Temperature) String() string {
	text, err := t.Format(celsiusUnit)
	if err != nil {
		return "<invalid temperature>"
	}
	return text
}

// String String 공개 API의 동작을 수행한다.
func (d TemperatureDelta) String() string {
	text, err := d.Format(celsiusUnit)
	if err != nil {
		return "<invalid temperature delta>"
	}
	return text
}

// ParseTemperature ParseTemperature 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseTemperature가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// ParseTemperatureDelta ParseTemperatureDelta 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseTemperatureDelta가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
