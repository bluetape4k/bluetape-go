# Issue #34 Measured Values Spec

Issue: #34
Title: Port measured value and unit helper types
Date: 2026-06-08
Milestone: 0.6.0
Work type: Type A full feature
Target package: `measure`

## Goal

Add a Go-native measured value package that ports the useful core of
`bluetape4k-projects/utils/measured` without mechanically copying Kotlin
extension APIs. The package must provide a coherent unit model that can express
plain units, compatible conversions, human formatting, parsing, and selected
compound units such as velocity, acceleration, area, volume, energy, and power.

The first #34 implementation should cover the source unit families listed in the
issue instead of narrowing to only binary size or duration helpers.

## Source Evidence

- `bluetape4k-projects/utils/measured/README.md` describes `Units`,
  `Measure<T>`, `UnitsProduct`, `UnitsRatio`, `InverseUnits`, and the provided
  unit families.
- `bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured`
  contains source families:
  - `Length.kt`
  - `Time.kt`
  - `Mass.kt`
  - `Volume.kt`
  - `Temperature.kt`
  - `Angle.kt`
  - `Area.kt`
  - `Storage.kt`
  - `BinarySize.kt`
  - `Frequency.kt`
  - `EnergyPower.kt`
  - `Motion.kt`
  - `GraphicsLength.kt`
  - `Pressure.kt`
- `bluetape4k-exposed/exposed/exposed-measured` stores measured values as
  base-unit `DOUBLE` columns. #34 does not add DB integration, but the Go model
  should expose base-unit conversion cleanly enough for a future adapter.
- Earlier `docs/research/2026-06-01-milestone-0.6.0-utilities-research.md`
  described `measure` too narrowly as binary size and related helpers. Current
  issue #34 supersedes that scope with full `bluetape4k-measured` family
  comparison.

## Non-Goals

- No database, Exposed, SQL, JSON schema, or persistence adapter in #34.
- No external units dependency unless Step 3-R review proves a dependency is
  clearly safer than a first-party implementation. The expected path is
  first-party.
- No global mutable registry.
- No reflection-heavy dimensional analysis engine.
- No automatic locale-aware i18n formatting. Use stable ASCII suffixes and
  deterministic numeric formatting.
- No arbitrary user-defined unit DSL in #34. The public core can be extensible
  through `Unit[D]`, but built-in registries define supported parsing and
  formatting.

## Package Shape

Use package `measure`.

Rationale:

- `measure.Measure` reads naturally and avoids package stutter compared with
  `measured.Measure`.
- The package is a portable utility peer of `id` and `jwt`.
- The name leaves room for measured values, unit metadata, and format/parse
  helpers without becoming a catch-all `utils` package.

## Core Types

The core model should be generic enough for type-safe compatible conversions
while remaining ordinary Go.

Required public shape:

```go
type Unit[D any] struct { /* immutable value */ }
type Measure[D any] struct { /* amount + Unit[D] */ }
type Registry[D any] struct { /* immutable suffix lookup */ }

func NewUnit[D any](name, suffix string, ratio float64, options ...UnitOption) (Unit[D], error)
func MustUnit[D any](name, suffix string, ratio float64, options ...UnitOption) Unit[D]

func New[D any](amount float64, unit Unit[D]) (Measure[D], error)
func Must[D any](amount float64, unit Unit[D]) Measure[D]

func NewRegistry[D any](units ...Unit[D]) (Registry[D], error)
func MustRegistry[D any](units ...Unit[D]) Registry[D]
```

Required `Unit[D]` behavior:

- Expose suffix, ratio-to-base, and whether to include a space before suffix.
- Zero-value `Unit[D]` is invalid.
- Package-defined unit values are immutable values and safe for concurrent use.
- Equality can use value equality for built-in units.
- `ErrInvalidUnit` is returned for zero/invalid units or invalid ratios.
- `NewUnit` rejects blank name/suffix, NaN ratio, infinity ratio, zero ratio, and
  negative ratio.
- `MustUnit` panics only for package initialization and examples where failure
  would be a programmer error; normal caller code should use `NewUnit`.

Required `Measure[D]` behavior:

- `Amount() float64`
- `Unit() Unit[D]`
- `In(unit Unit[D]) (float64, error)`
- `As(unit Unit[D]) (Measure[D], error)`
- `Add(other Measure[D]) (Measure[D], error)`
- `Sub(other Measure[D]) (Measure[D], error)`
- `MulScalar(value float64) Measure[D]`
- `DivScalar(value float64) (Measure[D], error)`
- `Neg() Measure[D]`
- `Compare(other Measure[D]) (int, error)`
- `ToNearest(nearest float64) (Measure[D], error)`
- `Format(unit Unit[D]) (string, error)`
- `String() string`

Error contracts:

- `ErrInvalidUnit`
- `ErrInvalidMeasure`
- `ErrIncompatibleUnit`
- `ErrInvalidParse`
- `ErrDivideByZero`
- Errors must support `errors.Is`.
- Formatting and conversion errors must not silently return zero values.
- `New` rejects NaN and infinity amounts.
- `Must` panics only for package initialization and examples where inputs are
  static; normal caller code should use `New`.
- Zero-value `Measure[D]` is invalid because its zero-value unit is invalid;
  conversion/formatting methods on zero values must return typed errors rather
  than panic.
- `String()` is a no-panic debug/convenience representation. For invalid
  zero-value measures it returns a stable marker such as `<invalid measure>`.
  Callers that need typed error reporting must use `Format(unit)`.

Required `Registry[D]` behavior:

- `NewRegistry` copies supplied unit values into an immutable suffix lookup.
- `NewRegistry` rejects zero/invalid units, blank suffixes, and duplicate
  suffixes with `ErrInvalidUnit` or a typed wrapper carrying suffix metadata.
- `MustRegistry` is only for package initialization and static examples.
- Zero-value `Registry[D]` is invalid; parsing with it returns `ErrInvalidParse`
  and wraps `ErrInvalidUnit`.
- Built-in family units and registries are returned through package accessor
  functions backed by unexported immutable values, so callers cannot reassign
  shared globals while other goroutines parse or convert values.

Floating-point contract:

- Use `float64`, matching the Kotlin source and Exposed base-unit storage model.
- Tests use tolerance helpers for conversions.
- README documents that this is not arbitrary-precision money or decimal math;
  #35 owns money/decimal helpers.

## Compound Units

The package should support type-safe unit products and ratios where practical:

```go
type Product[A, B any] struct{}
type Ratio[A, B any] struct{}
type Inverse[D any] struct{}

func ProductUnit[A, B any](left Unit[A], right Unit[B]) (Unit[Product[A, B]], error)
func RatioUnit[A, B any](numerator Unit[A], denominator Unit[B]) (Unit[Ratio[A, B]], error)
func InverseUnit[D any](unit Unit[D]) (Unit[Inverse[D]], error)

func Mul[A, B any](left Measure[A], right Measure[B]) (Measure[Product[A, B]], error)
func Div[A, B any](left Measure[A], right Measure[B]) (Measure[Ratio[A, B]], error)
```

`Velocity` and `Acceleration` are aliases of ratio dimensions so generic `Div`
interoperates with specialized helpers:

```go
type Length struct{}
type Time struct{}
type Velocity = Ratio[Length, Time]
type Acceleration = Ratio[Length, Product[Time, Time]]
```

Specialized helpers should keep common source-parity operations ergonomic:

- `AreaFromLength(width, height Measure[Length]) (Measure[Area], error)`
- `VolumeFromAreaLength(area Measure[Area], length Measure[Length])`
- `LengthFromVolumeArea(volume Measure[Volume], area Measure[Area])`
- `AreaFromVolumeLength(volume Measure[Volume], length Measure[Length])`
- `VelocityFromLengthTime(length Measure[Length], duration Measure[Time])`
- `LengthFromVelocityTime(velocity Measure[Velocity], duration Measure[Time])`
- `PowerFromEnergyTime(energy Measure[Energy], duration Measure[Time])`
- `EnergyFromPowerTime(power Measure[Power], duration Measure[Time])`

These helpers should return known family units such as square meters, cubic
meters, meters per second, joules, and watts.

## Dimension Markers And Built-In Units

Use exported zero-size marker types for built-in dimensions:

- `Length`
- `Time`
- `Mass`
- `Area`
- `Volume`
- `Storage`
- `BinarySize`
- `Frequency`
- `Energy`
- `Power`
- `Pressure`
- `Angle`
- `GraphicsLength`
- `Velocity` as `Ratio[Length, Time]`
- `Acceleration` as `Ratio[Length, Product[Time, Time]]`

