package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// TemperatureUnit  절대 온도와 온도 차이의 표현 단위입니다.
type TemperatureUnit struct {
	name   string
	suffix string
}

var (
	// KelvinUnit  Kelvin 표현 단위입니다.
	KelvinUnit = TemperatureUnit{name: "kelvin", suffix: "K"}
	// CelsiusUnit  Celsius 표현 단위입니다.
	CelsiusUnit = TemperatureUnit{name: "celsius", suffix: "degC"}
	// FahrenheitUnit  Fahrenheit 표현 단위입니다.
	FahrenheitUnit = TemperatureUnit{name: "fahrenheit", suffix: "degF"}
)

var temperatureUnits = []TemperatureUnit{FahrenheitUnit, CelsiusUnit, KelvinUnit}

// Name  온도 단위 이름을 반환합니다.
func (u TemperatureUnit) Name() string {
	return u.name
}

// Suffix  온도 단위 suffix를 반환합니다.
func (u TemperatureUnit) Suffix() string {
	return u.suffix
}

// Temperature  Kelvin 기준으로 저장되는 절대 온도입니다.
type Temperature struct {
	kelvin float64
}

// TemperatureDelta  Kelvin 기준으로 저장되는 온도 차이입니다.
type TemperatureDelta struct {
	kelvin float64
}

// Kelvin  Kelvin 값으로 절대 온도를 생성합니다.
func Kelvin(value float64) (Temperature, error) {
	if !finite(value) {
		return Temperature{}, fmt.Errorf("%w: temperature must be finite", ErrInvalidMeasure)
	}
	return Temperature{kelvin: value}, nil
}

