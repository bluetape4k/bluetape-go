package measure

import (
	"fmt"
	"math"
	"time"
)

var (
	// lengthMillimeter  millimeter 단위입니다.
	lengthMillimeter = MustUnit[Length]("millimeter", "mm", 0.001)
	// lengthCentimeter  centimeter 단위입니다.
	lengthCentimeter = MustUnit[Length]("centimeter", "cm", 0.01)
	// lengthMeter  meter 기준 단위입니다.
	lengthMeter = MustUnit[Length]("meter", "m", 1)
	// lengthKilometer  kilometer 단위입니다.
	lengthKilometer = MustUnit[Length]("kilometer", "km", 1000)
	// lengthInch  inch 단위입니다.
	lengthInch = MustUnit[Length]("inch", "in", 0.0254)
	// lengthFoot  foot 단위입니다.
	lengthFoot = MustUnit[Length]("foot", "ft", 0.3048)
	// lengthMile  mile 단위입니다.
	lengthMile = MustUnit[Length]("mile", "mi", 1609.344)
	// lengthRegistry  길이 단위 registry입니다.
	lengthRegistry = MustRegistry(lengthMillimeter, lengthCentimeter, lengthMeter, lengthKilometer, lengthInch, lengthFoot, lengthMile)
)

var (
	// timeMillisecond  millisecond 기준 단위입니다.
	timeMillisecond = MustUnit[Time]("millisecond", "ms", 1)
	// timeSecond  second 단위입니다.
	timeSecond = MustUnit[Time]("second", "s", 1000)
	// timeMinute  minute 단위입니다.
	timeMinute = MustUnit[Time]("minute", "min", 60_000)
	// timeHour  hour 단위입니다.
	timeHour = MustUnit[Time]("hour", "hr", 3_600_000)
	// timeRegistry  시간 단위 registry입니다.
	timeRegistry = MustRegistry(timeMillisecond, timeSecond, timeMinute, timeHour)
)

var (
	// massGram  gram 기준 단위입니다.
	massGram = MustUnit[Mass]("gram", "g", 1)
	// massKilogram  kilogram 단위입니다.
	massKilogram = MustUnit[Mass]("kilogram", "kg", 1000)
	// massTon  metric ton 단위입니다.
	massTon = MustUnit[Mass]("ton", "ton", 1_000_000)
	// massRegistry  질량 단위 registry입니다.
	massRegistry = MustRegistry(massGram, massKilogram, massTon)
)

var (
	// areaSquareMillimeter  square millimeter 단위입니다.
	areaSquareMillimeter = MustUnit[Area]("square millimeter", "mm^2", 1e-6)
	// areaSquareCentimeter  square centimeter 단위입니다.
	areaSquareCentimeter = MustUnit[Area]("square centimeter", "cm^2", 1e-4)
	// areaSquareMeter  square meter 기준 단위입니다.
	areaSquareMeter = MustUnit[Area]("square meter", "m^2", 1)
	// areaSquareKilometer  square kilometer 단위입니다.
	areaSquareKilometer = MustUnit[Area]("square kilometer", "km^2", 1e6)
	// areaRegistry  면적 단위 registry입니다.
	areaRegistry = MustRegistry(areaSquareMillimeter, areaSquareCentimeter, areaSquareMeter, areaSquareKilometer)
)

var (
	// volumeCubicMillimeter  cubic millimeter 단위입니다.
	volumeCubicMillimeter = MustUnit[Volume]("cubic millimeter", "mm^3", 1e-9)
	// volumeCubicCentimeter  cubic centimeter 단위입니다.
	volumeCubicCentimeter = MustUnit[Volume]("cubic centimeter", "cm^3", 1e-6)
	// volumeMilliliter  milliliter 단위입니다.
	volumeMilliliter = MustUnit[Volume]("milliliter", "mL", 1e-6)
	// volumeLiter  liter 단위입니다.
	volumeLiter = MustUnit[Volume]("liter", "L", 1e-3)
	// volumeCubicMeter  cubic meter 기준 단위입니다.
	volumeCubicMeter = MustUnit[Volume]("cubic meter", "m^3", 1)
	// volumeRegistry  부피 단위 registry입니다.
	volumeRegistry = MustRegistry(volumeCubicMillimeter, volumeCubicCentimeter, volumeMilliliter, volumeLiter, volumeCubicMeter)
)

