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

// FromDuration FromDuration 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - duration: FromDuration 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func FromDuration(duration time.Duration) Measure[Time] {
	return Must(float64(duration)/float64(time.Millisecond), timeMillisecond)
}

// LengthMillimeter LengthMillimeter 공개 API의 동작을 수행한다.
func LengthMillimeter() Unit[Length] { return lengthMillimeter }

// LengthCentimeter LengthCentimeter 공개 API의 동작을 수행한다.
func LengthCentimeter() Unit[Length] { return lengthCentimeter }

// LengthMeter LengthMeter 공개 API의 동작을 수행한다.
func LengthMeter() Unit[Length] { return lengthMeter }

// LengthKilometer LengthKilometer 공개 API의 동작을 수행한다.
func LengthKilometer() Unit[Length] { return lengthKilometer }

// LengthInch LengthInch 공개 API의 동작을 수행한다.
func LengthInch() Unit[Length] { return lengthInch }

// LengthFoot LengthFoot 공개 API의 동작을 수행한다.
func LengthFoot() Unit[Length] { return lengthFoot }

// LengthMile LengthMile 공개 API의 동작을 수행한다.
func LengthMile() Unit[Length] { return lengthMile }

// LengthRegistry LengthRegistry 공개 API의 동작을 수행한다.
func LengthRegistry() Registry[Length] { return lengthRegistry }

// TimeMillisecond TimeMillisecond 공개 API의 동작을 수행한다.
func TimeMillisecond() Unit[Time] { return timeMillisecond }

// TimeSecond TimeSecond 공개 API의 동작을 수행한다.
func TimeSecond() Unit[Time] { return timeSecond }

// TimeMinute TimeMinute 공개 API의 동작을 수행한다.
func TimeMinute() Unit[Time] { return timeMinute }

// TimeHour TimeHour 공개 API의 동작을 수행한다.
func TimeHour() Unit[Time] { return timeHour }

// TimeRegistry TimeRegistry 공개 API의 동작을 수행한다.
func TimeRegistry() Registry[Time] { return timeRegistry }

// MassGram MassGram 공개 API의 동작을 수행한다.
func MassGram() Unit[Mass] { return massGram }

// MassKilogram MassKilogram 공개 API의 동작을 수행한다.
func MassKilogram() Unit[Mass] { return massKilogram }

// MassTon MassTon 공개 API의 동작을 수행한다.
func MassTon() Unit[Mass] { return massTon }

// MassRegistry MassRegistry 공개 API의 동작을 수행한다.
func MassRegistry() Registry[Mass] { return massRegistry }

// AreaSquareMillimeter AreaSquareMillimeter 공개 API의 동작을 수행한다.
func AreaSquareMillimeter() Unit[Area] { return areaSquareMillimeter }

// AreaSquareCentimeter AreaSquareCentimeter 공개 API의 동작을 수행한다.
func AreaSquareCentimeter() Unit[Area] { return areaSquareCentimeter }

// AreaSquareMeter AreaSquareMeter 공개 API의 동작을 수행한다.
func AreaSquareMeter() Unit[Area] { return areaSquareMeter }

// AreaSquareKilometer AreaSquareKilometer 공개 API의 동작을 수행한다.
func AreaSquareKilometer() Unit[Area] { return areaSquareKilometer }

// AreaRegistry AreaRegistry 공개 API의 동작을 수행한다.
func AreaRegistry() Registry[Area] { return areaRegistry }

// VolumeCubicMillimeter VolumeCubicMillimeter 공개 API의 동작을 수행한다.
func VolumeCubicMillimeter() Unit[Volume] { return volumeCubicMillimeter }

// VolumeCubicCentimeter VolumeCubicCentimeter 공개 API의 동작을 수행한다.
func VolumeCubicCentimeter() Unit[Volume] { return volumeCubicCentimeter }

// VolumeMilliliter VolumeMilliliter 공개 API의 동작을 수행한다.
func VolumeMilliliter() Unit[Volume] { return volumeMilliliter }

// VolumeLiter VolumeLiter 공개 API의 동작을 수행한다.
func VolumeLiter() Unit[Volume] { return volumeLiter }

// VolumeCubicMeter VolumeCubicMeter 공개 API의 동작을 수행한다.
func VolumeCubicMeter() Unit[Volume] { return volumeCubicMeter }

// VolumeRegistry VolumeRegistry 공개 API의 동작을 수행한다.
func VolumeRegistry() Registry[Volume] { return volumeRegistry }