// MustKelvin  Kelvin 생성 실패 시 panic을 발생시킵니다.
func MustKelvin(value float64) Temperature {
	temperature, err := Kelvin(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Celsius  Celsius 값으로 절대 온도를 생성합니다.
func Celsius(value float64) (Temperature, error) {
	return Kelvin(value + 273.15)
}

// MustCelsius  Celsius 생성 실패 시 panic을 발생시킵니다.
func MustCelsius(value float64) Temperature {
	temperature, err := Celsius(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// Fahrenheit  Fahrenheit 값으로 절대 온도를 생성합니다.
func Fahrenheit(value float64) (Temperature, error) {
	return Kelvin((value-32)*5/9 + 273.15)
}

// MustFahrenheit  Fahrenheit 생성 실패 시 panic을 발생시킵니다.
func MustFahrenheit(value float64) Temperature {
	temperature, err := Fahrenheit(value)
	if err != nil {
		panic(err)
	}
	return temperature
}

// KelvinDelta  Kelvin 온도 차이를 생성합니다.
func KelvinDelta(value float64) (TemperatureDelta, error) {
	if !finite(value) {
		return TemperatureDelta{}, fmt.Errorf("%w: temperature delta must be finite", ErrInvalidMeasure)
	}
	return TemperatureDelta{kelvin: value}, nil
}

// MustKelvinDelta  Kelvin 온도 차이 생성 실패 시 panic을 발생시킵니다.
func MustKelvinDelta(value float64) TemperatureDelta {
	delta, err := KelvinDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// CelsiusDelta  Celsius 온도 차이를 생성합니다.
func CelsiusDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value)
}

// MustCelsiusDelta  Celsius 온도 차이 생성 실패 시 panic을 발생시킵니다.
func MustCelsiusDelta(value float64) TemperatureDelta {
	delta, err := CelsiusDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// FahrenheitDelta  Fahrenheit 온도 차이를 생성합니다.
func FahrenheitDelta(value float64) (TemperatureDelta, error) {
	return KelvinDelta(value * 5 / 9)
}

// MustFahrenheitDelta  Fahrenheit 온도 차이 생성 실패 시 panic을 발생시킵니다.
func MustFahrenheitDelta(value float64) TemperatureDelta {
	delta, err := FahrenheitDelta(value)
	if err != nil {
		panic(err)
	}
	return delta
}

// InKelvin  절대 온도를 Kelvin으로 반환합니다.
func (t Temperature) InKelvin() float64 {
	return t.kelvin
}

// InCelsius  절대 온도를 Celsius로 반환합니다.
func (t Temperature) InCelsius() float64 {
	return t.kelvin - 273.15
}

// InFahrenheit  절대 온도를 Fahrenheit로 반환합니다.
func (t Temperature) InFahrenheit() float64 {
	return (t.kelvin-273.15)*9/5 + 32
}

// InKelvin  온도 차이를 Kelvin으로 반환합니다.
func (d TemperatureDelta) InKelvin() float64 {
	return d.kelvin
}

// InCelsius  온도 차이를 Celsius로 반환합니다.
func (d TemperatureDelta) InCelsius() float64 {
	return d.kelvin
}

// InFahrenheit  온도 차이를 Fahrenheit로 반환합니다.
func (d TemperatureDelta) InFahrenheit() float64 {
	return d.kelvin * 9 / 5
}

// Add  절대 온도에 온도 차이를 더합니다.
func (t Temperature) Add(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin + delta.kelvin}
}

// Sub  절대 온도에서 온도 차이를 뺍니다.
func (t Temperature) Sub(delta TemperatureDelta) Temperature {
	return Temperature{kelvin: t.kelvin - delta.kelvin}
}

// Delta  두 절대 온도의 차이를 반환합니다.
func (t Temperature) Delta(other Temperature) TemperatureDelta {
	return TemperatureDelta{kelvin: t.kelvin - other.kelvin}
}

// Compare  두 절대 온도를 비교합니다.
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

// Format  절대 온도를 지정 단위로 포맷합니다.
func (t Temperature) Format(unit TemperatureUnit) (string, error) {
	if !finite(t.kelvin) {
		return "", fmt.Errorf("%w: temperature must be finite", ErrInvalidMeasure)
	}
	switch unit {
	case KelvinUnit:
		return renderNumber(t.InKelvin()) + " " + unit.suffix, nil
	case CelsiusUnit:
		return renderNumber(t.InCelsius()) + " " + unit.suffix, nil
	case FahrenheitUnit:
		return renderNumber(t.InFahrenheit()) + " " + unit.suffix, nil
	default:
		return "", fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// Format  온도 차이를 지정 단위로 포맷합니다.
func (d TemperatureDelta) Format(unit TemperatureUnit) (string, error) {
	if !finite(d.kelvin) {
		return "", fmt.Errorf("%w: temperature delta must be finite", ErrInvalidMeasure)
	}
	switch unit {
	case KelvinUnit:
		return renderNumber(d.InKelvin()) + " " + unit.suffix, nil
	case CelsiusUnit:
		return renderNumber(d.InCelsius()) + " " + unit.suffix, nil
	case FahrenheitUnit:
		return renderNumber(d.InFahrenheit()) + " " + unit.suffix, nil
	default:
		return "", fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// String  절대 온도를 Celsius 기준 디버그 문자열로 반환합니다.
func (t Temperature) String() string {
	text, err := t.Format(CelsiusUnit)
	if err != nil {
		return "<invalid temperature>"
	}
	return text
}

// String  온도 차이를 Celsius 기준 디버그 문자열로 반환합니다.
func (d TemperatureDelta) String() string {
	text, err := d.Format(CelsiusUnit)
	if err != nil {
		return "<invalid temperature delta>"
	}
	return text
}

// ParseTemperature  절대 온도 문자열을 파싱합니다.
func ParseTemperature(text string) (Temperature, error) {
	value, unit, err := parseTemperatureValue(text)
	if err != nil {
		return Temperature{}, err
	}
	switch unit {
	case KelvinUnit:
		return Kelvin(value)
	case CelsiusUnit:
		return Celsius(value)
	case FahrenheitUnit:
		return Fahrenheit(value)
	default:
		return Temperature{}, fmt.Errorf("%w: unknown temperature unit", ErrInvalidUnit)
	}
}

// ParseTemperatureDelta  온도 차이 문자열을 파싱합니다.
func ParseTemperatureDelta(text string) (TemperatureDelta, error) {
	value, unit, err := parseTemperatureValue(text)
	if err != nil {
		return TemperatureDelta{}, err
	}
	switch unit {
	case KelvinUnit:
		return KelvinDelta(value)
	case CelsiusUnit:
		return CelsiusDelta(value)
	case FahrenheitUnit:
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
