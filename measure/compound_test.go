package measure_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/measure"
)

func TestCompoundUnitsAndMeasureOperations(t *testing.T) {
	areaUnit, err := measure.ProductUnit(measure.LengthMeter, measure.LengthMeter)
	if err != nil {
		t.Fatalf("ProductUnit failed: %v", err)
	}
	if areaUnit.Suffix() != "(m)^2" {
		t.Fatalf("unexpected area suffix: %q", areaUnit.Suffix())
	}

	speedUnit, err := measure.RatioUnit(measure.LengthMeter, measure.TimeSecond)
	if err != nil {
		t.Fatalf("RatioUnit failed: %v", err)
	}
	if speedUnit.Suffix() != "m/s" {
		t.Fatalf("unexpected speed suffix: %q", speedUnit.Suffix())
	}

	inverse, err := measure.InverseUnit(measure.TimeSecond)
	if err != nil {
		t.Fatalf("InverseUnit failed: %v", err)
	}
	if inverse.Suffix() != "1/s" {
		t.Fatalf("unexpected inverse suffix: %q", inverse.Suffix())
	}

	length := measure.Must(10, measure.LengthMeter)
	width := measure.Must(2, measure.LengthMeter)
	area, err := measure.Mul(length, width)
	if err != nil {
		t.Fatalf("Mul failed: %v", err)
	}
	assertMeasureIn(t, area, areaUnit, 20)

	elapsed := measure.Must(2, measure.TimeSecond)
	speed, err := measure.Div(length, elapsed)
	if err != nil {
		t.Fatalf("Div failed: %v", err)
	}
	assertMeasureIn(t, speed, speedUnit, 5)

	restoredLength, err := measure.MulRatioByDenominator(speed, elapsed, measure.LengthMeter)
	if err != nil {
		t.Fatalf("MulRatioByDenominator failed: %v", err)
	}
	assertMeasureIn(t, restoredLength, measure.LengthMeter, 10)

	restoredWidth, err := measure.DivProductByLeft(area, length, measure.LengthMeter)
	if err != nil {
		t.Fatalf("DivProductByLeft failed: %v", err)
	}
	assertMeasureIn(t, restoredWidth, measure.LengthMeter, 2)

	if _, err := measure.Div(length, measure.Must(0, measure.TimeSecond)); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected zero divisor error, got %v", err)
	}
	if _, err := measure.MulRatioByDenominator(speed, elapsed, measure.Unit[measure.Length]{}); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected invalid result unit error, got %v", err)
	}
	if _, err := measure.DivProductByLeft(area, length, measure.Unit[measure.Length]{}); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected invalid result unit error, got %v", err)
	}
}

func TestSourceParityNamedHelpers(t *testing.T) {
	width := measure.Must(10, measure.LengthMeter)
	height := measure.Must(2, measure.LengthMeter)
	area, err := measure.AreaFromLength(width, height)
	if err != nil {
		t.Fatalf("AreaFromLength failed: %v", err)
	}
	assertMeasureIn(t, area, measure.AreaSquareMeter, 20)

	volume, err := measure.VolumeFromAreaLength(area, measure.Must(3, measure.LengthMeter))
	if err != nil {
		t.Fatalf("VolumeFromAreaLength failed: %v", err)
	}
	assertMeasureIn(t, volume, measure.VolumeCubicMeter, 60)

	length, err := measure.LengthFromVolumeArea(volume, area)
	if err != nil {
		t.Fatalf("LengthFromVolumeArea failed: %v", err)
	}
	assertMeasureIn(t, length, measure.LengthMeter, 3)

	areaAgain, err := measure.AreaFromVolumeLength(volume, length)
	if err != nil {
		t.Fatalf("AreaFromVolumeLength failed: %v", err)
	}
	assertMeasureIn(t, areaAgain, measure.AreaSquareMeter, 20)

	velocity, err := measure.VelocityFromLengthTime(measure.Must(100, measure.LengthMeter), measure.Must(10, measure.TimeSecond))
	if err != nil {
		t.Fatalf("VelocityFromLengthTime failed: %v", err)
	}
	assertMeasureIn(t, velocity, measure.VelocityMeterPerSecond, 10)

	distance, err := measure.LengthFromVelocityTime(velocity, measure.Must(3, measure.TimeSecond))
	if err != nil {
		t.Fatalf("LengthFromVelocityTime failed: %v", err)
	}
	assertMeasureIn(t, distance, measure.LengthMeter, 30)

	power, err := measure.PowerFromEnergyTime(measure.Must(7200, measure.EnergyJoule), measure.Must(2, measure.TimeSecond))
	if err != nil {
		t.Fatalf("PowerFromEnergyTime failed: %v", err)
	}
	assertMeasureIn(t, power, measure.PowerWatt, 3600)

	energy, err := measure.EnergyFromPowerTime(measure.Must(2, measure.PowerKilowatt), measure.Must(3, measure.TimeHour))
	if err != nil {
		t.Fatalf("EnergyFromPowerTime failed: %v", err)
	}
	assertMeasureIn(t, energy, measure.EnergyJoule, 21_600_000)
}
