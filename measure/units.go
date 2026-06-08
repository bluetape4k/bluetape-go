package measure

import (
	"math"
	"time"
)

var (
	// LengthMillimeter  millimeter 단위입니다.
	LengthMillimeter = MustUnit[Length]("millimeter", "mm", 0.001)
	// LengthCentimeter  centimeter 단위입니다.
	LengthCentimeter = MustUnit[Length]("centimeter", "cm", 0.01)
	// LengthMeter  meter 기준 단위입니다.
	LengthMeter = MustUnit[Length]("meter", "m", 1)
	// LengthKilometer  kilometer 단위입니다.
	LengthKilometer = MustUnit[Length]("kilometer", "km", 1000)
	// LengthInch  inch 단위입니다.
	LengthInch = MustUnit[Length]("inch", "in", 0.0254)
	// LengthFoot  foot 단위입니다.
	LengthFoot = MustUnit[Length]("foot", "ft", 0.3048)
	// LengthMile  mile 단위입니다.
	LengthMile = MustUnit[Length]("mile", "mi", 1609.344)
	// LengthRegistry  길이 단위 registry입니다.
	LengthRegistry = MustRegistry(LengthMillimeter, LengthCentimeter, LengthMeter, LengthKilometer, LengthInch, LengthFoot, LengthMile)
)

var (
	// TimeMillisecond  millisecond 기준 단위입니다.
	TimeMillisecond = MustUnit[Time]("millisecond", "ms", 1)
	// TimeSecond  second 단위입니다.
	TimeSecond = MustUnit[Time]("second", "s", 1000)
	// TimeMinute  minute 단위입니다.
	TimeMinute = MustUnit[Time]("minute", "min", 60_000)
	// TimeHour  hour 단위입니다.
	TimeHour = MustUnit[Time]("hour", "hr", 3_600_000)
	// TimeRegistry  시간 단위 registry입니다.
	TimeRegistry = MustRegistry(TimeMillisecond, TimeSecond, TimeMinute, TimeHour)
)

var (
	// MassGram  gram 기준 단위입니다.
	MassGram = MustUnit[Mass]("gram", "g", 1)
	// MassKilogram  kilogram 단위입니다.
	MassKilogram = MustUnit[Mass]("kilogram", "kg", 1000)
	// MassTon  metric ton 단위입니다.
	MassTon = MustUnit[Mass]("ton", "ton", 1_000_000)
	// MassRegistry  질량 단위 registry입니다.
	MassRegistry = MustRegistry(MassGram, MassKilogram, MassTon)
)

var (
	// AreaSquareMillimeter  square millimeter 단위입니다.
	AreaSquareMillimeter = MustUnit[Area]("square millimeter", "mm^2", 1e-6)
	// AreaSquareCentimeter  square centimeter 단위입니다.
	AreaSquareCentimeter = MustUnit[Area]("square centimeter", "cm^2", 1e-4)
	// AreaSquareMeter  square meter 기준 단위입니다.
	AreaSquareMeter = MustUnit[Area]("square meter", "m^2", 1)
	// AreaSquareKilometer  square kilometer 단위입니다.
	AreaSquareKilometer = MustUnit[Area]("square kilometer", "km^2", 1e6)
	// AreaRegistry  면적 단위 registry입니다.
	AreaRegistry = MustRegistry(AreaSquareMillimeter, AreaSquareCentimeter, AreaSquareMeter, AreaSquareKilometer)
)

var (
	// VolumeCubicMillimeter  cubic millimeter 단위입니다.
	VolumeCubicMillimeter = MustUnit[Volume]("cubic millimeter", "mm^3", 1e-9)
	// VolumeCubicCentimeter  cubic centimeter 단위입니다.
	VolumeCubicCentimeter = MustUnit[Volume]("cubic centimeter", "cm^3", 1e-6)
	// VolumeMilliliter  milliliter 단위입니다.
	VolumeMilliliter = MustUnit[Volume]("milliliter", "mL", 1e-6)
	// VolumeLiter  liter 단위입니다.
	VolumeLiter = MustUnit[Volume]("liter", "L", 1e-3)
	// VolumeCubicMeter  cubic meter 기준 단위입니다.
	VolumeCubicMeter = MustUnit[Volume]("cubic meter", "m^3", 1)
	// VolumeRegistry  부피 단위 registry입니다.
	VolumeRegistry = MustRegistry(VolumeCubicMillimeter, VolumeCubicCentimeter, VolumeMilliliter, VolumeLiter, VolumeCubicMeter)
)