Temperature is affine and must be separate concrete values, not ratio-only
`Measure[Temperature]`:

- `Temperature` concrete value type
- `TemperatureDelta` concrete value type
- `TemperatureUnit` immutable affine unit descriptor

Required built-in units:

| Family | Units |
|---|---|
| Length | millimeter, centimeter, meter, kilometer, inch, foot, mile |
| Time | millisecond, second, minute, hour |
| Mass | gram, kilogram, ton |
| Area | square millimeter, square centimeter, square meter, square kilometer |
| Volume | cubic millimeter, cubic centimeter, milliliter, liter, cubic meter |
| Storage | B, KB, MB, GB, TB, PB, EB, ZB, YB using 1024 ratio |
| BinarySize | B, kB, MB, GB, TB, PB, KiB, MiB, GiB, TiB, PiB, bit, kbit, Mbit, Gbit, Tbit, Pbit |
| Frequency | Hz, kHz, MHz, GHz |
| Energy | J, kJ, MJ, Wh, kWh |
| Power | mW, W, kW, MW, GW |
| Pressure | Pa, hPa, kPa, MPa, GPa, bar, dbar, mbar, atm, psi, torr, mmHg |
| Angle | rad, degree |
| GraphicsLength | px |
| Velocity | m/s, km/hr |
| Acceleration | m/s^2 |
| Temperature | K, degC, degF absolute values |
| TemperatureDelta | K, degC, degF deltas |

Go naming should be Go-native and avoid Kotlin extension names:

- Unit values: `Meter`, `Kilometer`, `Second`, `Hour`, `Kilogram`, `Byte`,
  `Kibibyte`, `Kilobyte`, `Joule`, `Watt`, `Pascal`, `Degree`, `Radian`.
- Constructors: `Meters(v)`, `Kilometers(v)`, `Seconds(v)`, `Hours(v)`,
  `Kilograms(v)`, `Bytes(v)`, `Kibibytes(v)`, `Kilobytes(v)`,
  `Joules(v)`, `Watts(v)`, `Degrees(v)`, `Radians(v)`.
- Avoid Go keywords and ambiguous names. For binary-size decimal KB use
  `Kilobytes10` only if needed to disambiguate from 1024-based `Storage`.

## Formatting

Required formatting:

- `Measure[D].Format(unit)` formats with explicit unit.
- Family-specific `Human()` helpers choose a practical unit where absolute base
  amount is at least 1, following Kotlin source behavior.
- Angle human formatting normalizes to `0..360` degrees by default.
- Temperature and delta formatting support K, degC, and degF.
- Suffixes are stable and README-documented.
- Formatting is deterministic and ASCII except for degree suffix if existing
  docs/tests explicitly choose `deg` as the Go-safe spelling. Prefer `deg` in Go
  identifiers and allow output suffix `°` only after Step 2-R accepts it.
- Numeric rendering uses this rule unless plan review replaces it with stronger
  evidence: round to 9 fractional digits, trim trailing zeroes, and keep at
  least one fractional digit. Examples: `1.0`, `1.5`, `3.141592654`.
- Formatting rejects NaN and infinity values with `ErrInvalidMeasure`.

## Parsing

Parsing is required where suffix-based text is meaningful:

- Generic parser with registry:
  - `Parse[D any](text string, registry Registry[D]) (Measure[D], error)`
  - `Registry[D]` maps suffixes to built-in `Unit[D]` values.
  - `NewRegistry[D](units ...Unit[D])` is the public constructor for custom
    family-scoped registries.
- Family parsers:
  - `ParseLength`
  - `ParseTime`
  - `ParseMass`
  - `ParseArea`
  - `ParseVolume`
  - `ParseStorage`
  - `ParseBinarySize`
  - `ParseFrequency`
  - `ParseEnergy`
  - `ParsePower`
  - `ParsePressure`
  - `ParseAngle`
  - `ParseGraphicsLength`
  - `ParseTemperature`
  - `ParseTemperatureDelta`
- Invalid parse must return `ErrInvalidParse`.
- Unknown suffix must be distinguishable with `errors.Is(err, ErrInvalidUnit)` or
  a typed parse error carrying suffix metadata.
