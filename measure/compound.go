package measure

import "fmt"

// ProductUnit는 ProductUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - first: ProductUnit 동작에 필요한 first 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - second: ProductUnit 동작에 필요한 second 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ProductUnit[A, B any](first Unit[A], second Unit[B]) (Unit[Product[A, B]], error) {
	if err := first.validate(); err != nil {
		return Unit[Product[A, B]]{}, err
	}
	if err := second.validate(); err != nil {
		return Unit[Product[A, B]]{}, err
	}
	suffix := fmt.Sprintf("%s*%s", first.suffix, second.suffix)
	if first.suffix == second.suffix && first.ratio == second.ratio {
		suffix = fmt.Sprintf("(%s)^2", first.suffix)
	}
	return NewUnit[Product[A, B]](first.name+"*"+second.name, suffix, first.ratio*second.ratio)
}

// MustProductUnit는 MustProductUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - first: MustProductUnit 동작에 필요한 first 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - second: MustProductUnit 동작에 필요한 second 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustProductUnit[A, B any](first Unit[A], second Unit[B]) Unit[Product[A, B]] {
	unit, err := ProductUnit(first, second)
	if err != nil {
		panic(err)
	}
	return unit
}

// RatioUnit는 RatioUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - numerator: RatioUnit 동작에 필요한 numerator 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - denominator: RatioUnit 동작에 필요한 denominator 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RatioUnit[A, B any](numerator Unit[A], denominator Unit[B]) (Unit[Ratio[A, B]], error) {
	if err := numerator.validate(); err != nil {
		return Unit[Ratio[A, B]]{}, err
	}
	if err := denominator.validate(); err != nil {
		return Unit[Ratio[A, B]]{}, err
	}
	return NewUnit[Ratio[A, B]](
		numerator.name+"/"+denominator.name,
		numerator.suffix+"/"+denominator.suffix,
		numerator.ratio/denominator.ratio,
	)
}

// MustRatioUnit는 MustRatioUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - numerator: MustRatioUnit 동작에 필요한 numerator 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - denominator: MustRatioUnit 동작에 필요한 denominator 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustRatioUnit[A, B any](numerator Unit[A], denominator Unit[B]) Unit[Ratio[A, B]] {
	unit, err := RatioUnit(numerator, denominator)
	if err != nil {
		panic(err)
	}
	return unit
}

// InverseUnit는 InverseUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: InverseUnit 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func InverseUnit[D any](unit Unit[D]) (Unit[Inverse[D]], error) {
	if err := unit.validate(); err != nil {
		return Unit[Inverse[D]]{}, err
	}
	return NewUnit[Inverse[D]]("1/"+unit.name, "1/"+unit.suffix, 1/unit.ratio)
}

// MustInverseUnit는 MustInverseUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: MustInverseUnit 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustInverseUnit[D any](unit Unit[D]) Unit[Inverse[D]] {
	inverse, err := InverseUnit(unit)
	if err != nil {
		panic(err)
	}
	return inverse
}

// Mul는 Mul 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - left: Mul 동작에 필요한 left 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - right: Mul 동작에 필요한 right 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Mul[A, B any](left Measure[A], right Measure[B]) (Measure[Product[A, B]], error) {
	unit, err := ProductUnit(left.unit, right.unit)
	if err != nil {
		return Measure[Product[A, B]]{}, err
	}
	if err := left.validate(); err != nil {
		return Measure[Product[A, B]]{}, err
	}
	if err := right.validate(); err != nil {
		return Measure[Product[A, B]]{}, err
	}
	return New(left.amount*right.amount, unit)
}

// Div는 Div 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - left: Div 동작에 필요한 left 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - right: Div 동작에 필요한 right 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Div[A, B any](left Measure[A], right Measure[B]) (Measure[Ratio[A, B]], error) {
	unit, err := RatioUnit(left.unit, right.unit)
	if err != nil {
		return Measure[Ratio[A, B]]{}, err
	}
	if err := left.validate(); err != nil {
		return Measure[Ratio[A, B]]{}, err
	}
	if err := right.validate(); err != nil {
		return Measure[Ratio[A, B]]{}, err
	}
	if right.amount == 0 {
		return Measure[Ratio[A, B]]{}, fmt.Errorf("%w: divisor amount must be non-zero", ErrDivideByZero)
	}
	return New(left.amount/right.amount, unit)
}

// MulRatioByDenominator는 MulRatioByDenominator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ratio: MulRatioByDenominator 동작에 필요한 ratio 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - denominator: MulRatioByDenominator 동작에 필요한 denominator 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - result: MulRatioByDenominator 동작에 필요한 result 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func MulRatioByDenominator[A, B any](ratio Measure[Ratio[A, B]], denominator Measure[B], result Unit[A]) (Measure[A], error) {
	if err := ratio.validate(); err != nil {
		return Measure[A]{}, err
	}
	if err := denominator.validate(); err != nil {
		return Measure[A]{}, err
	}
	if err := result.validate(); err != nil {
		return Measure[A]{}, fmt.Errorf("%w: %w", ErrInvalidUnit, err)
	}
	base := ratio.amount * ratio.unit.ratio * denominator.amount * denominator.unit.ratio
	return New(base/result.ratio, result)
}