var (
	// storageByte byte 기준 단위입니다.
	storageByte = MustUnit[Storage]("byte", "B", 1)
	// storageKilobyte 1024 byte 단위입니다.
	storageKilobyte = MustUnit[Storage]("kilobyte", "KB", 1024)
	// storageMegabyte 1024^2 byte 단위입니다.
	storageMegabyte = MustUnit[Storage]("megabyte", "MB", 1024*1024)
	// storageGigabyte 1024^3 byte 단위입니다.
	storageGigabyte = MustUnit[Storage]("gigabyte", "GB", 1024*1024*1024)
	// storageTerabyte  1024^4 byte 단위입니다.
	storageTerabyte = MustUnit[Storage]("terabyte", "TB", 1024*storageGigabyte.Ratio())
	// storagePetabyte  1024^5 byte 단위입니다.
	storagePetabyte = MustUnit[Storage]("petabyte", "PB", 1024*storageTerabyte.Ratio())
	// storageExabyte  1024^6 byte 단위입니다.
	storageExabyte = MustUnit[Storage]("exabyte", "EB", 1024*storagePetabyte.Ratio())
	// storageZettabyte  1024^7 byte 단위입니다.
	storageZettabyte = MustUnit[Storage]("zettabyte", "ZB", 1024*storageExabyte.Ratio())
	// storageYottabyte  1024^8 byte 단위입니다.
	storageYottabyte = MustUnit[Storage]("yottabyte", "YB", 1024*storageZettabyte.Ratio())
	// storageRegistry  1024 배율 저장 용량 registry입니다.
	storageRegistry = MustRegistry(
		storageByte, storageKilobyte, storageMegabyte, storageGigabyte, storageTerabyte,
		storagePetabyte, storageExabyte, storageZettabyte, storageYottabyte,
	)
)

var (
	// binaryByte  byte 기준 단위입니다.
	binaryByte = MustUnit[BinarySize]("byte", "B", 1)
	// binaryKilobyte  1000 byte 단위입니다.
	binaryKilobyte = MustUnit[BinarySize]("kilobyte", "kB", 1000)
	// binaryMegabyte  1000^2 byte 단위입니다.
	binaryMegabyte = MustUnit[BinarySize]("megabyte", "MB", 1_000_000)
	// binaryGigabyte  1000^3 byte 단위입니다.
	binaryGigabyte = MustUnit[BinarySize]("gigabyte", "GB", 1_000_000_000)
	// binaryTerabyte  1000^4 byte 단위입니다.
	binaryTerabyte = MustUnit[BinarySize]("terabyte", "TB", 1_000_000_000_000)
	// binaryPetabyte  1000^5 byte 단위입니다.
	binaryPetabyte = MustUnit[BinarySize]("petabyte", "PB", 1_000_000_000_000_000)
	// binaryKibibyte  1024 byte 단위입니다.
	binaryKibibyte = MustUnit[BinarySize]("kibibyte", "KiB", 1024)
	// binaryMebibyte  1024^2 byte 단위입니다.
	binaryMebibyte = MustUnit[BinarySize]("mebibyte", "MiB", 1_048_576)
	// binaryGibibyte  1024^3 byte 단위입니다.
	binaryGibibyte = MustUnit[BinarySize]("gibibyte", "GiB", 1_073_741_824)
	// binaryTebibyte  1024^4 byte 단위입니다.
	binaryTebibyte = MustUnit[BinarySize]("tebibyte", "TiB", 1_099_511_627_776)
	// binaryPebibyte  1024^5 byte 단위입니다.
	binaryPebibyte = MustUnit[BinarySize]("pebibyte", "PiB", 1_125_899_906_842_624)
	// binaryBit  bit 단위입니다.
	binaryBit = MustUnit[BinarySize]("bit", "bit", 1.0/8.0)
	// binaryKilobit  1000 bit 단위입니다.
	binaryKilobit = MustUnit[BinarySize]("kilobit", "kbit", 1000.0/8.0)
	// binaryMegabit  1000^2 bit 단위입니다.
	binaryMegabit = MustUnit[BinarySize]("megabit", "Mbit", 1_000_000.0/8.0)
	// binaryGigabit  1000^3 bit 단위입니다.
	binaryGigabit = MustUnit[BinarySize]("gigabit", "Gbit", 1_000_000_000.0/8.0)
	// binaryTerabit  1000^4 bit 단위입니다.
	binaryTerabit = MustUnit[BinarySize]("terabit", "Tbit", 1_000_000_000_000.0/8.0)
	// binaryPetabit  1000^5 bit 단위입니다.
	binaryPetabit = MustUnit[BinarySize]("petabit", "Pbit", 1_000_000_000_000_000.0/8.0)
	// binarySizeRegistry  binary size registry입니다.
	binarySizeRegistry = MustRegistry(
		binaryByte, binaryKilobyte, binaryMegabyte, binaryGigabyte, binaryTerabyte, binaryPetabyte,
		binaryKibibyte, binaryMebibyte, binaryGibibyte, binaryTebibyte, binaryPebibyte,
		binaryBit, binaryKilobit, binaryMegabit, binaryGigabit, binaryTerabit, binaryPetabit,
	)
)