- Parsed amounts must reject NaN and infinity.
- Do not parse expressions such as `kg*m/s^2` in #34 unless the implementation
  remains simple and reviewed. Built-in compound parsers for `m/s`, `km/hr`, and
  `m/s^2` are sufficient if compound expression parsing is deferred.

## Temperature Contract

Temperature is not a ratio-only `Measure`.

Required behavior:

- `FromKelvin`, `FromCelsius`, `FromFahrenheit`.
- Convenience constructors `Kelvin`, `Celsius`, `Fahrenheit` return concrete
  `Temperature` values.
- `InKelvin`, `InCelsius`, `InFahrenheit`.
- `TemperatureDelta` constructors for Kelvin, Celsius, Fahrenheit deltas.
- `Temperature.Add(delta)`, `Temperature.Sub(delta)`, and
  `Temperature.Delta(other)`.
- Fahrenheit delta converts by `5/9`; absolute Fahrenheit converts with offset.
- Compare methods and formatting.
- Document that negative Kelvin is not rejected in #34 if source behavior allows
  raw Kelvin values; if Step 2-R considers this unsafe, reject below absolute
  zero and add tests.

## Concurrency And Cancellation

#34 production APIs are local value operations with immutable built-in units and
no background workers or external I/O.

Required tests:

- `GoroutineStressTester` runs conversion, parsing, formatting, arithmetic, and
  compound helper tasks against shared built-in units.
- `AsyncJobTester` must be used in tests to satisfy #34 stress-test acceptance.
  Because production APIs have no caller-observable cancellation boundary, use it
  only as a test harness around context-aware test jobs that execute representative
  local operations and respect the tester context before work starts. Do not add
  a production async API only for testing.
- Run `go test -race -count=1 ./measure`.

If Step 2-R rejects the `AsyncJobTester` harness-only interpretation, update the
issue before implementation or record a precise N/A rationale with reviewer
approval.

## Documentation

Required docs:

- `measure/README.md`
- `measure/README.ko.md`
- `measure/doc.go`
- Root `README.md` / `README.ko.md` package table updates.
- `CHANGELOG.md` and `WIP.md`.

README package table must show implemented families and explicitly state what is
not included:

- no DB adapter in #34,
- no arbitrary precision money/decimal behavior (#35),
- no locale-aware formatting,
- no arbitrary expression parser unless implemented.

## Acceptance Tests

Tests must cover:

- Conversion ratios for every built-in family in the issue.
- Add/subtract/compare between compatible units.
- Scalar multiply/divide, including divide-by-zero.
- `ToNearest` invalid nearest.
- Human formatting for representative families.
- Parsing valid and invalid text, unknown suffix, malformed number, and
  whitespace.
- Temperature absolute and delta conversions, offset behavior, formatting, and
  comparisons.
- Compound helpers:
  - length * length -> area,
  - area * length -> volume,
  - length / time -> velocity,
  - velocity * time -> length,
  - power * time -> energy,
  - energy / time -> power,
  - acceleration formatting/conversion.
- Boundary values:
  - zero amount,
  - negative amount,
  - large storage/binary values,
  - invalid zero `Unit[D]`,
  - invalid zero `Measure[D]` conversion attempts.
- Stress tests with `GoroutineStressTester` and `AsyncJobTester`.
- Examples compile with `go test -count=1 ./measure -run Example`.

## Validation Commands

```bash
gofmt -w measure
go test -count=1 ./measure
go test -race -count=1 ./measure
go test -count=1 ./...
golangci-lint config verify
golangci-lint run ./measure
git diff --check origin/develop --
rg -n "GoroutineStressTester" measure docs/superpowers/reviews
rg -n "AsyncJobTester" measure docs/superpowers/reviews
rg -n "Length|Time|Mass|Temperature|Storage|BinarySize|Frequency|Energy|Power|Velocity|Acceleration|Area|Volume|Pressure|Angle" measure/README.md measure/README.ko.md measure/doc.go
go list -deps ./measure | rg 'github.com/docker/go-units' && exit 1 || true
```

## Follow-Up Policy

Follow-up issues are required only if implementation cannot safely cover a source
family in #34 after Step 3-R. Follow-up issues must keep the `measure` API
compatible with later family additions and must be linked from README/WIP.
