package measure

import "fmt"

// ProductUnit 두 단위를 곱한 product unit을 만든다.
//
// 매개변수:
//   - first: ProductUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - second: ProductUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MustProductUnit product unit 생성에 실패하면 panic한다.
//
// 매개변수:
//   - first: MustProductUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - second: MustProductUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustProductUnit[A, B any](first Unit[A], second Unit[B]) Unit[Product[A, B]] {
	unit, err := ProductUnit(first, second)
	if err != nil {
		panic(err)
	}
	return unit
}

// RatioUnit 분자/분모 단위로 ratio unit을 만든다.
//
// 매개변수:
//   - numerator: RatioUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - denominator: RatioUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MustRatioUnit ratio unit 생성에 실패하면 panic한다.
//
// 매개변수:
//   - numerator: MustRatioUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - denominator: MustRatioUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustRatioUnit[A, B any](numerator Unit[A], denominator Unit[B]) Unit[Ratio[A, B]] {
	unit, err := RatioUnit(numerator, denominator)
	if err != nil {
		panic(err)
	}
	return unit
}

// InverseUnit 값을 대상 단위나 표현으로 변환한다.
//
// 매개변수:
//   - unit: InverseUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func InverseUnit[D any](unit Unit[D]) (Unit[Inverse[D]], error) {
	if err := unit.validate(); err != nil {
		return Unit[Inverse[D]]{}, err
	}
	return NewUnit[Inverse[D]]("1/"+unit.name, "1/"+unit.suffix, 1/unit.ratio)
}

// MustInverseUnit 역단위 생성에 실패하면 panic한다.
//
// 매개변수:
//   - unit: MustInverseUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func MustInverseUnit[D any](unit Unit[D]) Unit[Inverse[D]] {
	inverse, err := InverseUnit(unit)
	if err != nil {
		panic(err)
	}
	return inverse
}

// Mul 현재 값에 입력 값을 곱한 결과를 반환한다.
//
// 매개변수:
//   - left: Mul에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - right: Mul에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Div 현재 값을 입력 값으로 나눈 결과를 반환한다.
//
// 매개변수:
//   - left: Div에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - right: Div에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MulRatioByDenominator ratio 값에 분모 측정값을 곱해 분자 측정값을 계산한다.
//
// 매개변수:
//   - ratio: MulRatioByDenominator에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - denominator: MulRatioByDenominator에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - result: MulRatioByDenominator에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// DivProductByLeft product 값을 왼쪽 측정값으로 나눠 오른쪽 측정값을 계산한다.
//
// 매개변수:
//   - product: DivProductByLeft에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - left: DivProductByLeft에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - result: DivProductByLeft에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// AreaFromLength 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - width: AreaFromLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - height: AreaFromLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// VolumeFromAreaLength 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - area: VolumeFromAreaLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - length: VolumeFromAreaLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// LengthFromVolumeArea 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - volume: LengthFromVolumeArea에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - area: LengthFromVolumeArea에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// AreaFromVolumeLength 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - volume: AreaFromVolumeLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - length: AreaFromVolumeLength에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// VelocityFromLengthTime 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - length: VelocityFromLengthTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: VelocityFromLengthTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// LengthFromVelocityTime 입력 측정값으로 파생 측정값을 계산한다.
//
// 매개변수:
//   - velocity: LengthFromVelocityTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: LengthFromVelocityTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func LengthFromVelocityTime(velocity Measure[Velocity], duration Measure[Time]) (Measure[Length], error) {
	return MulRatioByDenominator(velocity, duration, LengthMeter())
}

// PowerFromEnergyTime energy와 time으로 power를 계산한다.
//
// 매개변수:
//   - energy: PowerFromEnergyTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: PowerFromEnergyTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// EnergyFromPowerTime power와 time으로 energy를 계산한다.
//
// 매개변수:
//   - power: EnergyFromPowerTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: EnergyFromPowerTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