var (
	// frequencyHertz  hertz 기준 단위입니다.
	frequencyHertz = MustUnit[Frequency]("hertz", "Hz", 1)
	// frequencyKilohertz  kilohertz 단위입니다.
	frequencyKilohertz = MustUnit[Frequency]("kilohertz", "kHz", 1e3)
	// frequencyMegahertz  megahertz 단위입니다.
	frequencyMegahertz = MustUnit[Frequency]("megahertz", "MHz", 1e6)
	// frequencyGigahertz  gigahertz 단위입니다.
	frequencyGigahertz = MustUnit[Frequency]("gigahertz", "GHz", 1e9)
	// frequencyRegistry  주파수 단위 registry입니다.
	frequencyRegistry = MustRegistry(frequencyHertz, frequencyKilohertz, frequencyMegahertz, frequencyGigahertz)
)

var (
	// energyJoule  joule 기준 단위입니다.
	energyJoule = MustUnit[Energy]("joule", "J", 1)
	// energyKilojoule  kilojoule 단위입니다.
	energyKilojoule = MustUnit[Energy]("kilojoule", "kJ", 1e3)
	// energyMegajoule  megajoule 단위입니다.
	energyMegajoule = MustUnit[Energy]("megajoule", "MJ", 1e6)
	// energyWattHour  watt-hour 단위입니다.
	energyWattHour = MustUnit[Energy]("watt-hour", "Wh", 3600)
	// energyKilowattHour  kilowatt-hour 단위입니다.
	energyKilowattHour = MustUnit[Energy]("kilowatt-hour", "kWh", 3_600_000)
	// energyRegistry  에너지 단위 registry입니다.
	energyRegistry = MustRegistry(energyJoule, energyKilojoule, energyMegajoule, energyWattHour, energyKilowattHour)
)

var (
	// powerMilliwatt  milliwatt 단위입니다.
	powerMilliwatt = MustUnit[Power]("milliwatt", "mW", 1e-3)
	// powerWatt  watt 기준 단위입니다.
	powerWatt = MustUnit[Power]("watt", "W", 1)
	// powerKilowatt  kilowatt 단위입니다.
	powerKilowatt = MustUnit[Power]("kilowatt", "kW", 1e3)
	// powerMegawatt  megawatt 단위입니다.
	powerMegawatt = MustUnit[Power]("megawatt", "MW", 1e6)
	// powerGigawatt  gigawatt 단위입니다.
	powerGigawatt = MustUnit[Power]("gigawatt", "GW", 1e9)
	// powerRegistry  전력 단위 registry입니다.
	powerRegistry = MustRegistry(powerMilliwatt, powerWatt, powerKilowatt, powerMegawatt, powerGigawatt)
)

var (
	// pressurePascal  pascal 기준 단위입니다.
	pressurePascal = MustUnit[Pressure]("pascal", "Pa", 1)
	// pressureHectopascal  hectopascal 단위입니다.
	pressureHectopascal = MustUnit[Pressure]("hectopascal", "hPa", 100)
	// pressureKilopascal  kilopascal 단위입니다.
	pressureKilopascal = MustUnit[Pressure]("kilopascal", "kPa", 1000)
	// pressureMegapascal  megapascal 단위입니다.
	pressureMegapascal = MustUnit[Pressure]("megapascal", "MPa", 1_000_000)
	// pressureGigapascal  gigapascal 단위입니다.
	pressureGigapascal = MustUnit[Pressure]("gigapascal", "GPa", 1_000_000_000)
	// pressureBar  bar 단위입니다.
	pressureBar = MustUnit[Pressure]("bar", "bar", 100_000)
	// pressureDecibar  decibar 단위입니다.
	pressureDecibar = MustUnit[Pressure]("decibar", "dbar", 10_000)
	// pressureMillibar  millibar 단위입니다.
	pressureMillibar = MustUnit[Pressure]("millibar", "mbar", 100)
	// pressureAtmosphere  standard atmosphere 단위입니다.
	pressureAtmosphere = MustUnit[Pressure]("atmosphere", "atm", 101_325)
	// pressurePSI  pounds per square inch 단위입니다.
	pressurePSI = MustUnit[Pressure]("psi", "psi", 6894.757)
	// pressureTorr  torr 단위입니다.
	pressureTorr = MustUnit[Pressure]("torr", "torr", 101_325.0/760.0)
	// pressureMillimeterMercury  mmHg 단위입니다.
	pressureMillimeterMercury = MustUnit[Pressure]("millimeter mercury", "mmHg", 101_325.0/760.0)
	// pressureRegistry  압력 단위 registry입니다.
	pressureRegistry = MustRegistry(
		pressurePascal, pressureHectopascal, pressureKilopascal, pressureMegapascal, pressureGigapascal,
		pressureBar, pressureDecibar, pressureMillibar, pressureAtmosphere, pressurePSI, pressureTorr, pressureMillimeterMercury,
	)
)