var (
	// StorageByte byte 기준 단위입니다.
	StorageByte = MustUnit[Storage]("byte", "B", 1)
	// StorageKilobyte 1024 byte 단위입니다.
	StorageKilobyte = MustUnit[Storage]("kilobyte", "KB", 1024)
	// StorageMegabyte 1024^2 byte 단위입니다.
	StorageMegabyte = MustUnit[Storage]("megabyte", "MB", 1024*1024)
	// StorageGigabyte 1024^3 byte 단위입니다.
	StorageGigabyte = MustUnit[Storage]("gigabyte", "GB", 1024*1024*1024)
	// StorageTerabyte  1024^4 byte 단위입니다.
	StorageTerabyte = MustUnit[Storage]("terabyte", "TB", 1024*StorageGigabyte.Ratio())
	// StoragePetabyte  1024^5 byte 단위입니다.
	StoragePetabyte = MustUnit[Storage]("petabyte", "PB", 1024*StorageTerabyte.Ratio())
	// StorageExabyte  1024^6 byte 단위입니다.
	StorageExabyte = MustUnit[Storage]("exabyte", "EB", 1024*StoragePetabyte.Ratio())
	// StorageZettabyte  1024^7 byte 단위입니다.
	StorageZettabyte = MustUnit[Storage]("zettabyte", "ZB", 1024*StorageExabyte.Ratio())
	// StorageYottabyte  1024^8 byte 단위입니다.
	StorageYottabyte = MustUnit[Storage]("yottabyte", "YB", 1024*StorageZettabyte.Ratio())
	// StorageRegistry  1024 배율 저장 용량 registry입니다.
	StorageRegistry = MustRegistry(
		StorageByte, StorageKilobyte, StorageMegabyte, StorageGigabyte, StorageTerabyte,
		StoragePetabyte, StorageExabyte, StorageZettabyte, StorageYottabyte,
	)
)

var (
	// BinaryByte  byte 기준 단위입니다.
	BinaryByte = MustUnit[BinarySize]("byte", "B", 1)
	// BinaryKilobyte  1000 byte 단위입니다.
	BinaryKilobyte = MustUnit[BinarySize]("kilobyte", "kB", 1000)
	// BinaryMegabyte  1000^2 byte 단위입니다.
	BinaryMegabyte = MustUnit[BinarySize]("megabyte", "MB", 1_000_000)
	// BinaryGigabyte  1000^3 byte 단위입니다.
	BinaryGigabyte = MustUnit[BinarySize]("gigabyte", "GB", 1_000_000_000)
	// BinaryTerabyte  1000^4 byte 단위입니다.
	BinaryTerabyte = MustUnit[BinarySize]("terabyte", "TB", 1_000_000_000_000)
	// BinaryPetabyte  1000^5 byte 단위입니다.
	BinaryPetabyte = MustUnit[BinarySize]("petabyte", "PB", 1_000_000_000_000_000)
	// BinaryKibibyte  1024 byte 단위입니다.
	BinaryKibibyte = MustUnit[BinarySize]("kibibyte", "KiB", 1024)
	// BinaryMebibyte  1024^2 byte 단위입니다.
	BinaryMebibyte = MustUnit[BinarySize]("mebibyte", "MiB", 1_048_576)
	// BinaryGibibyte  1024^3 byte 단위입니다.
	BinaryGibibyte = MustUnit[BinarySize]("gibibyte", "GiB", 1_073_741_824)
	// BinaryTebibyte  1024^4 byte 단위입니다.
	BinaryTebibyte = MustUnit[BinarySize]("tebibyte", "TiB", 1_099_511_627_776)
	// BinaryPebibyte  1024^5 byte 단위입니다.
	BinaryPebibyte = MustUnit[BinarySize]("pebibyte", "PiB", 1_125_899_906_842_624)
	// BinaryBit  bit 단위입니다.
	BinaryBit = MustUnit[BinarySize]("bit", "bit", 1.0/8.0)
	// BinaryKilobit  1000 bit 단위입니다.
	BinaryKilobit = MustUnit[BinarySize]("kilobit", "kbit", 1000.0/8.0)
	// BinaryMegabit  1000^2 bit 단위입니다.
	BinaryMegabit = MustUnit[BinarySize]("megabit", "Mbit", 1_000_000.0/8.0)
	// BinaryGigabit  1000^3 bit 단위입니다.
	BinaryGigabit = MustUnit[BinarySize]("gigabit", "Gbit", 1_000_000_000.0/8.0)
	// BinaryTerabit  1000^4 bit 단위입니다.
	BinaryTerabit = MustUnit[BinarySize]("terabit", "Tbit", 1_000_000_000_000.0/8.0)
	// BinaryPetabit  1000^5 bit 단위입니다.
	BinaryPetabit = MustUnit[BinarySize]("petabit", "Pbit", 1_000_000_000_000_000.0/8.0)
	// BinarySizeRegistry  binary size registry입니다.
	BinarySizeRegistry = MustRegistry(
		BinaryByte, BinaryKilobyte, BinaryMegabyte, BinaryGigabyte, BinaryTerabyte, BinaryPetabyte,
		BinaryKibibyte, BinaryMebibyte, BinaryGibibyte, BinaryTebibyte, BinaryPebibyte,
		BinaryBit, BinaryKilobit, BinaryMegabit, BinaryGigabit, BinaryTerabit, BinaryPetabit,
	)
)

