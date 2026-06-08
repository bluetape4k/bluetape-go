package measure_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/measure"
)

const tolerance = 1e-9

func assertClose(tb testing.TB, got, want float64) {
	tb.Helper()
	if math.Abs(got-want) > tolerance {
		tb.Fatalf("got %v, want %v", got, want)
	}
}

func assertMeasureIn[D any](tb testing.TB, value measure.Measure[D], unit measure.Unit[D], want float64) {
	tb.Helper()
	got, err := value.In(unit)
	if err != nil {
		tb.Fatalf("In failed: %v", err)
	}
	assertClose(tb, got, want)
}

func TestUnitAndRegistryValidation(t *testing.T) {
	if _, err := measure.NewUnit[measure.Length]("", "bad", 1); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit for blank name, got %v", err)
	}
	if _, err := measure.NewUnit[measure.Length]("bad", "bad", math.NaN()); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit for NaN ratio, got %v", err)
	}
	if _, err := measure.NewRegistry(measure.LengthMeter, measure.MustUnit[measure.Length]("duplicate meter", "m", 1)); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected duplicate suffix ErrInvalidUnit, got %v", err)
	}

	var registry measure.Registry[measure.Length]
	if _, err := measure.Parse("1 m", registry); !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("expected zero registry ErrInvalidUnit, got %v", err)
	}
}

func TestMeasureOperations(t *testing.T) {
	meters := measure.Must(500.0, measure.LengthMeter)
	kilometer := measure.Must(1.0, measure.LengthKilometer)

	sum, err := meters.Add(kilometer)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	assertMeasureIn(t, sum, measure.LengthMeter, 1500)
	if sum.Unit() != measure.LengthMeter {
		t.Fatalf("expected smaller result unit meter, got %v", sum.Unit())
	}

	diff, err := kilometer.Sub(meters)
	if err != nil {
		t.Fatalf("Sub failed: %v", err)
	}
	assertMeasureIn(t, diff, measure.LengthMeter, 500)

	doubled, err := meters.MulScalar(2)
	if err != nil {
		t.Fatalf("MulScalar failed: %v", err)
	}
	assertMeasureIn(t, doubled, measure.LengthMeter, 1000)

	rounded, err := measure.Must(10.26, measure.LengthMeter).ToNearest(0.1)
	if err != nil {
		t.Fatalf("ToNearest failed: %v", err)
	}
	assertMeasureIn(t, rounded, measure.LengthMeter, 10.3)

	for _, nearest := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := meters.ToNearest(nearest); !errors.Is(err, measure.ErrInvalidMeasure) {
			t.Fatalf("expected invalid nearest error for %v, got %v", nearest, err)
		}
	}
	if _, err := meters.DivScalar(0); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected DivScalar zero error, got %v", err)
	}

	formatted, err := kilometer.Format(measure.LengthMeter)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	if formatted != "1000.0 m" {
		t.Fatalf("unexpected format: %q", formatted)
	}
	if got := (measure.Measure[measure.Length]{}).String(); got != "<invalid measure>" {
		t.Fatalf("unexpected invalid string: %q", got)
	}

	overflow := measure.Must(math.MaxFloat64, measure.LengthKilometer)
	if _, err := overflow.BaseAmount(); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected BaseAmount overflow error, got %v", err)
	}
	if _, err := overflow.In(measure.LengthMeter); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected In overflow error, got %v", err)
	}
	if _, err := overflow.Format(measure.LengthMeter); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected Format overflow error, got %v", err)
	}
	if _, err := overflow.Human(measure.LengthMeter, measure.LengthKilometer); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected Human overflow error, got %v", err)
	}
	if _, err := overflow.Compare(measure.Must(1, measure.LengthMeter)); !errors.Is(err, measure.ErrInvalidMeasure) {
		t.Fatalf("expected Compare overflow error, got %v", err)
	}
}