// DivProductByLeft는 DivProductByLeft 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - product: DivProductByLeft 동작에 필요한 product 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - left: DivProductByLeft 동작에 필요한 left 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - result: DivProductByLeft 동작에 필요한 result 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DivProductByLeft[A, B any](product Measure[Product[A, B]], left Measure[A], result Unit[B]) (Measure[B], error) {
	if err := product.validate(); err != nil {
		return Measure[B]{}, err
	}
	if err := left.validate(); err != nil {
		return Measure[B]{}, err
	}
	if err := result.validate(); err != nil {
		return Measure[B]{}, fmt.Errorf("%w: %w", ErrInvalidUnit, err)
	}
	if left.amount == 0 {
		return Measure[B]{}, fmt.Errorf("%w: divisor amount must be non-zero", ErrDivideByZero)
	}
	base := product.amount * product.unit.ratio / (left.amount * left.unit.ratio)
	return New(base/result.ratio, result)
}

// AreaFromLength는 AreaFromLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - width: AreaFromLength 동작에 필요한 width 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - height: AreaFromLength 동작에 필요한 height 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func AreaFromLength(width, height Measure[Length]) (Measure[Area], error) {
	if err := width.validate(); err != nil {
		return Measure[Area]{}, err
	}
	if err := height.validate(); err != nil {
		return Measure[Area]{}, err
	}
	base := width.amount * width.unit.ratio * height.amount * height.unit.ratio
	return New(base/AreaSquareMeter().Ratio(), AreaSquareMeter())
}

// VolumeFromAreaLength는 VolumeFromAreaLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - area: VolumeFromAreaLength 동작에 필요한 area 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - length: VolumeFromAreaLength 동작에 필요한 length 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func VolumeFromAreaLength(area Measure[Area], length Measure[Length]) (Measure[Volume], error) {
	if err := area.validate(); err != nil {
		return Measure[Volume]{}, err
	}
	if err := length.validate(); err != nil {
		return Measure[Volume]{}, err
	}
	base := area.amount * area.unit.ratio * length.amount * length.unit.ratio
	return New(base/VolumeCubicMeter().Ratio(), VolumeCubicMeter())
}

// LengthFromVolumeArea는 LengthFromVolumeArea 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - volume: LengthFromVolumeArea 동작에 필요한 volume 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - area: LengthFromVolumeArea 동작에 필요한 area 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func LengthFromVolumeArea(volume Measure[Volume], area Measure[Area]) (Measure[Length], error) {
	if err := volume.validate(); err != nil {
		return Measure[Length]{}, err
	}
	if err := area.validate(); err != nil {
		return Measure[Length]{}, err
	}
	if area.amount == 0 {
		return Measure[Length]{}, fmt.Errorf("%w: divisor amount must be non-zero", ErrDivideByZero)
	}
	base := volume.amount * volume.unit.ratio / (area.amount * area.unit.ratio)
	return New(base/LengthMeter().Ratio(), LengthMeter())
}

// AreaFromVolumeLength는 AreaFromVolumeLength 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - volume: AreaFromVolumeLength 동작에 필요한 volume 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - length: AreaFromVolumeLength 동작에 필요한 length 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func AreaFromVolumeLength(volume Measure[Volume], length Measure[Length]) (Measure[Area], error) {
	if err := volume.validate(); err != nil {
		return Measure[Area]{}, err
	}
	if err := length.validate(); err != nil {
		return Measure[Area]{}, err
	}
	if length.amount == 0 {
		return Measure[Area]{}, fmt.Errorf("%w: divisor amount must be non-zero", ErrDivideByZero)
	}
	base := volume.amount * volume.unit.ratio / (length.amount * length.unit.ratio)
	return New(base/AreaSquareMeter().Ratio(), AreaSquareMeter())
}

// VelocityFromLengthTime는 VelocityFromLengthTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - length: VelocityFromLengthTime 동작에 필요한 length 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: VelocityFromLengthTime 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func VelocityFromLengthTime(length Measure[Length], duration Measure[Time]) (Measure[Velocity], error) {
	velocity, err := Div(length, duration)
	if err != nil {
		return Measure[Velocity]{}, err
	}
	value, err := velocity.In(VelocityMeterPerSecond())
	if err != nil {
		return Measure[Velocity]{}, err
	}
	return New(value, VelocityMeterPerSecond())
}

// LengthFromVelocityTime는 LengthFromVelocityTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - velocity: LengthFromVelocityTime 동작에 필요한 velocity 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: LengthFromVelocityTime 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func LengthFromVelocityTime(velocity Measure[Velocity], duration Measure[Time]) (Measure[Length], error) {
	return MulRatioByDenominator(velocity, duration, LengthMeter())
}

// PowerFromEnergyTime는 PowerFromEnergyTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - energy: PowerFromEnergyTime 동작에 필요한 energy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: PowerFromEnergyTime 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func PowerFromEnergyTime(energy Measure[Energy], duration Measure[Time]) (Measure[Power], error) {
	if err := energy.validate(); err != nil {
		return Measure[Power]{}, err
	}
	if err := duration.validate(); err != nil {
		return Measure[Power]{}, err
	}
	if duration.amount == 0 {
		return Measure[Power]{}, fmt.Errorf("%w: divisor amount must be non-zero", ErrDivideByZero)
	}
	joules := energy.amount * energy.unit.ratio
	millis := duration.amount * duration.unit.ratio
	return New((joules/(millis/1000))/PowerWatt().Ratio(), PowerWatt())
}

// EnergyFromPowerTime는 EnergyFromPowerTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - power: EnergyFromPowerTime 동작에 필요한 power 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: EnergyFromPowerTime 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func EnergyFromPowerTime(power Measure[Power], duration Measure[Time]) (Measure[Energy], error) {
	if err := power.validate(); err != nil {
		return Measure[Energy]{}, err
	}
	if err := duration.validate(); err != nil {
		return Measure[Energy]{}, err
	}
	watts := power.amount * power.unit.ratio
	millis := duration.amount * duration.unit.ratio
	return New((watts*(millis/1000))/EnergyJoule().Ratio(), EnergyJoule())
}