var (
	// angleRadian  radian 기준 단위입니다.
	angleRadian = MustUnit[Angle]("radian", "rad", 1)
	// angleDegree  degree 단위입니다.
	angleDegree = MustUnit[Angle]("degree", "deg", math.Pi/180, WithSpaceBeforeSuffix[Angle](false))
	// angleRegistry  각도 단위 registry입니다.
	angleRegistry = MustRegistry(angleRadian, angleDegree)
)

var (
	// graphicsPixel  pixel 기준 단위입니다.
	graphicsPixel = MustUnit[GraphicsLength]("pixel", "px", 1)
	// graphicsLengthRegistry  그래픽 길이 단위 registry입니다.
	graphicsLengthRegistry = MustRegistry(graphicsPixel)
)

var (
	// velocityMeterPerSecond  meter/second 속도 단위입니다.
	velocityMeterPerSecond = MustRatioUnit(lengthMeter, timeSecond)
	// velocityKilometerPerHour  kilometer/hour 속도 단위입니다.
	velocityKilometerPerHour = MustRatioUnit(lengthKilometer, timeHour)
	// velocityRegistry  속도 단위 registry입니다.
	velocityRegistry = MustRegistry(velocityMeterPerSecond, velocityKilometerPerHour)
	// accelerationMeterPerSecondSquared  meter/second^2 가속도 단위입니다.
	accelerationMeterPerSecondSquared = MustUnit[Acceleration]("meter/second squared", "m/s^2", 1e-6)
	// accelerationRegistry  가속도 단위 registry입니다.
	accelerationRegistry = MustRegistry(accelerationMeterPerSecondSquared)
)

// FromDuration time.Duration 값을 측정값으로 변환한다.
//
// 매개변수:
//   - duration: FromDuration에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func FromDuration(duration time.Duration) Measure[Time] {
	return Must(float64(duration)/float64(time.Millisecond), timeMillisecond)
}

// LengthMillimeter 해당 측정 단위를 반환한다.
func LengthMillimeter() Unit[Length] { return lengthMillimeter }

// LengthCentimeter 해당 측정 단위를 반환한다.
func LengthCentimeter() Unit[Length] { return lengthCentimeter }

// LengthMeter 해당 측정 단위를 반환한다.
func LengthMeter() Unit[Length] { return lengthMeter }

// LengthKilometer 해당 측정 단위를 반환한다.
func LengthKilometer() Unit[Length] { return lengthKilometer }

// LengthInch 해당 측정 단위를 반환한다.
func LengthInch() Unit[Length] { return lengthInch }

// LengthFoot 해당 측정 단위를 반환한다.
func LengthFoot() Unit[Length] { return lengthFoot }

// LengthMile 해당 측정 단위를 반환한다.
func LengthMile() Unit[Length] { return lengthMile }

// LengthRegistry 해당 차원의 단위 registry를 반환한다.
func LengthRegistry() Registry[Length] { return lengthRegistry }

// TimeMillisecond 해당 측정 단위를 반환한다.
func TimeMillisecond() Unit[Time] { return timeMillisecond }

// TimeSecond 해당 측정 단위를 반환한다.
func TimeSecond() Unit[Time] { return timeSecond }

// TimeMinute 해당 측정 단위를 반환한다.
func TimeMinute() Unit[Time] { return timeMinute }

// TimeHour 해당 측정 단위를 반환한다.
func TimeHour() Unit[Time] { return timeHour }

// TimeRegistry 해당 차원의 단위 registry를 반환한다.
func TimeRegistry() Registry[Time] { return timeRegistry }

// MassGram 해당 측정 단위를 반환한다.
func MassGram() Unit[Mass] { return massGram }

// MassKilogram 해당 측정 단위를 반환한다.
func MassKilogram() Unit[Mass] { return massKilogram }

// MassTon 해당 측정 단위를 반환한다.
func MassTon() Unit[Mass] { return massTon }

// MassRegistry 해당 차원의 단위 registry를 반환한다.
func MassRegistry() Registry[Mass] { return massRegistry }