// StorageByte StorageByte 공개 API의 동작을 수행한다.
func StorageByte() Unit[Storage] { return storageByte }

// StorageKilobyte StorageKilobyte 공개 API의 동작을 수행한다.
func StorageKilobyte() Unit[Storage] { return storageKilobyte }

// StorageMegabyte StorageMegabyte 공개 API의 동작을 수행한다.
func StorageMegabyte() Unit[Storage] { return storageMegabyte }

// StorageGigabyte StorageGigabyte 공개 API의 동작을 수행한다.
func StorageGigabyte() Unit[Storage] { return storageGigabyte }

// StorageTerabyte StorageTerabyte 공개 API의 동작을 수행한다.
func StorageTerabyte() Unit[Storage] { return storageTerabyte }

// StoragePetabyte StoragePetabyte 공개 API의 동작을 수행한다.
func StoragePetabyte() Unit[Storage] { return storagePetabyte }

// StorageExabyte StorageExabyte 공개 API의 동작을 수행한다.
func StorageExabyte() Unit[Storage] { return storageExabyte }

// StorageZettabyte StorageZettabyte 공개 API의 동작을 수행한다.
func StorageZettabyte() Unit[Storage] { return storageZettabyte }

// StorageYottabyte StorageYottabyte 공개 API의 동작을 수행한다.
func StorageYottabyte() Unit[Storage] { return storageYottabyte }

// StorageRegistry StorageRegistry 공개 API의 동작을 수행한다.
func StorageRegistry() Registry[Storage] { return storageRegistry }

// BinaryByte BinaryByte 공개 API의 동작을 수행한다.
func BinaryByte() Unit[BinarySize] { return binaryByte }

// BinaryKilobyte BinaryKilobyte 공개 API의 동작을 수행한다.
func BinaryKilobyte() Unit[BinarySize] { return binaryKilobyte }

// BinaryMegabyte BinaryMegabyte 공개 API의 동작을 수행한다.
func BinaryMegabyte() Unit[BinarySize] { return binaryMegabyte }

// BinaryGigabyte BinaryGigabyte 공개 API의 동작을 수행한다.
func BinaryGigabyte() Unit[BinarySize] { return binaryGigabyte }

// BinaryTerabyte BinaryTerabyte 공개 API의 동작을 수행한다.
func BinaryTerabyte() Unit[BinarySize] { return binaryTerabyte }

// BinaryPetabyte BinaryPetabyte 공개 API의 동작을 수행한다.
func BinaryPetabyte() Unit[BinarySize] { return binaryPetabyte }

// BinaryKibibyte BinaryKibibyte 공개 API의 동작을 수행한다.
func BinaryKibibyte() Unit[BinarySize] { return binaryKibibyte }

// BinaryMebibyte BinaryMebibyte 공개 API의 동작을 수행한다.
func BinaryMebibyte() Unit[BinarySize] { return binaryMebibyte }

// BinaryGibibyte BinaryGibibyte 공개 API의 동작을 수행한다.
func BinaryGibibyte() Unit[BinarySize] { return binaryGibibyte }

// BinaryTebibyte BinaryTebibyte 공개 API의 동작을 수행한다.
func BinaryTebibyte() Unit[BinarySize] { return binaryTebibyte }

// BinaryPebibyte BinaryPebibyte 공개 API의 동작을 수행한다.
func BinaryPebibyte() Unit[BinarySize] { return binaryPebibyte }

// BinaryBit BinaryBit 공개 API의 동작을 수행한다.
func BinaryBit() Unit[BinarySize] { return binaryBit }

// BinaryKilobit BinaryKilobit 공개 API의 동작을 수행한다.
func BinaryKilobit() Unit[BinarySize] { return binaryKilobit }

// BinaryMegabit BinaryMegabit 공개 API의 동작을 수행한다.
func BinaryMegabit() Unit[BinarySize] { return binaryMegabit }

// BinaryGigabit BinaryGigabit 공개 API의 동작을 수행한다.
func BinaryGigabit() Unit[BinarySize] { return binaryGigabit }

// BinaryTerabit BinaryTerabit 공개 API의 동작을 수행한다.
func BinaryTerabit() Unit[BinarySize] { return binaryTerabit }

// BinaryPetabit BinaryPetabit 공개 API의 동작을 수행한다.
func BinaryPetabit() Unit[BinarySize] { return binaryPetabit }

// BinarySizeRegistry BinarySizeRegistry 공개 API의 동작을 수행한다.
func BinarySizeRegistry() Registry[BinarySize] { return binarySizeRegistry }