func TestFamilyConversionsUseEveryBuiltInRatio(t *testing.T) {
	lengths := []struct {
		unit measure.Unit[measure.Length]
		base float64
	}{
		{measure.LengthMillimeter, 0.001},
		{measure.LengthCentimeter, 0.01},
		{measure.LengthMeter, 1},
		{measure.LengthKilometer, 1000},
		{measure.LengthInch, 0.0254},
		{measure.LengthFoot, 0.3048},
		{measure.LengthMile, 1609.344},
	}
	for _, tc := range lengths {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.LengthMeter, tc.base)
	}

	times := []struct {
		unit measure.Unit[measure.Time]
		base float64
	}{
		{measure.TimeMillisecond, 1},
		{measure.TimeSecond, 1000},
		{measure.TimeMinute, 60_000},
		{measure.TimeHour, 3_600_000},
	}
	for _, tc := range times {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.TimeMillisecond, tc.base)
	}

	masses := []struct {
		unit measure.Unit[measure.Mass]
		base float64
	}{
		{measure.MassGram, 1},
		{measure.MassKilogram, 1000},
		{measure.MassTon, 1_000_000},
	}
	for _, tc := range masses {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.MassGram, tc.base)
	}

	areas := []struct {
		unit measure.Unit[measure.Area]
		base float64
	}{
		{measure.AreaSquareMillimeter, 1e-6},
		{measure.AreaSquareCentimeter, 1e-4},
		{measure.AreaSquareMeter, 1},
		{measure.AreaSquareKilometer, 1e6},
	}
	for _, tc := range areas {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.AreaSquareMeter, tc.base)
	}

	volumes := []struct {
		unit measure.Unit[measure.Volume]
		base float64
	}{
		{measure.VolumeCubicMillimeter, 1e-9},
		{measure.VolumeCubicCentimeter, 1e-6},
		{measure.VolumeMilliliter, 1e-6},
		{measure.VolumeLiter, 1e-3},
		{measure.VolumeCubicMeter, 1},
	}
	for _, tc := range volumes {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.VolumeCubicMeter, tc.base)
	}

	storages := []struct {
		unit measure.Unit[measure.Storage]
		base float64
	}{
		{measure.StorageByte, 1},
		{measure.StorageKilobyte, 1024},
		{measure.StorageMegabyte, 1024 * 1024},
		{measure.StorageGigabyte, 1024 * 1024 * 1024},
		{measure.StorageTerabyte, math.Pow(1024, 4)},
		{measure.StoragePetabyte, math.Pow(1024, 5)},
		{measure.StorageExabyte, math.Pow(1024, 6)},
		{measure.StorageZettabyte, math.Pow(1024, 7)},
		{measure.StorageYottabyte, math.Pow(1024, 8)},
	}
	for _, tc := range storages {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.StorageByte, tc.base)
	}

	binaries := []struct {
		unit measure.Unit[measure.BinarySize]
		base float64
	}{
		{measure.BinaryByte, 1},
		{measure.BinaryKilobyte, 1000},
		{measure.BinaryMegabyte, 1_000_000},
		{measure.BinaryGigabyte, 1_000_000_000},
		{measure.BinaryTerabyte, 1_000_000_000_000},
		{measure.BinaryPetabyte, 1_000_000_000_000_000},
		{measure.BinaryKibibyte, 1024},
		{measure.BinaryMebibyte, 1024 * 1024},
		{measure.BinaryGibibyte, 1024 * 1024 * 1024},
		{measure.BinaryTebibyte, math.Pow(1024, 4)},
		{measure.BinaryPebibyte, math.Pow(1024, 5)},
		{measure.BinaryBit, 1.0 / 8.0},
		{measure.BinaryKilobit, 1000.0 / 8.0},
		{measure.BinaryMegabit, 1_000_000.0 / 8.0},
		{measure.BinaryGigabit, 1_000_000_000.0 / 8.0},
		{measure.BinaryTerabit, 1_000_000_000_000.0 / 8.0},
		{measure.BinaryPetabit, 1_000_000_000_000_000.0 / 8.0},
	}
	for _, tc := range binaries {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.BinaryByte, tc.base)
	}

	frequencies := []struct {
		unit measure.Unit[measure.Frequency]
		base float64
	}{
		{measure.FrequencyHertz, 1},
		{measure.FrequencyKilohertz, 1e3},
		{measure.FrequencyMegahertz, 1e6},
		{measure.FrequencyGigahertz, 1e9},
	}
	for _, tc := range frequencies {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.FrequencyHertz, tc.base)
	}

	energies := []struct {
		unit measure.Unit[measure.Energy]
		base float64
	}{
		{measure.EnergyJoule, 1},
		{measure.EnergyKilojoule, 1e3},
		{measure.EnergyMegajoule, 1e6},
		{measure.EnergyWattHour, 3600},
		{measure.EnergyKilowattHour, 3_600_000},
	}
	for _, tc := range energies {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.EnergyJoule, tc.base)
	}

	powers := []struct {
		unit measure.Unit[measure.Power]
		base float64
	}{
		{measure.PowerMilliwatt, 1e-3},
		{measure.PowerWatt, 1},
		{measure.PowerKilowatt, 1e3},
		{measure.PowerMegawatt, 1e6},
		{measure.PowerGigawatt, 1e9},
	}
	for _, tc := range powers {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.PowerWatt, tc.base)
	}

	pressures := []struct {
		unit measure.Unit[measure.Pressure]
		base float64
	}{
		{measure.PressurePascal, 1},
		{measure.PressureHectopascal, 100},
		{measure.PressureKilopascal, 1000},
		{measure.PressureMegapascal, 1_000_000},
		{measure.PressureGigapascal, 1_000_000_000},
		{measure.PressureBar, 100_000},
		{measure.PressureDecibar, 10_000},
		{measure.PressureMillibar, 100},
		{measure.PressureAtmosphere, 101_325},
		{measure.PressurePSI, 6894.757},
		{measure.PressureTorr, 101_325.0 / 760.0},
		{measure.PressureMillimeterMercury, 101_325.0 / 760.0},
	}
	for _, tc := range pressures {
		assertMeasureIn(t, measure.Must(1, tc.unit), measure.PressurePascal, tc.base)
	}

	assertMeasureIn(t, measure.Must(1, measure.AngleRadian), measure.AngleRadian, 1)
	assertMeasureIn(t, measure.Must(180, measure.AngleDegree), measure.AngleRadian, math.Pi)
	assertMeasureIn(t, measure.Must(1920, measure.GraphicsPixel), measure.GraphicsPixel, 1920)
	assertMeasureIn(t, measure.Must(36, measure.VelocityKilometerPerHour), measure.VelocityMeterPerSecond, 10)
	assertMeasureIn(t, measure.Must(9.8, measure.AccelerationMeterPerSecondSquared), measure.AccelerationMeterPerSecondSquared, 9.8)
}