// AreaSquareMillimeter 해당 측정 단위를 반환한다.
func AreaSquareMillimeter() Unit[Area] { return areaSquareMillimeter }

// AreaSquareCentimeter 해당 측정 단위를 반환한다.
func AreaSquareCentimeter() Unit[Area] { return areaSquareCentimeter }

// AreaSquareMeter 해당 측정 단위를 반환한다.
func AreaSquareMeter() Unit[Area] { return areaSquareMeter }

// AreaSquareKilometer 해당 측정 단위를 반환한다.
func AreaSquareKilometer() Unit[Area] { return areaSquareKilometer }

// AreaRegistry 해당 차원의 단위 registry를 반환한다.
func AreaRegistry() Registry[Area] { return areaRegistry }

// VolumeCubicMillimeter 해당 측정 단위를 반환한다.
func VolumeCubicMillimeter() Unit[Volume] { return volumeCubicMillimeter }

// VolumeCubicCentimeter 해당 측정 단위를 반환한다.
func VolumeCubicCentimeter() Unit[Volume] { return volumeCubicCentimeter }

// VolumeMilliliter 해당 측정 단위를 반환한다.
func VolumeMilliliter() Unit[Volume] { return volumeMilliliter }

// VolumeLiter 해당 측정 단위를 반환한다.
func VolumeLiter() Unit[Volume] { return volumeLiter }

// VolumeCubicMeter 해당 측정 단위를 반환한다.
func VolumeCubicMeter() Unit[Volume] { return volumeCubicMeter }

// VolumeRegistry 해당 차원의 단위 registry를 반환한다.
func VolumeRegistry() Registry[Volume] { return volumeRegistry }

// StorageByte 해당 측정 단위를 반환한다.
func StorageByte() Unit[Storage] { return storageByte }

// StorageKilobyte 해당 측정 단위를 반환한다.
func StorageKilobyte() Unit[Storage] { return storageKilobyte }

// StorageMegabyte 해당 측정 단위를 반환한다.
func StorageMegabyte() Unit[Storage] { return storageMegabyte }

// StorageGigabyte 해당 측정 단위를 반환한다.
func StorageGigabyte() Unit[Storage] { return storageGigabyte }

// StorageTerabyte 해당 측정 단위를 반환한다.
func StorageTerabyte() Unit[Storage] { return storageTerabyte }

// StoragePetabyte 해당 측정 단위를 반환한다.
func StoragePetabyte() Unit[Storage] { return storagePetabyte }

// StorageExabyte 해당 측정 단위를 반환한다.
func StorageExabyte() Unit[Storage] { return storageExabyte }

// StorageZettabyte 해당 측정 단위를 반환한다.
func StorageZettabyte() Unit[Storage] { return storageZettabyte }

// StorageYottabyte 해당 측정 단위를 반환한다.
func StorageYottabyte() Unit[Storage] { return storageYottabyte }

// StorageRegistry 해당 차원의 단위 registry를 반환한다.
func StorageRegistry() Registry[Storage] { return storageRegistry }

// BinaryByte 해당 측정 단위를 반환한다.
func BinaryByte() Unit[BinarySize] { return binaryByte }

// BinaryKilobyte 해당 측정 단위를 반환한다.
func BinaryKilobyte() Unit[BinarySize] { return binaryKilobyte }

// BinaryMegabyte 해당 측정 단위를 반환한다.
func BinaryMegabyte() Unit[BinarySize] { return binaryMegabyte }

// BinaryGigabyte 해당 측정 단위를 반환한다.
func BinaryGigabyte() Unit[BinarySize] { return binaryGigabyte }

// BinaryTerabyte 해당 측정 단위를 반환한다.
func BinaryTerabyte() Unit[BinarySize] { return binaryTerabyte }

// BinaryPetabyte 해당 측정 단위를 반환한다.
func BinaryPetabyte() Unit[BinarySize] { return binaryPetabyte }

// BinaryKibibyte 해당 측정 단위를 반환한다.
func BinaryKibibyte() Unit[BinarySize] { return binaryKibibyte }

// BinaryMebibyte 해당 측정 단위를 반환한다.
func BinaryMebibyte() Unit[BinarySize] { return binaryMebibyte }

// BinaryGibibyte 해당 측정 단위를 반환한다.
func BinaryGibibyte() Unit[BinarySize] { return binaryGibibyte }

// BinaryTebibyte 해당 측정 단위를 반환한다.
func BinaryTebibyte() Unit[BinarySize] { return binaryTebibyte }

// BinaryPebibyte 해당 측정 단위를 반환한다.
func BinaryPebibyte() Unit[BinarySize] { return binaryPebibyte }