// FrequencyHertz FrequencyHertz 공개 API의 동작을 수행한다.
func FrequencyHertz() Unit[Frequency] { return frequencyHertz }

// FrequencyKilohertz FrequencyKilohertz 공개 API의 동작을 수행한다.
func FrequencyKilohertz() Unit[Frequency] { return frequencyKilohertz }

// FrequencyMegahertz FrequencyMegahertz 공개 API의 동작을 수행한다.
func FrequencyMegahertz() Unit[Frequency] { return frequencyMegahertz }

// FrequencyGigahertz FrequencyGigahertz 공개 API의 동작을 수행한다.
func FrequencyGigahertz() Unit[Frequency] { return frequencyGigahertz }

// FrequencyRegistry FrequencyRegistry 공개 API의 동작을 수행한다.
func FrequencyRegistry() Registry[Frequency] { return frequencyRegistry }

// EnergyJoule EnergyJoule 공개 API의 동작을 수행한다.
func EnergyJoule() Unit[Energy] { return energyJoule }

// EnergyKilojoule EnergyKilojoule 공개 API의 동작을 수행한다.
func EnergyKilojoule() Unit[Energy] { return energyKilojoule }

// EnergyMegajoule EnergyMegajoule 공개 API의 동작을 수행한다.
func EnergyMegajoule() Unit[Energy] { return energyMegajoule }

// EnergyWattHour EnergyWattHour 공개 API의 동작을 수행한다.
func EnergyWattHour() Unit[Energy] { return energyWattHour }

// EnergyKilowattHour EnergyKilowattHour 공개 API의 동작을 수행한다.
func EnergyKilowattHour() Unit[Energy] { return energyKilowattHour }

// EnergyRegistry EnergyRegistry 공개 API의 동작을 수행한다.
func EnergyRegistry() Registry[Energy] { return energyRegistry }

// PowerMilliwatt PowerMilliwatt 공개 API의 동작을 수행한다.
func PowerMilliwatt() Unit[Power] { return powerMilliwatt }

// PowerWatt PowerWatt 공개 API의 동작을 수행한다.
func PowerWatt() Unit[Power] { return powerWatt }

// PowerKilowatt PowerKilowatt 공개 API의 동작을 수행한다.
func PowerKilowatt() Unit[Power] { return powerKilowatt }

// PowerMegawatt PowerMegawatt 공개 API의 동작을 수행한다.
func PowerMegawatt() Unit[Power] { return powerMegawatt }

// PowerGigawatt PowerGigawatt 공개 API의 동작을 수행한다.
func PowerGigawatt() Unit[Power] { return powerGigawatt }

// PowerRegistry PowerRegistry 공개 API의 동작을 수행한다.
func PowerRegistry() Registry[Power] { return powerRegistry }

// PressurePascal PressurePascal 공개 API의 동작을 수행한다.
func PressurePascal() Unit[Pressure] { return pressurePascal }

// PressureHectopascal PressureHectopascal 공개 API의 동작을 수행한다.
func PressureHectopascal() Unit[Pressure] { return pressureHectopascal }

// PressureKilopascal PressureKilopascal 공개 API의 동작을 수행한다.
func PressureKilopascal() Unit[Pressure] { return pressureKilopascal }

// PressureMegapascal PressureMegapascal 공개 API의 동작을 수행한다.
func PressureMegapascal() Unit[Pressure] { return pressureMegapascal }

// PressureGigapascal PressureGigapascal 공개 API의 동작을 수행한다.
func PressureGigapascal() Unit[Pressure] { return pressureGigapascal }

// PressureBar PressureBar 공개 API의 동작을 수행한다.
func PressureBar() Unit[Pressure] { return pressureBar }

// PressureDecibar PressureDecibar 공개 API의 동작을 수행한다.
func PressureDecibar() Unit[Pressure] { return pressureDecibar }

// PressureMillibar PressureMillibar 공개 API의 동작을 수행한다.
func PressureMillibar() Unit[Pressure] { return pressureMillibar }

// PressureAtmosphere PressureAtmosphere 공개 API의 동작을 수행한다.
func PressureAtmosphere() Unit[Pressure] { return pressureAtmosphere }

// PressurePSI PressurePSI 공개 API의 동작을 수행한다.
func PressurePSI() Unit[Pressure] { return pressurePSI }

// PressureTorr PressureTorr 공개 API의 동작을 수행한다.
func PressureTorr() Unit[Pressure] { return pressureTorr }

