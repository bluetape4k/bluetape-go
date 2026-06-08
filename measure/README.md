# measure

[English](README.md) | [한국어](README.ko.md)

`measure` provides typed units, measured values, parsing, formatting, compound
units, and affine temperature helpers inspired by `bluetape4k-measured`.

## Import

```go
import "github.com/bluetape4k/bluetape-go/measure"
```

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Type-safe physical quantity | `Measure[D]` with a built-in `Unit[D]` | Dimensions prevent mixing length, time, mass, etc. |
| Convert or format values | `In`, `As`, `Format`, `Human*` | Conversion uses each unit's ratio to its base unit. |
| Parse user-facing text | `Parse*` or generic `Parse` | Registries are explicit and suffix-scoped. |
| Compound units | `ProductUnit`, `RatioUnit`, `Mul`, `Div` | Includes velocity and acceleration built-ins. |
| Absolute temperature | `Temperature` | Affine Kelvin/Celsius/Fahrenheit conversion is not modeled as `Measure`. |
| Money or decimal exactness | Deferred | Use the future money package; `measure` uses `float64`. |

## Usage

```go
distance := measure.Must(1500, measure.LengthMeter)
text, err := distance.Format(measure.LengthKilometer)
if err != nil {
    return err
}
fmt.Println(text) // 1.5 km

speed, err := measure.Div(
    measure.Must(100, measure.LengthMeter),
    measure.Must(9.58, measure.TimeSecond),
)
if err != nil {
    return err
}
metersPerSecond, err := speed.In(measure.VelocityMeterPerSecond)
```

## Behavior

- `Unit[D]` is an immutable value with name, suffix, base-unit ratio, and
  formatting spacing metadata.
- Built-in families cover Length, Time, Mass, Area, Volume, Storage,
  BinarySize, Frequency, Energy, Power, Pressure, Angle, GraphicsLength,
  Velocity, Acceleration, Temperature, and TemperatureDelta.
- `Measure[D]` stores the amount in its current unit. `In` converts with
  `amount * from.Ratio() / to.Ratio()`.
- Public constructors and error-returning operations wrap repo-owned sentinels:
  `ErrInvalidUnit`, `ErrInvalidMeasure`, and `ErrInvalidParse`.
- `String()` is no-panic debug formatting. Use `Format` or `Parse` when callers
  need typed validation failures.
- Built-in storage units use 1024 ratios (`KB`, `MB`, ...). Binary size keeps a
  separate registry for decimal bytes (`kB`, `MB`, ...), IEC bytes (`KiB`, ...),
  and bit units.
- Temperature is affine: `Temperature` stores Kelvin absolute values, while
  `TemperatureDelta` stores Kelvin deltas.
- `measure` does not import `github.com/docker/go-units`; it is first-party so
  compound units, typed dimensions, registry parsing, and temperature contracts
  stay under this module's API.

## Test

```bash
go test -count=1 ./measure
go test -race -count=1 ./measure
```