// BinaryBit 해당 측정 단위를 반환한다.
func BinaryBit() Unit[BinarySize] { return binaryBit }

// BinaryKilobit 해당 측정 단위를 반환한다.
func BinaryKilobit() Unit[BinarySize] { return binaryKilobit }

// BinaryMegabit 해당 측정 단위를 반환한다.
func BinaryMegabit() Unit[BinarySize] { return binaryMegabit }

// BinaryGigabit 해당 측정 단위를 반환한다.
func BinaryGigabit() Unit[BinarySize] { return binaryGigabit }

// BinaryTerabit 해당 측정 단위를 반환한다.
func BinaryTerabit() Unit[BinarySize] { return binaryTerabit }

// BinaryPetabit 해당 측정 단위를 반환한다.
func BinaryPetabit() Unit[BinarySize] { return binaryPetabit }

// BinarySizeRegistry 해당 차원의 단위 registry를 반환한다.
func BinarySizeRegistry() Registry[BinarySize] { return binarySizeRegistry }

// FrequencyHertz 해당 측정 단위를 반환한다.
func FrequencyHertz() Unit[Frequency] { return frequencyHertz }

// FrequencyKilohertz 해당 측정 단위를 반환한다.
func FrequencyKilohertz() Unit[Frequency] { return frequencyKilohertz }

// FrequencyMegahertz 해당 측정 단위를 반환한다.
func FrequencyMegahertz() Unit[Frequency] { return frequencyMegahertz }

// FrequencyGigahertz 해당 측정 단위를 반환한다.
func FrequencyGigahertz() Unit[Frequency] { return frequencyGigahertz }

// FrequencyRegistry 해당 차원의 단위 registry를 반환한다.
func FrequencyRegistry() Registry[Frequency] { return frequencyRegistry }

// EnergyJoule 해당 측정 단위를 반환한다.
func EnergyJoule() Unit[Energy] { return energyJoule }

// EnergyKilojoule 해당 측정 단위를 반환한다.
func EnergyKilojoule() Unit[Energy] { return energyKilojoule }

// EnergyMegajoule 해당 측정 단위를 반환한다.
func EnergyMegajoule() Unit[Energy] { return energyMegajoule }

// EnergyWattHour 해당 측정 단위를 반환한다.
func EnergyWattHour() Unit[Energy] { return energyWattHour }

// EnergyKilowattHour 해당 측정 단위를 반환한다.
func EnergyKilowattHour() Unit[Energy] { return energyKilowattHour }

// EnergyRegistry 해당 차원의 단위 registry를 반환한다.
func EnergyRegistry() Registry[Energy] { return energyRegistry }

// PowerMilliwatt 해당 측정 단위를 반환한다.
func PowerMilliwatt() Unit[Power] { return powerMilliwatt }

// PowerWatt 해당 측정 단위를 반환한다.
func PowerWatt() Unit[Power] { return powerWatt }

// PowerKilowatt 해당 측정 단위를 반환한다.
func PowerKilowatt() Unit[Power] { return powerKilowatt }

// PowerMegawatt 해당 측정 단위를 반환한다.
func PowerMegawatt() Unit[Power] { return powerMegawatt }

// PowerGigawatt 해당 측정 단위를 반환한다.
func PowerGigawatt() Unit[Power] { return powerGigawatt }

// PowerRegistry 해당 차원의 단위 registry를 반환한다.
func PowerRegistry() Registry[Power] { return powerRegistry }

// PressurePascal 해당 측정 단위를 반환한다.
func PressurePascal() Unit[Pressure] { return pressurePascal }

// PressureHectopascal 해당 측정 단위를 반환한다.
func PressureHectopascal() Unit[Pressure] { return pressureHectopascal }

// PressureKilopascal 해당 측정 단위를 반환한다.
func PressureKilopascal() Unit[Pressure] { return pressureKilopascal }

// PressureMegapascal 해당 측정 단위를 반환한다.
func PressureMegapascal() Unit[Pressure] { return pressureMegapascal }

// PressureGigapascal 해당 측정 단위를 반환한다.
func PressureGigapascal() Unit[Pressure] { return pressureGigapascal }

// PressureBar 해당 측정 단위를 반환한다.
func PressureBar() Unit[Pressure] { return pressureBar }

// PressureDecibar 해당 측정 단위를 반환한다.
func PressureDecibar() Unit[Pressure] { return pressureDecibar }

// PressureMillibar 해당 측정 단위를 반환한다.
func PressureMillibar() Unit[Pressure] { return pressureMillibar }

// PressureAtmosphere 해당 측정 단위를 반환한다.
func PressureAtmosphere() Unit[Pressure] { return pressureAtmosphere }