// PressureMillimeterMercury PressureMillimeterMercury 공개 API의 동작을 수행한다.
func PressureMillimeterMercury() Unit[Pressure] { return pressureMillimeterMercury }

// PressureRegistry PressureRegistry 공개 API의 동작을 수행한다.
func PressureRegistry() Registry[Pressure] { return pressureRegistry }

// AngleRadian AngleRadian 공개 API의 동작을 수행한다.
func AngleRadian() Unit[Angle] { return angleRadian }

// AngleDegree AngleDegree 공개 API의 동작을 수행한다.
func AngleDegree() Unit[Angle] { return angleDegree }

// AngleRegistry AngleRegistry 공개 API의 동작을 수행한다.
func AngleRegistry() Registry[Angle] { return angleRegistry }

// GraphicsPixel GraphicsPixel 공개 API의 동작을 수행한다.
func GraphicsPixel() Unit[GraphicsLength] { return graphicsPixel }

// GraphicsLengthRegistry GraphicsLengthRegistry 공개 API의 동작을 수행한다.
func GraphicsLengthRegistry() Registry[GraphicsLength] { return graphicsLengthRegistry }

// VelocityMeterPerSecond VelocityMeterPerSecond 공개 API의 동작을 수행한다.
func VelocityMeterPerSecond() Unit[Velocity] { return velocityMeterPerSecond }

// VelocityKilometerPerHour VelocityKilometerPerHour 공개 API의 동작을 수행한다.
func VelocityKilometerPerHour() Unit[Velocity] { return velocityKilometerPerHour }

// VelocityRegistry VelocityRegistry 공개 API의 동작을 수행한다.
func VelocityRegistry() Registry[Velocity] { return velocityRegistry }

// AccelerationMeterPerSecondSquared AccelerationMeterPerSecondSquared 공개 API의 동작을 수행한다.
func AccelerationMeterPerSecondSquared() Unit[Acceleration] {
	return accelerationMeterPerSecondSquared
}

// AccelerationRegistry AccelerationRegistry 공개 API의 동작을 수행한다.
func AccelerationRegistry() Registry[Acceleration] { return accelerationRegistry }

// Duration Duration 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Duration 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// ParseLength ParseLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseLength가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseLength(text string) (Measure[Length], error) { return Parse(text, lengthRegistry) }

// ParseTime ParseTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseTime가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseTime(text string) (Measure[Time], error) { return Parse(text, timeRegistry) }

// ParseMass ParseMass 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseMass가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseMass(text string) (Measure[Mass], error) { return Parse(text, massRegistry) }

// ParseArea ParseArea 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseArea가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseArea(text string) (Measure[Area], error) { return Parse(text, areaRegistry) }

// ParseVolume ParseVolume 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseVolume가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseVolume(text string) (Measure[Volume], error) { return Parse(text, volumeRegistry) }

// ParseStorage ParseStorage 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseStorage가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseStorage(text string) (Measure[Storage], error) { return Parse(text, storageRegistry) }

// ParseBinarySize ParseBinarySize 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseBinarySize가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseBinarySize(text string) (Measure[BinarySize], error) {
	return Parse(text, binarySizeRegistry)
}

// ParseFrequency ParseFrequency 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseFrequency가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseFrequency(text string) (Measure[Frequency], error) { return Parse(text, frequencyRegistry) }

// ParseEnergy ParseEnergy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseEnergy가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseEnergy(text string) (Measure[Energy], error) { return Parse(text, energyRegistry) }

// ParsePower ParsePower 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParsePower가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParsePower(text string) (Measure[Power], error) { return Parse(text, powerRegistry) }

// ParsePressure ParsePressure 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParsePressure가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParsePressure(text string) (Measure[Pressure], error) { return Parse(text, pressureRegistry) }

// ParseAngle ParseAngle 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseAngle가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseAngle(text string) (Measure[Angle], error) { return Parse(text, angleRegistry) }

// ParseGraphicsLength ParseGraphicsLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseGraphicsLength가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseGraphicsLength(text string) (Measure[GraphicsLength], error) {
	return Parse(text, graphicsLengthRegistry)
}

// ParseVelocity ParseVelocity 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseVelocity가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseVelocity(text string) (Measure[Velocity], error) { return Parse(text, velocityRegistry) }

// ParseAcceleration ParseAcceleration 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: ParseAcceleration가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseAcceleration(text string) (Measure[Acceleration], error) {
	return Parse(text, accelerationRegistry)
}

// HumanLength HumanLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanLength 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanLength(value Measure[Length]) (string, error) {
	return value.Human(lengthMillimeter, lengthCentimeter, lengthMeter, lengthKilometer)
}

