# Issue #34 Preimplementation Risk Record

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Issue: #34 `Port measured value and unit helper types`
- Branch: `issue-34-measured`
- Source baseline: `origin/develop` merge commit `58bccab3a3ff0ad9ebba0bcd4739fae1fa050f99`
- Planned Go package: `measure`

## Source Evidence

Kotlin source parity was checked against:

- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Units.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Length.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Time.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Mass.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Area.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Volume.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Storage.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/BinarySize.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Frequency.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/EnergyPower.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Pressure.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Angle.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/GraphicsLength.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Motion.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/Temperature.kt`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/measured/src/main/kotlin/io/bluetape4k/measured/TypeAliases.kt`

Source tests were present for every family plus `MeasureTest.kt`.

## Design Constraints

- Keep a first-party implementation. `github.com/docker/go-units` is already present indirectly through Testcontainers/Moby, but it only covers generic size/time helpers and is not a typed measure, compound unit, or affine temperature model.
- Do not directly import `github.com/docker/go-units` from `./measure`; verify with `go list -deps ./measure | rg 'github.com/docker/go-units' && exit 1 || true`.
- Use Go-shaped API: `Unit[D]`, `Measure[D]`, `Registry[D]`, explicit sentinel errors, and no Kotlin-style numeric extension methods.
- Treat `String()` as no-panic debug formatting; error-returning validation belongs to `Format`, `Parse`, `In`, `As`, arithmetic, and registry constructors.
- Keep temperature as affine concrete `Temperature` / `TemperatureDelta` values, not `Measure[Temperature]`.
- Use `GoroutineStressTester` and `AsyncJobTester` from `testing/concurrency` only in tests; do not add production async APIs solely for stress evidence.
- Base-unit conversion is the future persistence boundary: every family must expose a base unit and tests must prove `Measure.In(base)` returns the canonical base amount.

## 검증 증거

- `mcp__codegraph.codegraph_status` on the #34 worktree: 183 indexed files, 1981 nodes, 4232 edges.
- `mcp__codegraph.codegraph_files` confirmed root-level package layout and `testing/concurrency` package availability.
- `mcp__codegraph.codegraph_explore` confirmed current `GoroutineStressTester`, `AsyncJobTester`, `Options`, and `Task` APIs.
- `find .../utils/measured/src/main/kotlin -type f` returned 16 source files.
- `find .../utils/measured/src/test/kotlin -type f` returned 15 test files.
- `go mod why -m github.com/docker/go-units` shows the indirect path is `testcontainers/redis -> testcontainers-go -> moby/api/types/container -> docker/go-units`.
- `rg -n "github.com/docker/go-units|go-units" go.mod go.sum **/*.go` found only `go.mod` and `go.sum`, no direct Go source import.