// PressurePSI 해당 측정 단위를 반환한다.
func PressurePSI() Unit[Pressure] { return pressurePSI }

// PressureTorr 해당 측정 단위를 반환한다.
func PressureTorr() Unit[Pressure] { return pressureTorr }

// PressureMillimeterMercury 해당 측정 단위를 반환한다.
func PressureMillimeterMercury() Unit[Pressure] { return pressureMillimeterMercury }

// PressureRegistry 해당 차원의 단위 registry를 반환한다.
func PressureRegistry() Registry[Pressure] { return pressureRegistry }

// AngleRadian 해당 측정 단위를 반환한다.
func AngleRadian() Unit[Angle] { return angleRadian }

// AngleDegree 해당 측정 단위를 반환한다.
func AngleDegree() Unit[Angle] { return angleDegree }

// AngleRegistry 해당 차원의 단위 registry를 반환한다.
func AngleRegistry() Registry[Angle] { return angleRegistry }

// GraphicsPixel 해당 측정 단위를 반환한다.
func GraphicsPixel() Unit[GraphicsLength] { return graphicsPixel }

// GraphicsLengthRegistry 해당 차원의 단위 registry를 반환한다.
func GraphicsLengthRegistry() Registry[GraphicsLength] { return graphicsLengthRegistry }

// VelocityMeterPerSecond 해당 측정 단위를 반환한다.
func VelocityMeterPerSecond() Unit[Velocity] { return velocityMeterPerSecond }

// VelocityKilometerPerHour 해당 측정 단위를 반환한다.
func VelocityKilometerPerHour() Unit[Velocity] { return velocityKilometerPerHour }

// VelocityRegistry 해당 차원의 단위 registry를 반환한다.
func VelocityRegistry() Registry[Velocity] { return velocityRegistry }

// AccelerationMeterPerSecondSquared 해당 측정 단위를 반환한다.
func AccelerationMeterPerSecondSquared() Unit[Acceleration] {
	return accelerationMeterPerSecondSquared
}

// AccelerationRegistry 해당 차원의 단위 registry를 반환한다.
func AccelerationRegistry() Registry[Acceleration] { return accelerationRegistry }

// Duration 측정값을 time.Duration으로 변환한다.
//
// 매개변수:
//   - value: Duration에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Duration(value Measure[Time]) (time.Duration, error) {
	millis, err := value.In(timeMillisecond)
	if err != nil {
		return 0, err
	}
	nanos := millis * float64(time.Millisecond)
	if !finite(nanos) || nanos > float64(math.MaxInt64) || nanos < float64(math.MinInt64) {
		return 0, fmt.Errorf("%w: duration overflow", ErrInvalidMeasure)
	}
	return time.Duration(nanos), nil
}

// ParseLength 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseLength가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseLength(text string) (Measure[Length], error) { return Parse(text, lengthRegistry) }

// ParseTime 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseTime가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseTime(text string) (Measure[Time], error) { return Parse(text, timeRegistry) }

// ParseMass 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseMass가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseMass(text string) (Measure[Mass], error) { return Parse(text, massRegistry) }

// ParseArea 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseArea가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseArea(text string) (Measure[Area], error) { return Parse(text, areaRegistry) }

// ParseVolume 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseVolume가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseVolume(text string) (Measure[Volume], error) { return Parse(text, volumeRegistry) }

// ParseStorage 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseStorage가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseStorage(text string) (Measure[Storage], error) { return Parse(text, storageRegistry) }

// ParseBinarySize 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseBinarySize가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseBinarySize(text string) (Measure[BinarySize], error) {
	return Parse(text, binarySizeRegistry)
}

// ParseFrequency 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseFrequency가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseFrequency(text string) (Measure[Frequency], error) { return Parse(text, frequencyRegistry) }

// ParseEnergy 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseEnergy가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseEnergy(text string) (Measure[Energy], error) { return Parse(text, energyRegistry) }

// ParsePower 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParsePower가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParsePower(text string) (Measure[Power], error) { return Parse(text, powerRegistry) }

// ParsePressure 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParsePressure가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParsePressure(text string) (Measure[Pressure], error) { return Parse(text, pressureRegistry) }

// ParseAngle 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseAngle가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseAngle(text string) (Measure[Angle], error) { return Parse(text, angleRegistry) }

// ParseGraphicsLength 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseGraphicsLength가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseGraphicsLength(text string) (Measure[GraphicsLength], error) {
	return Parse(text, graphicsLengthRegistry)
}

// ParseVelocity 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseVelocity가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseVelocity(text string) (Measure[Velocity], error) { return Parse(text, velocityRegistry) }