// HumanTime HumanTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanTime 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanTime(value Measure[Time]) (string, error) {
	return value.Human(timeMillisecond, timeSecond, timeMinute, timeHour)
}

// HumanMass HumanMass 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanMass 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanMass(value Measure[Mass]) (string, error) {
	return value.Human(massGram, massKilogram, massTon)
}

// HumanArea HumanArea 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanArea 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanArea(value Measure[Area]) (string, error) {
	return value.Human(areaSquareMillimeter, areaSquareCentimeter, areaSquareMeter, areaSquareKilometer)
}

// HumanVolume HumanVolume 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanVolume 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanVolume(value Measure[Volume]) (string, error) {
	return value.Human(volumeCubicMillimeter, volumeCubicCentimeter, volumeMilliliter, volumeLiter, volumeCubicMeter)
}

// HumanStorage HumanStorage 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanStorage 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanStorage(value Measure[Storage]) (string, error) {
	return value.Human(storageByte, storageKilobyte, storageMegabyte, storageGigabyte, storageTerabyte, storagePetabyte)
}

// HumanBinarySize HumanBinarySize 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanBinarySize 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanBinarySize(value Measure[BinarySize]) (string, error) {
	return value.Human(binaryBit, binaryByte, binaryKilobyte, binaryMegabyte, binaryGigabyte, binaryTerabyte, binaryPetabyte)
}

// HumanFrequency HumanFrequency 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanFrequency 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanFrequency(value Measure[Frequency]) (string, error) {
	return value.Human(frequencyHertz, frequencyKilohertz, frequencyMegahertz, frequencyGigahertz)
}

// HumanEnergy HumanEnergy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanEnergy 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanEnergy(value Measure[Energy]) (string, error) {
	return value.Human(energyJoule, energyKilojoule, energyMegajoule, energyWattHour, energyKilowattHour)
}

// HumanPower HumanPower 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanPower 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanPower(value Measure[Power]) (string, error) {
	return value.Human(powerMilliwatt, powerWatt, powerKilowatt, powerMegawatt, powerGigawatt)
}

// HumanPressure HumanPressure 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanPressure 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanPressure(value Measure[Pressure]) (string, error) {
	return value.Human(pressurePascal, pressureHectopascal, pressureKilopascal, pressureMegapascal, pressureGigapascal, pressureBar, pressureAtmosphere, pressurePSI)
}

// HumanAngle HumanAngle 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: HumanAngle 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func HumanAngle(value Measure[Angle]) (string, error) {
	degrees, err := value.In(angleDegree)
	if err != nil {
		return "", err
	}
	normalized := math.Mod(math.Mod(degrees, 360)+360, 360)
	return formatValue(normalized, angleDegree), nil
}

// Sin Sin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Sin 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Sin(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Sin(radians), nil
}

// Cos Cos 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Cos 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Cos(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Cos(radians), nil
}

// Tan Tan 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Tan 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Tan(value Measure[Angle]) (float64, error) {
	radians, err := value.In(angleRadian)
	if err != nil {
		return 0, err
	}
	return math.Tan(radians), nil
}

// ASin ASin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ASin 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ASin(value float64) (Measure[Angle], error) {
	return New(math.Asin(value), angleRadian)
}

// MustASin MustASin 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustASin 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustASin(value float64) Measure[Angle] {
	angle, err := ASin(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ACos ACos 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ACos 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ACos(value float64) (Measure[Angle], error) {
	return New(math.Acos(value), angleRadian)
}

// MustACos MustACos 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustACos 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustACos(value float64) Measure[Angle] {
	angle, err := ACos(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan ATan 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ATan 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ATan(value float64) (Measure[Angle], error) {
	return New(math.Atan(value), angleRadian)
}

// MustATan MustATan 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: MustATan 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustATan(value float64) Measure[Angle] {
	angle, err := ATan(value)
	if err != nil {
		panic(err)
	}
	return angle
}

// ATan2 ATan2 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - y: ATan2 동작에 필요한 y 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - x: ATan2 동작에 필요한 x 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ATan2(y, x float64) (Measure[Angle], error) {
	return New(math.Atan2(y, x), angleRadian)
}

// MustATan2 MustATan2 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - y: MustATan2 동작에 필요한 y 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - x: MustATan2 동작에 필요한 x 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustATan2(y, x float64) Measure[Angle] {
	angle, err := ATan2(y, x)
	if err != nil {
		panic(err)
	}
	return angle
}