var (
	// FrequencyHertz  hertz 기준 단위입니다.
	FrequencyHertz = MustUnit[Frequency]("hertz", "Hz", 1)
	// FrequencyKilohertz  kilohertz 단위입니다.
	FrequencyKilohertz = MustUnit[Frequency]("kilohertz", "kHz", 1e3)
	// FrequencyMegahertz  megahertz 단위입니다.
	FrequencyMegahertz = MustUnit[Frequency]("megahertz", "MHz", 1e6)
	// FrequencyGigahertz  gigahertz 단위입니다.
	FrequencyGigahertz = MustUnit[Frequency]("gigahertz", "GHz", 1e9)
	// FrequencyRegistry  주파수 단위 registry입니다.
	FrequencyRegistry = MustRegistry(FrequencyHertz, FrequencyKilohertz, FrequencyMegahertz, FrequencyGigahertz)
)

var (
	// EnergyJoule  joule 기준 단위입니다.
	EnergyJoule = MustUnit[Energy]("joule", "J", 1)
	// EnergyKilojoule  kilojoule 단위입니다.
	EnergyKilojoule = MustUnit[Energy]("kilojoule", "kJ", 1e3)
	// EnergyMegajoule  megajoule 단위입니다.
	EnergyMegajoule = MustUnit[Energy]("megajoule", "MJ", 1e6)
	// EnergyWattHour  watt-hour 단위입니다.
	EnergyWattHour = MustUnit[Energy]("watt-hour", "Wh", 3600)
	// EnergyKilowattHour  kilowatt-hour 단위입니다.
	EnergyKilowattHour = MustUnit[Energy]("kilowatt-hour", "kWh", 3_600_000)
	// EnergyRegistry  에너지 단위 registry입니다.
	EnergyRegistry = MustRegistry(EnergyJoule, EnergyKilojoule, EnergyMegajoule, EnergyWattHour, EnergyKilowattHour)
)

var (
	// PowerMilliwatt  milliwatt 단위입니다.
	PowerMilliwatt = MustUnit[Power]("milliwatt", "mW", 1e-3)
	// PowerWatt  watt 기준 단위입니다.
	PowerWatt = MustUnit[Power]("watt", "W", 1)
	// PowerKilowatt  kilowatt 단위입니다.
	PowerKilowatt = MustUnit[Power]("kilowatt", "kW", 1e3)
	// PowerMegawatt  megawatt 단위입니다.
	PowerMegawatt = MustUnit[Power]("megawatt", "MW", 1e6)
	// PowerGigawatt  gigawatt 단위입니다.
	PowerGigawatt = MustUnit[Power]("gigawatt", "GW", 1e9)
	// PowerRegistry  전력 단위 registry입니다.
	PowerRegistry = MustRegistry(PowerMilliwatt, PowerWatt, PowerKilowatt, PowerMegawatt, PowerGigawatt)
)