func TestParseFamilies(t *testing.T) {
	length, err := measure.ParseLength("1.5 km")
	if err != nil {
		t.Fatalf("ParseLength failed: %v", err)
	}
	assertMeasureIn(t, length, measure.LengthMeter, 1500)

	cases := []struct {
		name string
		run  func() error
	}{
		{"time", func() error { _, err := measure.ParseTime("2 hr"); return err }},
		{"mass", func() error { _, err := measure.ParseMass("3 kg"); return err }},
		{"area", func() error { _, err := measure.ParseArea("4 m^2"); return err }},
		{"volume", func() error { _, err := measure.ParseVolume("5 L"); return err }},
		{"storage", func() error { _, err := measure.ParseStorage("6 GB"); return err }},
		{"binary", func() error { _, err := measure.ParseBinarySize("7 MiB"); return err }},
		{"frequency", func() error { _, err := measure.ParseFrequency("8 MHz"); return err }},
		{"energy", func() error { _, err := measure.ParseEnergy("9 kWh"); return err }},
		{"power", func() error { _, err := measure.ParsePower("10 kW"); return err }},
		{"pressure", func() error { _, err := measure.ParsePressure("11 psi"); return err }},
		{"angle", func() error { _, err := measure.ParseAngle("90deg"); return err }},
		{"graphics", func() error { _, err := measure.ParseGraphicsLength("12 px"); return err }},
		{"velocity", func() error { _, err := measure.ParseVelocity("13 km/hr"); return err }},
		{"acceleration", func() error { _, err := measure.ParseAcceleration("14 m/s^2"); return err }},
	}
	for _, tc := range cases {
		if err := tc.run(); err != nil {
			t.Fatalf("%s parse failed: %v", tc.name, err)
		}
	}

	parserFailures := []struct {
		name string
		run  func(string) error
	}{
		{"length", func(text string) error { _, err := measure.ParseLength(text); return err }},
		{"time", func(text string) error { _, err := measure.ParseTime(text); return err }},
		{"mass", func(text string) error { _, err := measure.ParseMass(text); return err }},
		{"area", func(text string) error { _, err := measure.ParseArea(text); return err }},
		{"volume", func(text string) error { _, err := measure.ParseVolume(text); return err }},
		{"storage", func(text string) error { _, err := measure.ParseStorage(text); return err }},
		{"binary", func(text string) error { _, err := measure.ParseBinarySize(text); return err }},
		{"frequency", func(text string) error { _, err := measure.ParseFrequency(text); return err }},
		{"energy", func(text string) error { _, err := measure.ParseEnergy(text); return err }},
		{"power", func(text string) error { _, err := measure.ParsePower(text); return err }},
		{"pressure", func(text string) error { _, err := measure.ParsePressure(text); return err }},
		{"angle", func(text string) error { _, err := measure.ParseAngle(text); return err }},
		{"graphics", func(text string) error { _, err := measure.ParseGraphicsLength(text); return err }},
		{"velocity", func(text string) error { _, err := measure.ParseVelocity(text); return err }},
		{"acceleration", func(text string) error { _, err := measure.ParseAcceleration(text); return err }},
	}
	for _, parser := range parserFailures {
		for _, text := range []string{"", "m", "NaN m", "+Inf m", "1 parsec"} {
			if err := parser.run(text); !errors.Is(err, measure.ErrInvalidParse) {
				t.Fatalf("%s: expected ErrInvalidParse for %q, got %v", parser.name, text, err)
			}
		}
	}
}

func TestStorageAndBinarySuffixesAreRegistryScoped(t *testing.T) {
	storage, err := measure.ParseStorage("1 MB")
	if err != nil {
		t.Fatalf("ParseStorage failed: %v", err)
	}
	binary, err := measure.ParseBinarySize("1 MB")
	if err != nil {
		t.Fatalf("ParseBinarySize failed: %v", err)
	}
	assertMeasureIn(t, storage, measure.StorageByte, 1024*1024)
	assertMeasureIn(t, binary, measure.BinaryByte, 1_000_000)
}

func TestDurationHelpers(t *testing.T) {
	value := measure.FromDuration(1500 * time.Microsecond)
	assertMeasureIn(t, value, measure.TimeMillisecond, 1.5)

	duration, err := measure.Duration(value)
	if err != nil {
		t.Fatalf("Duration failed: %v", err)
	}
	if duration != 1500*time.Microsecond {
		t.Fatalf("got %v", duration)
	}
}
