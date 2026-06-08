package measure_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/measure"
)

func ExampleMeasure_Format() {
	distance := measure.Must(1500, measure.LengthMeter)
	text, _ := distance.Format(measure.LengthKilometer)
	fmt.Println(text)
	// Output:
	// 1.5 km
}

func ExampleParseTemperature() {
	temperature, _ := measure.ParseTemperature("25 degC")
	fmt.Printf("%.2f K\n", temperature.InKelvin())
	// Output:
	// 298.15 K
}

func ExampleVelocityFromLengthTime() {
	speed, _ := measure.VelocityFromLengthTime(
		measure.Must(100, measure.LengthMeter),
		measure.Must(10, measure.TimeSecond),
	)
	text, _ := speed.Format(measure.VelocityMeterPerSecond)
	fmt.Println(text)
	// Output:
	// 10.0 m/s
}

func ExampleEnergyFromPowerTime() {
	energy, _ := measure.EnergyFromPowerTime(
		measure.Must(2, measure.PowerKilowatt),
		measure.Must(3, measure.TimeHour),
	)
	text, _ := energy.Format(measure.EnergyKilowattHour)
	fmt.Println(text)
	// Output:
	// 6.0 kWh
}
