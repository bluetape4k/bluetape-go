package measure

import "fmt"

// ProductUnit  두 단위의 곱 단위를 생성합니다.
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

// MustProductUnit  곱 단위 생성 실패 시 panic을 발생시킵니다.
func MustProductUnit[A, B any](first Unit[A], second Unit[B]) Unit[Product[A, B]] {
	unit, err := ProductUnit(first, second)
	if err != nil {
		panic(err)
	}
	return unit
}

// RatioUnit  두 단위의 비율 단위를 생성합니다.
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

// MustRatioUnit  비율 단위 생성 실패 시 panic을 발생시킵니다.
func MustRatioUnit[A, B any](numerator Unit[A], denominator Unit[B]) Unit[Ratio[A, B]] {
	unit, err := RatioUnit(numerator, denominator)
	if err != nil {
		panic(err)
	}
	return unit
}

// InverseUnit  한 단위의 역수 단위를 생성합니다.
func InverseUnit[D any](unit Unit[D]) (Unit[Inverse[D]], error) {
	if err := unit.validate(); err != nil {
		return Unit[Inverse[D]]{}, err
	}
	return NewUnit[Inverse[D]]("1/"+unit.name, "1/"+unit.suffix, 1/unit.ratio)
}

// MustInverseUnit  역수 단위 생성 실패 시 panic을 발생시킵니다.
func MustInverseUnit[D any](unit Unit[D]) Unit[Inverse[D]] {
	inverse, err := InverseUnit(unit)
	if err != nil {
		panic(err)
	}
	return inverse
}

// Mul  두 측정값을 곱해 곱 차원 측정값을 반환합니다.
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

// Div  두 측정값을 나눠 비율 차원 측정값을 반환합니다.
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

// MulRatioByDenominator  Ratio[A,B] 측정값에 B 측정값을 곱해 A 측정값을 반환합니다.
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

// DivProductByLeft  Product[A,B] 측정값을 A 측정값으로 나눠 B 측정값을 반환합니다.
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

// AreaFromLength  두 길이 측정값을 곱해 square meter 기준 면적을 반환합니다.
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

// VolumeFromAreaLength  면적과 길이를 곱해 cubic meter 기준 부피를 반환합니다.
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

// LengthFromVolumeArea  부피를 면적으로 나눠 meter 기준 길이를 반환합니다.
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

// AreaFromVolumeLength  부피를 길이로 나눠 square meter 기준 면적을 반환합니다.
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

// VelocityFromLengthTime  길이를 시간으로 나눠 m/s 기준 속도를 반환합니다.
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

// LengthFromVelocityTime  속도와 시간을 곱해 meter 기준 길이를 반환합니다.
func LengthFromVelocityTime(velocity Measure[Velocity], duration Measure[Time]) (Measure[Length], error) {
	return MulRatioByDenominator(velocity, duration, LengthMeter())
}

// PowerFromEnergyTime  에너지를 시간으로 나눠 watt 기준 전력을 반환합니다.
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

// EnergyFromPowerTime  전력과 시간을 곱해 joule 기준 에너지를 반환합니다.
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