var (
	// PressurePascal  pascal 기준 단위입니다.
	PressurePascal = MustUnit[Pressure]("pascal", "Pa", 1)
	// PressureHectopascal  hectopascal 단위입니다.
	PressureHectopascal = MustUnit[Pressure]("hectopascal", "hPa", 100)
	// PressureKilopascal  kilopascal 단위입니다.
	PressureKilopascal = MustUnit[Pressure]("kilopascal", "kPa", 1000)
	// PressureMegapascal  megapascal 단위입니다.
	PressureMegapascal = MustUnit[Pressure]("megapascal", "MPa", 1_000_000)
	// PressureGigapascal  gigapascal 단위입니다.
	PressureGigapascal = MustUnit[Pressure]("gigapascal", "GPa", 1_000_000_000)
	// PressureBar  bar 단위입니다.
	PressureBar = MustUnit[Pressure]("bar", "bar", 100_000)
	// PressureDecibar  decibar 단위입니다.
	PressureDecibar = MustUnit[Pressure]("decibar", "dbar", 10_000)
	// PressureMillibar  millibar 단위입니다.
	PressureMillibar = MustUnit[Pressure]("millibar", "mbar", 100)
	// PressureAtmosphere  standard atmosphere 단위입니다.
	PressureAtmosphere = MustUnit[Pressure]("atmosphere", "atm", 101_325)
	// PressurePSI  pounds per square inch 단위입니다.
	PressurePSI = MustUnit[Pressure]("psi", "psi", 6894.757)
	// PressureTorr  torr 단위입니다.
	PressureTorr = MustUnit[Pressure]("torr", "torr", 101_325.0/760.0)
	// PressureMillimeterMercury  mmHg 단위입니다.
	PressureMillimeterMercury = MustUnit[Pressure]("millimeter mercury", "mmHg", 101_325.0/760.0)
	// PressureRegistry  압력 단위 registry입니다.
	PressureRegistry = MustRegistry(
		PressurePascal, PressureHectopascal, PressureKilopascal, PressureMegapascal, PressureGigapascal,
		PressureBar, PressureDecibar, PressureMillibar, PressureAtmosphere, PressurePSI, PressureTorr, PressureMillimeterMercury,
	)
)

var (
	// AngleRadian  radian 기준 단위입니다.
	AngleRadian = MustUnit[Angle]("radian", "rad", 1)
	// AngleDegree  degree 단위입니다.
	AngleDegree = MustUnit[Angle]("degree", "deg", math.Pi/180, WithSpaceBeforeSuffix[Angle](false))
	// AngleRegistry  각도 단위 registry입니다.
	AngleRegistry = MustRegistry(AngleRadian, AngleDegree)
)

var (
	// GraphicsPixel  pixel 기준 단위입니다.
	GraphicsPixel = MustUnit[GraphicsLength]("pixel", "px", 1)
	// GraphicsLengthRegistry  그래픽 길이 단위 registry입니다.
	GraphicsLengthRegistry = MustRegistry(GraphicsPixel)
)

var (
	// VelocityMeterPerSecond  meter/second 속도 단위입니다.
	VelocityMeterPerSecond = MustRatioUnit(LengthMeter, TimeSecond)
	// VelocityKilometerPerHour  kilometer/hour 속도 단위입니다.
	VelocityKilometerPerHour = MustRatioUnit(LengthKilometer, TimeHour)
	// VelocityRegistry  속도 단위 registry입니다.
	VelocityRegistry = MustRegistry(VelocityMeterPerSecond, VelocityKilometerPerHour)
	// AccelerationMeterPerSecondSquared  meter/second^2 가속도 단위입니다.
	AccelerationMeterPerSecondSquared = MustUnit[Acceleration]("meter/second squared", "m/s^2", 1e-6)
	// AccelerationRegistry  가속도 단위 registry입니다.
	AccelerationRegistry = MustRegistry(AccelerationMeterPerSecondSquared)
)

// FromDuration  time.Duration을 Time 측정값으로 변환합니다.
func FromDuration(duration time.Duration) Measure[Time] {
	return Must(float64(duration)/float64(time.Millisecond), TimeMillisecond)
}

// Duration  Time 측정값을 time.Duration으로 변환합니다.
func Duration(value Measure[Time]) (time.Duration, error) {
	millis, err := value.In(TimeMillisecond)
	if err != nil {
		return 0, err
	}
	return time.Duration(millis * float64(time.Millisecond)), nil
}

