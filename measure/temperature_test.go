package measure_test

import (
	"errors"
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/measure"
)

func TestTemperatureConversionsAndDeltas(t *testing.T) {
	celsius, err := measure.Celsius(100)
	if err != nil {
		t.Fatalf("Celsius failed: %v", err)
	}
	assertClose(t, celsius.InKelvin(), 373.15)
	assertClose(t, celsius.InFahrenheit(), 212)

	fahrenheit, err := measure.Fahrenheit(32)
	if err != nil {
		t.Fatalf("Fahrenheit failed: %v", err)
	}
	assertClose(t, fahrenheit.InCelsius(), 0)

	delta := measure.MustCelsiusDelta(10)
	raised := fahrenheit.Add(delta)
	assertClose(t, raised.InCelsius(), 10)
	assertClose(t, raised.Delta(fahrenheit).InFahrenheit(), 18)

	text, err := raised.Format(measure.CelsiusUnit)
	if err != nil {
		t.Fatalf("temperature format failed: %v", err)
	}
	if text != "10.0 degC" {
		t.Fatalf("unexpected temperature format: %q", text)
	}
}

func TestTemperatureParsing(t *testing.T) {
	absolute, err := measure.ParseTemperature("25 degC")
	if err != nil {
		t.Fatalf("ParseTemperature failed: %v", err)
	}
	assertClose(t, absolute.InKelvin(), 298.15)

	delta, err := measure.ParseTemperatureDelta("18 degF")
	if err != nil {
		t.Fatalf("ParseTemperatureDelta failed: %v", err)
	}
	assertClose(t, delta.InCelsius(), 10)

	if _, err := measure.ParseTemperature("NaN degC"); !errors.Is(err, measure.ErrInvalidParse) {
		t.Fatalf("expected invalid parse, got %v", err)
	}
	if _, err := measure.ParseTemperature("10 nope"); !errors.Is(err, measure.ErrInvalidParse) {
		t.Fatalf("expected invalid parse for unknown temperature suffix, got %v", err)
	}
	if _, err := measure.ParseTemperatureDelta("+Inf degC"); !errors.Is(err, measure.ErrInvalidParse) {
		t.Fatalf("expected invalid delta parse, got %v", err)
	}
	if _, err := measure.Kelvin(math.Inf(1)); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected invalid temperature, got %v", err)
	}
	overflow := measure.MustKelvin(math.MaxFloat64).Add(measure.MustKelvinDelta(math.MaxFloat64))
	if _, err := overflow.Format(measure.KelvinUnit); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected overflow format error, got %v", err)
	}
}

func TestAngleInverseFunctionsReturnErrors(t *testing.T) {
	if _, err := measure.ASin(2); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected ASin domain error, got %v", err)
	}
	if _, err := measure.ACos(2); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected ACos domain error, got %v", err)
	}
	if angle, err := measure.ATan(1); err != nil {
		t.Fatalf("ATan failed: %v", err)
	} else {
		assertMeasureIn(t, angle, measure.AngleRadian, math.Pi/4)
	}
}