// ParseAcceleration 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: ParseAcceleration가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseAcceleration(text string) (Measure[Acceleration], error) {
	return Parse(text, accelerationRegistry)
}

// HumanLength 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanLength(value Measure[Length]) (string, error) {
	return value.Human(lengthMillimeter, lengthCentimeter, lengthMeter, lengthKilometer)
}

// HumanTime 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanTime(value Measure[Time]) (string, error) {
	return value.Human(timeMillisecond, timeSecond, timeMinute, timeHour)
}

// HumanMass 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanMass에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanMass(value Measure[Mass]) (string, error) {
	return value.Human(massGram, massKilogram, massTon)
}

// HumanArea 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanArea에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanArea(value Measure[Area]) (string, error) {
	return value.Human(areaSquareMillimeter, areaSquareCentimeter, areaSquareMeter, areaSquareKilometer)
}

// HumanVolume 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanVolume에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanVolume(value Measure[Volume]) (string, error) {
	return value.Human(volumeCubicMillimeter, volumeCubicCentimeter, volumeMilliliter, volumeLiter, volumeCubicMeter)
}

// HumanStorage 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanStorage에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanStorage(value Measure[Storage]) (string, error) {
	return value.Human(storageByte, storageKilobyte, storageMegabyte, storageGigabyte, storageTerabyte, storagePetabyte)
}

// HumanBinarySize 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanBinarySize에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanBinarySize(value Measure[BinarySize]) (string, error) {
	return value.Human(binaryBit, binaryByte, binaryKilobyte, binaryMegabyte, binaryGigabyte, binaryTerabyte, binaryPetabyte)
}

// HumanFrequency 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanFrequency에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanFrequency(value Measure[Frequency]) (string, error) {
	return value.Human(frequencyHertz, frequencyKilohertz, frequencyMegahertz, frequencyGigahertz)
}

// HumanEnergy 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanEnergy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanEnergy(value Measure[Energy]) (string, error) {
	return value.Human(energyJoule, energyKilojoule, energyMegajoule, energyWattHour, energyKilowattHour)
}

// HumanPower 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanPower에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanPower(value Measure[Power]) (string, error) {
	return value.Human(powerMilliwatt, powerWatt, powerKilowatt, powerMegawatt, powerGigawatt)
}

// HumanPressure 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanPressure에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanPressure(value Measure[Pressure]) (string, error) {
	return value.Human(pressurePascal, pressureHectopascal, pressureKilopascal, pressureMegapascal, pressureGigapascal, pressureBar, pressureAtmosphere, pressurePSI)
}

// HumanAngle 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - value: HumanAngle에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func HumanAngle(value Measure[Angle]) (string, error) {
	degrees, err := value.In(angleDegree)
	if err != nil {
		return "", err
	}
	normalized := math.Mod(math.Mod(degrees, 360)+360, 360)
	return formatValue(normalized, angleDegree), nil
}

// Sin 각도 값의 삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: Sin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Sin(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Sin(radians), nil
}

// Cos 각도 값의 삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: Cos에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Cos(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Cos(radians), nil
}

// Tan 각도 값의 삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: Tan에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Tan(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Tan(radians), nil
}

// ASin 역삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: ASin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ASin(value float64) (Measure[Angle], error) {
	return New(math.Asin(value), angleRadian)
}

// MustASin 역삼각함수 계산에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustASin에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustASin(value float64) Measure[Angle] {
	angle, err := ASin(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ACos 역삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: ACos에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ACos(value float64) (Measure[Angle], error) {
	return New(math.Acos(value), angleRadian)
}

// MustACos 역삼각함수 계산에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustACos에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustACos(value float64) Measure[Angle] {
	angle, err := ACos(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan 역삼각함수 결과를 반환한다.
//
// 매개변수:
//   - value: ATan에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ATan(value float64) (Measure[Angle], error) {
	return New(math.Atan(value), angleRadian)
}

// MustATan 역삼각함수 계산에 실패하면 panic한다.
//
// 매개변수:
//   - value: MustATan에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustATan(value float64) Measure[Angle] {
	angle, err := ATan(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan2 역삼각함수 결과를 반환한다.
//
// 매개변수:
//   - y: ATan2에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - x: ATan2에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ATan2(y, x float64) (Measure[Angle], error) {
	return New(math.Atan2(y, x), angleRadian)
}

// MustATan2 역삼각함수 계산에 실패하면 panic한다.
//
// 매개변수:
//   - y: MustATan2에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - x: MustATan2에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustATan2(y, x float64) Measure[Angle] {
	angle, err := ATan2(y, x)
	if err != nil {
		panic(err)
	}
	return angle
}