// ParseLength  길이 측정값 문자열을 파싱합니다.
func ParseLength(text string) (Measure[Length], error) { return Parse(text, LengthRegistry) }

// ParseTime  시간 측정값 문자열을 파싱합니다.
func ParseTime(text string) (Measure[Time], error) { return Parse(text, TimeRegistry) }

// ParseMass  질량 측정값 문자열을 파싱합니다.
func ParseMass(text string) (Measure[Mass], error) { return Parse(text, MassRegistry) }

// ParseArea  면적 측정값 문자열을 파싱합니다.
func ParseArea(text string) (Measure[Area], error) { return Parse(text, AreaRegistry) }

// ParseVolume  부피 측정값 문자열을 파싱합니다.
func ParseVolume(text string) (Measure[Volume], error) { return Parse(text, VolumeRegistry) }

// ParseStorage  1024 배율 저장 용량 문자열을 파싱합니다.
func ParseStorage(text string) (Measure[Storage], error) { return Parse(text, StorageRegistry) }

// ParseBinarySize  binary size 측정값 문자열을 파싱합니다.
func ParseBinarySize(text string) (Measure[BinarySize], error) {
	return Parse(text, BinarySizeRegistry)
}

// ParseFrequency  주파수 측정값 문자열을 파싱합니다.
func ParseFrequency(text string) (Measure[Frequency], error) { return Parse(text, FrequencyRegistry) }

// ParseEnergy  에너지 측정값 문자열을 파싱합니다.
func ParseEnergy(text string) (Measure[Energy], error) { return Parse(text, EnergyRegistry) }

// ParsePower  전력 측정값 문자열을 파싱합니다.
func ParsePower(text string) (Measure[Power], error) { return Parse(text, PowerRegistry) }

// ParsePressure  압력 측정값 문자열을 파싱합니다.
func ParsePressure(text string) (Measure[Pressure], error) { return Parse(text, PressureRegistry) }

// ParseAngle  각도 측정값 문자열을 파싱합니다.
func ParseAngle(text string) (Measure[Angle], error) { return Parse(text, AngleRegistry) }

// ParseGraphicsLength  그래픽 길이 측정값 문자열을 파싱합니다.
func ParseGraphicsLength(text string) (Measure[GraphicsLength], error) {
	return Parse(text, GraphicsLengthRegistry)
}

// ParseVelocity  속도 측정값 문자열을 파싱합니다.
func ParseVelocity(text string) (Measure[Velocity], error) { return Parse(text, VelocityRegistry) }

// ParseAcceleration  가속도 측정값 문자열을 파싱합니다.
func ParseAcceleration(text string) (Measure[Acceleration], error) {
	return Parse(text, AccelerationRegistry)
}

// HumanLength  길이 측정값을 practical 단위로 포맷합니다.
func HumanLength(value Measure[Length]) (string, error) {
	return value.Human(LengthMillimeter, LengthCentimeter, LengthMeter, LengthKilometer)
}

// HumanTime  시간 측정값을 practical 단위로 포맷합니다.
func HumanTime(value Measure[Time]) (string, error) {
	return value.Human(TimeMillisecond, TimeSecond, TimeMinute, TimeHour)
}

// HumanMass  질량 측정값을 practical 단위로 포맷합니다.
func HumanMass(value Measure[Mass]) (string, error) {
	return value.Human(MassGram, MassKilogram, MassTon)
}

// HumanArea  면적 측정값을 practical 단위로 포맷합니다.
func HumanArea(value Measure[Area]) (string, error) {
	return value.Human(AreaSquareMillimeter, AreaSquareCentimeter, AreaSquareMeter, AreaSquareKilometer)
}

// HumanVolume  부피 측정값을 practical 단위로 포맷합니다.
func HumanVolume(value Measure[Volume]) (string, error) {
	return value.Human(VolumeCubicMillimeter, VolumeCubicCentimeter, VolumeMilliliter, VolumeLiter, VolumeCubicMeter)
}

// HumanStorage  저장 용량 측정값을 practical 단위로 포맷합니다.
func HumanStorage(value Measure[Storage]) (string, error) {
	return value.Human(StorageByte, StorageKilobyte, StorageMegabyte, StorageGigabyte, StorageTerabyte, StoragePetabyte)
}

// HumanBinarySize  binary size 측정값을 practical 단위로 포맷합니다.
func HumanBinarySize(value Measure[BinarySize]) (string, error) {
	return value.Human(BinaryBit, BinaryByte, BinaryKilobyte, BinaryMegabyte, BinaryGigabyte, BinaryTerabyte, BinaryPetabyte)
}

// HumanFrequency  주파수 측정값을 practical 단위로 포맷합니다.
func HumanFrequency(value Measure[Frequency]) (string, error) {
	return value.Human(FrequencyHertz, FrequencyKilohertz, FrequencyMegahertz, FrequencyGigahertz)
}

// HumanEnergy  에너지 측정값을 practical 단위로 포맷합니다.
func HumanEnergy(value Measure[Energy]) (string, error) {
	return value.Human(EnergyJoule, EnergyKilojoule, EnergyMegajoule, EnergyWattHour, EnergyKilowattHour)
}

// HumanPower  전력 측정값을 practical 단위로 포맷합니다.
func HumanPower(value Measure[Power]) (string, error) {
	return value.Human(PowerMilliwatt, PowerWatt, PowerKilowatt, PowerMegawatt, PowerGigawatt)
}

// HumanPressure  압력 측정값을 practical 단위로 포맷합니다.
func HumanPressure(value Measure[Pressure]) (string, error) {
	return value.Human(PressurePascal, PressureHectopascal, PressureKilopascal, PressureMegapascal, PressureGigapascal, PressureBar, PressureAtmosphere, PressurePSI)
}

// HumanAngle  각도 측정값을 0..360도 범위로 정규화해 포맷합니다.
func HumanAngle(value Measure[Angle]) (string, error) {
	degrees, err := value.In(AngleDegree)
	if err != nil {
		return "", err
	}
	normalized := math.Mod(math.Mod(degrees, 360)+360, 360)
	return formatValue(normalized, AngleDegree), nil
}

// Sin  Angle 측정값의 sine을 반환합니다.
func Sin(value Measure[Angle]) (float64, error) {
	radians, err := value.In(AngleRadian)
	if err != nil {
		return 0, err
	}
	return math.Sin(radians), nil
}

// Cos  Angle 측정값의 cosine을 반환합니다.
func Cos(value Measure[Angle]) (float64, error) {
	radians, err := value.In(AngleRadian)
	if err != nil {
		return 0, err
	}
	return math.Cos(radians), nil
}

// Tan  Angle 측정값의 tangent를 반환합니다.
func Tan(value Measure[Angle]) (float64, error) {
	radians, err := value.In(AngleRadian)
	if err != nil {
		return 0, err
	}
	return math.Tan(radians), nil
}

// ASin  sine 역함수 결과를 Angle 측정값으로 반환합니다.
func ASin(value float64) (Measure[Angle], error) {
	return New(math.Asin(value), AngleRadian)
}

// MustASin  sine 역함수 실패 시 panic을 발생시킵니다.
func MustASin(value float64) Measure[Angle] {
	angle, err := ASin(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ACos  cosine 역함수 결과를 Angle 측정값으로 반환합니다.
func ACos(value float64) (Measure[Angle], error) {
	return New(math.Acos(value), AngleRadian)
}

// MustACos  cosine 역함수 실패 시 panic을 발생시킵니다.
func MustACos(value float64) Measure[Angle] {
	angle, err := ACos(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan  tangent 역함수 결과를 Angle 측정값으로 반환합니다.
func ATan(value float64) (Measure[Angle], error) {
	return New(math.Atan(value), AngleRadian)
}

// MustATan  tangent 역함수 실패 시 panic을 발생시킵니다.
func MustATan(value float64) Measure[Angle] {
	angle, err := ATan(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan2  atan2 결과를 Angle 측정값으로 반환합니다.
func ATan2(y, x float64) (Measure[Angle], error) {
	return New(math.Atan2(y, x), AngleRadian)
}

// MustATan2  atan2 실패 시 panic을 발생시킵니다.
func MustATan2(y, x float64) Measure[Angle] {
	angle, err := ATan2(y, x)
	if err != nil {
		panic(err)
	}
	return angle
}
