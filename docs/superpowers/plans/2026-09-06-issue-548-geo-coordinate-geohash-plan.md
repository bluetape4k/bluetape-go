# Geo Coordinate and Geohash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 외부 dependency 없이 WGS 84 좌표 검증, 경계 포함 판정, Haversine 거리, canonical lowercase Geohash encode/decode를 제공하는 `geo` 패키지를 추가한다.

**Architecture:** `Point`, `Bounds`, `Cell`을 비공개 필드의 작은 값 타입으로 두고 constructor와 `Validate`가 invariant를 고정한다. 거리와 Geohash는 순수 표준 라이브러리 함수로 분리하며, 오류 precedence와 antimeridian 규칙은 package-local helper를 공유한다. 공개 동작은 table test, known vector, fuzz, example, benchmark와 영어/한국어 문서로 함께 잠근다.

**Tech Stack:** Go 표준 라이브러리(`errors`, `fmt`, `math`, `strings`, `testing`), Go fuzzing/benchmark, Make 기반 저장소 검증. 새 module dependency와 Testcontainers는 사용하지 않는다.

---

## 실행 경계

- **복잡도:** MEDIUM. 새 순수 계산 패키지 하나이지만 공개 값 계약, antimeridian, 부동소수점 경계와 canonical Geohash가 서로 의존한다.
- **작업 순서:** 오류/좌표 → 경계 → 거리 → Geohash cell → example/fuzz/benchmark → locale 문서/릴리스 추적 → 전체 검증 순서로 진행한다.
- **쓰기 범위:** `geo/**`, `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md`만 수정한다. `go.mod`, `go.sum`, 다른 패키지와 release/tag workflow는 수정하지 않는다.
- **필수 실행 skill:** 구현 시작 전에 `$bluetape-workflow`, `$bluetape-go-patterns`, `$bluetape-full-feature`, `$test-driven-development`를 읽고 적용한다. 각 task 완료 뒤 아래 Lore commit 예시로 독립 commit을 만든다.
- **문서 영향:** `geo/README.md`, `geo/README.ko.md`, root README locale pair, `CHANGELOG.md`, `WIP.md`를 같은 branch에서 갱신한다. 순수 계산 흐름은 표와 실행 예제가 더 직접적이므로 diagram은 N/A다.
- **의존성 영향:** 없음. `go mod tidy` 전후 `go.mod`와 `go.sum`이 바뀌면 즉시 되돌리고 원인을 조사한다.

## 파일 책임

| 파일 | 책임 |
|---|---|
| `geo/doc.go` | degree 단위, constructor 순서, 구면 근사와 비-GIS 경계를 package doc으로 고정한다. |
| `geo/errors.go` | 다섯 sentinel과 좌표/Geohash 원문을 노출하지 않는 wrapper helper를 둔다. |
| `geo/point.go` | `Point` 저장, constructor, accessor, latitude→longitude 검증 순서를 구현한다. |
| `geo/bounds.go` | 일반/crossing bounds와 ±180 동일 meridian 포함 규칙을 구현한다. |
| `geo/distance.go` | 평균 지구 반지름 Haversine과 안정적인 longitude delta/clamp를 구현한다. |
| `geo/geohash.go` | precision 1..12의 standard base32 encode/decode와 `Cell` invariant를 구현한다. |
| `geo/point_test.go` | zero value, signed zero, finite/range와 sentinel precedence를 검증한다. |
| `geo/bounds_test.go` | inclusive edge, crossing, pole, degenerate, full-longitude와 ±180 동치를 검증한다. |
| `geo/distance_test.go` | 동일점, 대칭, 도시 간 tolerance, antimeridian, antipodal 안정성을 검증한다. |
| `geo/geohash_test.go` | known vector, midpoint upper-half, invalid input, cell containment과 zero cell을 검증한다. |
| `geo/example_test.go` | 호출자가 복사할 수 있는 compile-checked 사용법을 제공한다. |
| `geo/fuzz_test.go` | 임의 hash decode가 panic하지 않고 성공 시 valid cell만 반환함을 검증한다. |
| `geo/benchmark_test.go` | 다섯 공개 hot path의 allocation 관찰 지점을 제공한다. |
| `geo/README.md`, `geo/README.ko.md` | 동일한 API, degree/GeoJSON 순서, 정밀도/비목표를 두 언어로 설명한다. |
| `README.md`, `README.ko.md` | root package table에 `geo`를 노출한다. |
| `CHANGELOG.md`, `WIP.md` | `[Unreleased]`와 milestone 0.22.0의 #548 상태를 기록한다. |

### Task 1: 오류와 `Point` 값 계약

**Files:**
- Create: `geo/doc.go`
- Create: `geo/errors.go`
- Create: `geo/point.go`
- Test: `geo/point_test.go`

- [ ] **Step 1: `Point`의 유효값, zero value, signed zero와 오류 precedence를 고정하는 실패 테스트 작성**

```go
package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNewPointAndAccessors(t *testing.T) {
	point, err := NewPoint(37.5665, 126.9780)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}
	if point.Latitude() != 37.5665 || point.Longitude() != 126.9780 {
		t.Fatalf("point = (%v, %v)", point.Latitude(), point.Longitude())
	}
	if err := point.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if err := (Point{}).Validate(); err != nil {
		t.Fatalf("zero Point must be valid: %v", err)
	}
}

func TestNewPointAcceptsClosedWGS84RangeAndSignedZero(t *testing.T) {
	for _, input := range []struct {
		name      string
		latitude  float64
		longitude float64
	}{
		{name: "south-west", latitude: -90, longitude: -180},
		{name: "north-east", latitude: 90, longitude: 180},
		{name: "signed-zero", latitude: math.Copysign(0, -1), longitude: math.Copysign(0, -1)},
	} {
		t.Run(input.name, func(t *testing.T) {
			point, err := NewPoint(input.latitude, input.longitude)
			if err != nil {
				t.Fatalf("NewPoint failed: %v", err)
			}
			if point.Latitude() != input.latitude || point.Longitude() != input.longitude {
				t.Fatalf("point = (%v, %v)", point.Latitude(), point.Longitude())
			}
		})
	}
}

func TestNewPointRejectsNonFiniteAndOutOfRangeValuesInFieldOrder(t *testing.T) {
	for _, input := range []struct {
		name      string
		latitude  float64
		longitude float64
		field     string
	}{
		{name: "latitude NaN", latitude: math.NaN(), field: "latitude"},
		{name: "latitude infinity", latitude: math.Inf(1), field: "latitude"},
		{name: "latitude low", latitude: -90.0001, field: "latitude"},
		{name: "latitude high", latitude: 90.0001, field: "latitude"},
		{name: "longitude NaN", longitude: math.NaN(), field: "longitude"},
		{name: "longitude infinity", longitude: math.Inf(-1), field: "longitude"},
		{name: "longitude low", longitude: -180.0001, field: "longitude"},
		{name: "longitude high", longitude: 180.0001, field: "longitude"},
		{name: "latitude wins", latitude: 91, longitude: 181, field: "latitude"},
	} {
		t.Run(input.name, func(t *testing.T) {
			point, err := NewPoint(input.latitude, input.longitude)
			if point != (Point{}) {
				t.Fatalf("failure result = %#v", point)
			}
			if !errors.Is(err, ErrInvalidPoint) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(err.Error(), input.field) {
				t.Fatalf("error %q does not identify %s", err, input.field)
			}
		})
	}
}
```

- [ ] **Step 2: 테스트를 실행해 공개 API가 아직 없어서 실패하는지 확인**

Run: `go test -count=1 ./geo`

Expected: FAIL with `undefined: NewPoint`, `undefined: Point`, or `undefined: ErrInvalidPoint`.

- [ ] **Step 3: package doc, sentinel과 `Point` 최소 구현 작성**

`geo/doc.go`:

```go
// Package geo는 WGS 84 degree 좌표를 검증하고 경계 포함, Haversine 거리,
// canonical lowercase Geohash encode/decode를 제공한다.
//
// NewPoint는 latitude, longitude 순서다. GeoJSON의 좌표 순서인 longitude,
// latitude와 다르며 radian 입력은 지원하지 않는다. DistanceMeters는 평균 지구
// 반지름을 쓰는 구면 근사이므로 측량, 과금 또는 법적 경계 판단 용도가 아니다.
package geo
```

`geo/errors.go`:

```go
package geo

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidPoint는 유한하지 않거나 WGS 84 범위를 벗어난 좌표를 나타낸다.
	ErrInvalidPoint = errors.New("geo: invalid point")
	// ErrInvalidBounds는 유한하지 않거나 범위/남북 순서가 잘못된 경계를 나타낸다.
	ErrInvalidBounds = errors.New("geo: invalid bounds")
	// ErrInvalidCell은 유효한 decode 결과가 아닌 Cell을 나타낸다.
	ErrInvalidCell = errors.New("geo: invalid cell")
	// ErrInvalidPrecision은 지원 범위 밖의 Geohash precision을 나타낸다.
	ErrInvalidPrecision = errors.New("geo: invalid precision")
	// ErrInvalidGeohash는 canonical lowercase Geohash가 아닌 입력을 나타낸다.
	ErrInvalidGeohash = errors.New("geo: invalid geohash")
)

func fieldError(kind error, field string) error {
	return fmt.Errorf("%w: %s", kind, field)
}
```

`geo/point.go`:

```go
package geo

import "math"

// Point는 degree 단위의 유효한 WGS 84 latitude/longitude 좌표다.
type Point struct {
	latitude  float64
	longitude float64
}

// NewPoint는 latitude, longitude 순서로 Point를 생성한다.
func NewPoint(latitude, longitude float64) (Point, error) {
	point := Point{latitude: latitude, longitude: longitude}
	if err := point.Validate(); err != nil {
		return Point{}, err
	}
	return point, nil
}

// Latitude는 degree 단위 latitude를 반환한다.
func (p Point) Latitude() float64 { return p.latitude }

// Longitude는 degree 단위 longitude를 반환한다.
func (p Point) Longitude() float64 { return p.longitude }

// Validate는 Point가 유한하고 WGS 84 범위 안에 있는지 확인한다.
func (p Point) Validate() error {
	if !finiteInRange(p.latitude, -90, 90) {
		return fieldError(ErrInvalidPoint, "latitude")
	}
	if !finiteInRange(p.longitude, -180, 180) {
		return fieldError(ErrInvalidPoint, "longitude")
	}
	return nil
}

func finiteInRange(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}
```

- [ ] **Step 4: `Point` 테스트가 통과하는지 확인**

Run: `gofmt -w geo/doc.go geo/errors.go geo/point.go geo/point_test.go && go test -count=1 ./geo`

Expected: PASS with `ok github.com/bluetape4k/bluetape-go/geo`.

- [ ] **Step 5: Task 1 변경 commit**

```bash
git add geo/doc.go geo/errors.go geo/point.go geo/point_test.go
git commit \
  -m "좌표 값의 유효성 경계를 먼저 고정한다" \
  -m "Constraint: WGS 84 degree 범위와 latitude-first constructor 계약을 보존해야 한다.
Rejected: 입력 clamp 또는 longitude normalize | caller 입력 오류와 원래 표현을 숨긴다.
Confidence: high
Scope-risk: narrow
Directive: 공개 field나 panic constructor를 추가하지 않는다.
Tested: go test -count=1 ./geo
Not-tested: Bounds, 거리와 Geohash는 후속 task에서 검증한다."
```

### Task 2: `Bounds`와 antimeridian 포함 판정

**Files:**
- Create: `geo/bounds.go`
- Test: `geo/bounds_test.go`

- [ ] **Step 1: 일반/crossing/full/degenerate 경계와 ±180 동일 meridian을 고정하는 실패 테스트 작성**

```go
package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestBoundsAccessorsAndValidation(t *testing.T) {
	bounds, err := NewBounds(120, 30, 130, 40)
	if err != nil {
		t.Fatalf("NewBounds failed: %v", err)
	}
	if bounds.West() != 120 || bounds.South() != 30 || bounds.East() != 130 || bounds.North() != 40 {
		t.Fatalf("bounds = %#v", bounds)
	}
	if bounds.CrossesAntimeridian() {
		t.Fatal("ordinary bounds must not cross antimeridian")
	}
	if err := (Bounds{}).Validate(); err != nil {
		t.Fatalf("zero Bounds must be valid: %v", err)
	}
}

func TestBoundsContainsInclusiveAndCrossingCoordinates(t *testing.T) {
	ordinary, _ := NewBounds(120, 30, 130, 40)
	crossing, _ := NewBounds(170, -10, -170, 10)
	full, _ := NewBounds(-180, -90, 180, 90)
	degenerate, _ := NewBounds(0, 0, 0, 0)

	assertContains := func(t *testing.T, bounds Bounds, latitude, longitude float64, want bool) {
		t.Helper()
		point, err := NewPoint(latitude, longitude)
		if err != nil {
			t.Fatalf("NewPoint failed: %v", err)
		}
		if got := bounds.Contains(point); got != want {
			t.Fatalf("Contains(%v, %v) = %v, want %v", latitude, longitude, got, want)
		}
	}

	assertContains(t, ordinary, 30, 120, true)
	assertContains(t, ordinary, 40, 130, true)
	assertContains(t, ordinary, 35, 119.999, false)
	assertContains(t, crossing, 0, 179, true)
	assertContains(t, crossing, 0, -179, true)
	assertContains(t, crossing, 0, 0, false)
	assertContains(t, full, -90, -180, true)
	assertContains(t, full, 90, 180, true)
	assertContains(t, degenerate, 0, 0, true)
	if !crossing.CrossesAntimeridian() {
		t.Fatal("crossing bounds must report antimeridian crossing")
	}
}

func TestBoundsTreatsMinusAndPlus180AsSameMeridian(t *testing.T) {
	westEdge, _ := NewBounds(-180, -10, -170, 10)
	eastEdge, _ := NewBounds(170, -10, 180, 10)
	plus180, _ := NewPoint(0, 180)
	minus180, _ := NewPoint(0, -180)
	if !westEdge.Contains(plus180) || !eastEdge.Contains(minus180) {
		t.Fatal("-180 and 180 must be equivalent for boundary inclusion")
	}
}

func TestNewBoundsRejectsInvalidFieldsInOrder(t *testing.T) {
	for _, input := range []struct {
		name, field            string
		west, south, east, north float64
	}{
		{name: "west NaN", field: "west", west: math.NaN()},
		{name: "west positive infinity", field: "west", west: math.Inf(1)},
		{name: "west negative infinity", field: "west", west: math.Inf(-1)},
		{name: "south NaN", field: "south", west: 0, south: math.NaN()},
		{name: "south positive infinity", field: "south", west: 0, south: math.Inf(1)},
		{name: "south negative infinity", field: "south", west: 0, south: math.Inf(-1)},
		{name: "east NaN", field: "east", west: 0, south: 0, east: math.NaN()},
		{name: "east positive infinity", field: "east", west: 0, south: 0, east: math.Inf(1)},
		{name: "east negative infinity", field: "east", west: 0, south: 0, east: math.Inf(-1)},
		{name: "north NaN", field: "north", west: 0, south: 0, east: 0, north: math.NaN()},
		{name: "north positive infinity", field: "north", west: 0, south: 0, east: 0, north: math.Inf(1)},
		{name: "north negative infinity", field: "north", west: 0, south: 0, east: 0, north: math.Inf(-1)},
		{name: "west below range", field: "west", west: -181},
		{name: "west above range", field: "west", west: 181},
		{name: "south below range", field: "south", west: 0, south: -91},
		{name: "south above range", field: "south", west: 0, south: 91},
		{name: "east below range", field: "east", west: 0, south: 0, east: -181},
		{name: "east above range", field: "east", west: 0, south: 0, east: 181},
		{name: "north below range", field: "north", west: 0, south: 0, east: 0, north: -91},
		{name: "north above range", field: "north", west: 0, south: 0, east: 0, north: 91},
		{name: "west before south", field: "west", west: math.NaN(), south: math.NaN()},
		{name: "south before east", field: "south", west: 0, south: math.NaN(), east: math.NaN()},
		{name: "east before north", field: "east", west: 0, south: 0, east: math.NaN(), north: math.NaN()},
		{name: "west range before south range", field: "west", west: 181, south: 91},
		{name: "south range before east range", field: "south", west: 0, south: 91, east: 181},
		{name: "east range before north range", field: "east", west: 0, south: 0, east: 181, north: 91},
		{name: "ordering", field: "south/north ordering", west: 0, south: 10, east: 0, north: 9},
	} {
		t.Run(input.name, func(t *testing.T) {
			bounds, err := NewBounds(input.west, input.south, input.east, input.north)
			if bounds != (Bounds{}) || !errors.Is(err, ErrInvalidBounds) {
				t.Fatalf("bounds=%#v error=%v", bounds, err)
			}
			if !strings.Contains(err.Error(), input.field) {
				t.Fatalf("error %q does not identify %s", err, input.field)
			}
		})
	}
}

func TestBoundsContainsRejectsPackageInternalInvalidPoint(t *testing.T) {
	bounds, _ := NewBounds(-180, -90, 180, 90)
	if bounds.Contains(Point{latitude: math.NaN()}) {
		t.Fatal("invalid internal Point must not be contained")
	}
}
```

- [ ] **Step 2: 테스트를 실행해 `Bounds` API가 없어서 실패하는지 확인**

Run: `go test -count=1 ./geo`

Expected: FAIL with `undefined: NewBounds` or `undefined: Bounds`.

- [ ] **Step 3: inclusive bounds와 meridian 동치를 구현**

`geo/bounds.go`:

```go
package geo

// Bounds는 [west, south, east, north] 순서의 inclusive degree 경계다.
type Bounds struct {
	west  float64
	south float64
	east  float64
	north float64
}

// NewBounds는 일반 경계 또는 antimeridian을 가로지르는 경계를 생성한다.
func NewBounds(west, south, east, north float64) (Bounds, error) {
	bounds := Bounds{west: west, south: south, east: east, north: north}
	if err := bounds.Validate(); err != nil {
		return Bounds{}, err
	}
	return bounds, nil
}

// West는 degree 단위 서쪽 경계를 반환한다.
func (b Bounds) West() float64 { return b.west }

// South는 degree 단위 남쪽 경계를 반환한다.
func (b Bounds) South() float64 { return b.south }

// East는 degree 단위 동쪽 경계를 반환한다.
func (b Bounds) East() float64 { return b.east }

// North는 degree 단위 북쪽 경계를 반환한다.
func (b Bounds) North() float64 { return b.north }

// Validate는 좌표 범위와 south/north 순서를 확인한다.
func (b Bounds) Validate() error {
	if !finiteInRange(b.west, -180, 180) {
		return fieldError(ErrInvalidBounds, "west")
	}
	if !finiteInRange(b.south, -90, 90) {
		return fieldError(ErrInvalidBounds, "south")
	}
	if !finiteInRange(b.east, -180, 180) {
		return fieldError(ErrInvalidBounds, "east")
	}
	if !finiteInRange(b.north, -90, 90) {
		return fieldError(ErrInvalidBounds, "north")
	}
	if b.south > b.north {
		return fieldError(ErrInvalidBounds, "south/north ordering")
	}
	return nil
}

// Contains는 point가 inclusive 경계 안에 있는지 반환한다.
func (b Bounds) Contains(point Point) bool {
	if b.Validate() != nil || point.Validate() != nil {
		return false
	}
	return b.containsValidPoint(point)
}

// containsValidPoint는 이미 검증된 Bounds와 Point에만 사용한다.
func (b Bounds) containsValidPoint(point Point) bool {
	if point.latitude < b.south || point.latitude > b.north {
		return false
	}
	if b.containsLongitude(point.longitude) {
		return true
	}
	if point.longitude == -180 {
		return b.containsLongitude(180)
	}
	if point.longitude == 180 {
		return b.containsLongitude(-180)
	}
	return false
}

// CrossesAntimeridian는 east가 west보다 작은 crossing 경계인지 반환한다.
func (b Bounds) CrossesAntimeridian() bool {
	return b.Validate() == nil && b.east < b.west
}

func (b Bounds) containsLongitude(longitude float64) bool {
	if b.east < b.west {
		return longitude >= b.west || longitude <= b.east
	}
	return longitude >= b.west && longitude <= b.east
}
```

- [ ] **Step 4: 경계 테스트를 formatter와 함께 통과시키기**

Run: `gofmt -w geo/bounds.go geo/bounds_test.go && go test -count=1 ./geo`

Expected: PASS with all `TestBounds*` cases green.

- [ ] **Step 5: Task 2 변경 commit**

```bash
git add geo/bounds.go geo/bounds_test.go
git commit \
  -m "날짜변경선 경계의 포함 의미를 명확히 한다" \
  -m "Constraint: east < west는 오류가 아니라 antimeridian crossing이어야 한다.
Rejected: 모든 longitude를 한 표현으로 normalize | constructor 입력 표현을 보존하지 못한다.
Confidence: high
Scope-risk: narrow
Directive: Contains는 오류를 추가하지 않는 predicate로 유지한다.
Tested: go test -count=1 ./geo
Not-tested: 거리와 Geohash cell 포함은 후속 task에서 검증한다."
```

### Task 3: 안정적인 Haversine 거리

**Files:**
- Create: `geo/distance.go`
- Test: `geo/distance_test.go`

- [ ] **Step 1: 거리 단위, 대칭성, 도시/antimeridian/antipodal 경계를 고정하는 실패 테스트 작성**

```go
package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestDistanceMetersKnownValuesAndSymmetry(t *testing.T) {
	seoul, _ := NewPoint(37.5665, 126.9780)
	busan, _ := NewPoint(35.1796, 129.0756)
	forward, err := DistanceMeters(seoul, busan)
	if err != nil {
		t.Fatalf("DistanceMeters failed: %v", err)
	}
	reverse, err := DistanceMeters(busan, seoul)
	if err != nil {
		t.Fatalf("DistanceMeters reverse failed: %v", err)
	}
	if math.Abs(forward-325_000) > 2_000 {
		t.Fatalf("Seoul-Busan distance = %v", forward)
	}
	if math.Abs(forward-reverse) > 1e-9 {
		t.Fatalf("distance is not symmetric: %v vs %v", forward, reverse)
	}
}

func TestDistanceMetersHandlesSameMeridianRepresentations(t *testing.T) {
	minus180, _ := NewPoint(0, -180)
	plus180, _ := NewPoint(0, 180)
	distance, err := DistanceMeters(minus180, plus180)
	if err != nil || distance != 0 {
		t.Fatalf("equivalent meridian distance=%v error=%v", distance, err)
	}

	west, _ := NewPoint(0, 179.9)
	east, _ := NewPoint(0, -179.9)
	distance, err = DistanceMeters(west, east)
	if err != nil {
		t.Fatalf("antimeridian distance failed: %v", err)
	}
	if math.Abs(distance-22_239) > 100 {
		t.Fatalf("antimeridian distance = %v", distance)
	}
}

func TestDistanceMetersIsFiniteForAntipodes(t *testing.T) {
	left, _ := NewPoint(0, 0)
	right, _ := NewPoint(0, 180)
	distance, err := DistanceMeters(left, right)
	if err != nil {
		t.Fatalf("DistanceMeters failed: %v", err)
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
		t.Fatalf("distance = %v", distance)
	}
}

func TestDistanceMetersIsFiniteAtAndNearPoles(t *testing.T) {
	for _, coordinates := range [][4]float64{
		{90, 0, 90, 180},
		{-90, 0, 90, 180},
		{89.999999, -179.999999, 89.999999, 179.999999},
		{-89.999999, -45, -89.999999, 135},
	} {
		left, err := NewPoint(coordinates[0], coordinates[1])
		if err != nil { t.Fatal(err) }
		right, err := NewPoint(coordinates[2], coordinates[3])
		if err != nil { t.Fatal(err) }
		distance, err := DistanceMeters(left, right)
		if err != nil || math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
			t.Fatalf("coordinates=%v distance=%v error=%v", coordinates, distance, err)
		}
	}
}

func TestDistanceMetersValidatesLeftBeforeRight(t *testing.T) {
	invalidLeft := Point{latitude: math.NaN()}
	invalidRight := Point{longitude: math.Inf(1)}
	distance, err := DistanceMeters(invalidLeft, invalidRight)
	if distance != 0 || !errors.Is(err, ErrInvalidPoint) {
		t.Fatalf("distance=%v error=%v", distance, err)
	}
}
```

- [ ] **Step 2: 테스트를 실행해 거리 함수가 없어서 실패하는지 확인**

Run: `go test -count=1 ./geo`

Expected: FAIL with `undefined: DistanceMeters`.

- [ ] **Step 3: normalized longitude delta와 `[0,1]` clamp를 포함한 Haversine 구현**

`geo/distance.go`:

```go
package geo

import "math"

const meanEarthRadiusMeters = 6_371_008.8

// DistanceMeters는 평균 지구 반지름 Haversine 구면 근사 거리를 meter로 반환한다.
func DistanceMeters(left, right Point) (float64, error) {
	if err := left.Validate(); err != nil {
		return 0, err
	}
	if err := right.Validate(); err != nil {
		return 0, err
	}
	latitude1 := degreesToRadians(left.latitude)
	latitude2 := degreesToRadians(right.latitude)
	deltaLatitude := latitude2 - latitude1
	deltaLongitude := degreesToRadians(longitudeDelta(left.longitude, right.longitude))

	sinLatitude := math.Sin(deltaLatitude / 2)
	sinLongitude := math.Sin(deltaLongitude / 2)
	a := sinLatitude*sinLatitude + math.Cos(latitude1)*math.Cos(latitude2)*sinLongitude*sinLongitude
	a = math.Max(0, math.Min(1, a))
	return 2 * meanEarthRadiusMeters * math.Asin(math.Sqrt(a)), nil
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func longitudeDelta(left, right float64) float64 {
	delta := math.Mod(right-left+180, 360)
	if delta < 0 {
		delta += 360
	}
	return delta - 180
}
```

- [ ] **Step 4: 거리 테스트를 통과시키고 race detector로 순수 값 경로 확인**

Run: `gofmt -w geo/distance.go geo/distance_test.go && go test -count=1 ./geo && go test -race -count=1 ./geo`

Expected: both commands PASS; distance results are finite and no race is reported.

- [ ] **Step 5: Task 3 변경 commit**

```bash
git add geo/distance.go geo/distance_test.go
git commit \
  -m "구면 거리 계산의 수치 경계를 고정한다" \
  -m "Constraint: 평균 지구 반지름 6371008.8m와 Haversine 근사를 공개 계약으로 사용한다.
Rejected: ellipsoidal geodesic dependency | 이번 작은 순수 패키지 범위를 넓힌다.
Confidence: high
Scope-risk: narrow
Directive: 측량 수준 정확도로 설명하거나 nanosecond 임계값을 release gate로 만들지 않는다.
Tested: go test -count=1 ./geo; go test -race -count=1 ./geo
Not-tested: Geohash encode/decode는 후속 task에서 검증한다."
```

### Task 4: canonical Geohash와 `Cell`

**Files:**
- Create: `geo/geohash.go`
- Test: `geo/geohash_test.go`

- [ ] **Step 1: known vector, precision, midpoint, canonical input과 `Cell` invariant 실패 테스트 작성**

```go
package geo

import (
	"errors"
	"math"
	"testing"
)

func TestEncodeKnownVectorAndPrecision(t *testing.T) {
	point, _ := NewPoint(57.64911, 10.40744)
	for index, want := range []string{
		"u", "u4", "u4p", "u4pr", "u4pru", "u4pruy",
		"u4pruyd", "u4pruydq", "u4pruydqq", "u4pruydqqv",
		"u4pruydqqvj", "u4pruydqqvj8",
	} {
		precision := index + 1
		hash, err := Encode(point, precision)
		if err != nil || hash != want {
			t.Fatalf("precision=%d hash=%q want=%q error=%v", precision, hash, want, err)
		}
		cell, err := Decode(hash)
		if err != nil || !cell.Bounds().Contains(point) {
			t.Fatalf("precision=%d round trip cell=%#v error=%v", precision, cell, err)
		}
	}
}

func TestEncodeSelectsUpperIntervalAtMidpoint(t *testing.T) {
	point, _ := NewPoint(0, 0)
	hash, err := Encode(point, 1)
	if err != nil || hash != "s" {
		t.Fatalf("midpoint hash=%q error=%v", hash, err)
	}
}

func TestDecodeReturnsValidContainingCell(t *testing.T) {
	original, _ := NewPoint(57.64911, 10.40744)
	cell, err := Decode("u4pruydqqvj")
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if err := cell.Validate(); err != nil {
		t.Fatalf("cell Validate failed: %v", err)
	}
	if !cell.Bounds().Contains(original) || !cell.Bounds().Contains(cell.Center()) {
		t.Fatalf("cell does not contain original and center: %#v", cell)
	}
	if cell.Center().Latitude() < cell.Bounds().South() || cell.Center().Latitude() > cell.Bounds().North() {
		t.Fatalf("center latitude outside bounds: %#v", cell)
	}
}

func TestEncodeDecodeContainsWGS84CornersAndAdjacentMidpoints(t *testing.T) {
	for _, coordinates := range [][2]float64{
		{-90, -180}, {-90, 180}, {90, -180}, {90, 180},
		{math.Nextafter(0, -1), math.Nextafter(0, -1)},
		{0, 0},
		{math.Nextafter(0, 1), math.Nextafter(0, 1)},
	} {
		point, err := NewPoint(coordinates[0], coordinates[1])
		if err != nil {
			t.Fatalf("NewPoint(%v) error = %v", coordinates, err)
		}
		for _, precision := range []int{1, 12} {
			first, err := Encode(point, precision)
			if err != nil {
				t.Fatalf("Encode(%v, %d) error = %v", coordinates, precision, err)
			}
			second, err := Encode(point, precision)
			if err != nil || second != first {
				t.Fatalf("Encode repeat = %q, want %q, error=%v", second, first, err)
			}
			cell, err := Decode(first)
			if err != nil || !cell.Bounds().Contains(point) {
				t.Fatalf("round trip point=%v precision=%d cell=%#v error=%v", coordinates, precision, cell, err)
			}
		}
	}
}

func TestDecodeRejectsNonCanonicalInput(t *testing.T) {
	for _, hash := range []string{"", "u4pruydqqvj0x", "U4PRUYDQQVJ", "u4pru y", "u4pruydqqva", "u4pruydqqvi"} {
		cell, err := Decode(hash)
		if cell != (Cell{}) || !errors.Is(err, ErrInvalidGeohash) {
			t.Fatalf("hash=%q cell=%#v error=%v", hash, cell, err)
		}
	}
}

func TestDecodeErrorPrecedenceAndCellValidationOrder(t *testing.T) {
	cell, err := Decode("UPPER-AND-TOO-LONG")
	if cell != (Cell{}) || !errors.Is(err, ErrInvalidGeohash) || !strings.Contains(err.Error(), "length") {
		t.Fatalf("Decode precedence cell=%#v error=%v", cell, err)
	}
	invalid := Cell{precision: 0, center: Point{latitude: math.NaN()}, bounds: Bounds{west: math.NaN()}}
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "precision") {
		t.Fatalf("Cell.Validate precedence error=%v", err)
	}
	invalidCenterAndBounds := Cell{precision: 1, center: Point{latitude: math.NaN()}, bounds: Bounds{west: math.NaN()}}
	if err := invalidCenterAndBounds.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "center") {
		t.Fatalf("Cell.Validate center precedence error=%v", err)
	}
	invalidBounds := Cell{precision: 1, center: Point{}, bounds: Bounds{west: math.NaN()}}
	if err := invalidBounds.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "bounds") {
		t.Fatalf("Cell.Validate bounds error=%v", err)
	}
}

func TestEncodeErrorPrecedenceAndZeroResult(t *testing.T) {
	invalid := Point{latitude: math.NaN()}
	hash, err := Encode(invalid, 0)
	if hash != "" || !errors.Is(err, ErrInvalidPoint) {
		t.Fatalf("hash=%q error=%v", hash, err)
	}
	valid, _ := NewPoint(0, 0)
	for _, precision := range []int{0, 13} {
		hash, err = Encode(valid, precision)
		if hash != "" || !errors.Is(err, ErrInvalidPrecision) {
			t.Fatalf("precision=%d hash=%q error=%v", precision, hash, err)
		}
	}
}

func TestCellZeroValueIsSafeButInvalid(t *testing.T) {
	var cell Cell
	if cell.Center() != (Point{}) || cell.Bounds() != (Bounds{}) {
		t.Fatalf("zero cell accessors changed: %#v", cell)
	}
	if !errors.Is(cell.Validate(), ErrInvalidCell) {
		t.Fatalf("zero cell error = %v", cell.Validate())
	}
}
```

- [ ] **Step 2: 테스트를 실행해 `Cell`, `Encode`, `Decode`가 없어서 실패하는지 확인**

Run: `go test -count=1 ./geo`

Expected: FAIL with `undefined: Encode`, `undefined: Decode`, or `undefined: Cell`.

- [ ] **Step 3: 표준 alphabet과 longitude-first bit 순서를 구현**

`geo/geohash.go`:

```go
package geo

import "strings"

const (
	geohashAlphabet = "0123456789bcdefghjkmnpqrstuvwxyz"
	minimumPrecision = 1
	maximumPrecision = 12
)

var geohashBits = [...]byte{16, 8, 4, 2, 1}

// Cell은 decode된 Geohash의 중심점과 inclusive 경계를 보존한다.
type Cell struct {
	center    Point
	bounds    Bounds
	precision int
}

// Center는 cell 중심점을 반환한다. zero Cell에서는 유효한 zero Point를 반환한다.
func (c Cell) Center() Point { return c.center }

// Bounds는 cell의 inclusive 경계를 반환한다. zero Cell에서는 유효한 zero Bounds를 반환한다.
func (c Cell) Bounds() Bounds { return c.bounds }

// Validate는 Cell이 성공한 decode 결과인지 확인한다.
func (c Cell) Validate() error {
	if c.precision < minimumPrecision || c.precision > maximumPrecision {
		return fieldError(ErrInvalidCell, "precision")
	}
	if c.center.Validate() != nil {
		return fieldError(ErrInvalidCell, "center")
	}
	if c.bounds.Validate() != nil || !c.bounds.containsValidPoint(c.center) {
		return fieldError(ErrInvalidCell, "bounds")
	}
	return nil
}

// Encode는 point를 지정 precision의 canonical lowercase Geohash로 변환한다.
func Encode(point Point, precision int) (string, error) {
	if err := point.Validate(); err != nil {
		return "", err
	}
	if precision < minimumPrecision || precision > maximumPrecision {
		return "", fieldError(ErrInvalidPrecision, "precision")
	}

	longitudeMinimum, longitudeMaximum := -180.0, 180.0
	latitudeMinimum, latitudeMaximum := -90.0, 90.0
	encoded := make([]byte, 0, precision)
	evenBit, bitIndex, character := true, 0, byte(0)
	for len(encoded) < precision {
		if evenBit {
			midpoint := (longitudeMinimum + longitudeMaximum) / 2
			if point.longitude >= midpoint {
				character |= geohashBits[bitIndex]
				longitudeMinimum = midpoint
			} else {
				longitudeMaximum = midpoint
			}
		} else {
			midpoint := (latitudeMinimum + latitudeMaximum) / 2
			if point.latitude >= midpoint {
				character |= geohashBits[bitIndex]
				latitudeMinimum = midpoint
			} else {
				latitudeMaximum = midpoint
			}
		}
		evenBit = !evenBit
		bitIndex++
		if bitIndex == len(geohashBits) {
			encoded = append(encoded, geohashAlphabet[character])
			bitIndex, character = 0, 0
		}
	}
	return string(encoded), nil
}

// Decode는 canonical lowercase Geohash를 중심점과 inclusive 경계를 가진 Cell로 변환한다.
func Decode(hash string) (Cell, error) {
	if len(hash) < minimumPrecision || len(hash) > maximumPrecision {
		return Cell{}, fieldError(ErrInvalidGeohash, "length")
	}
	longitudeMinimum, longitudeMaximum := -180.0, 180.0
	latitudeMinimum, latitudeMaximum := -90.0, 90.0
	evenBit := true
	for index := 0; index < len(hash); index++ {
		value := strings.IndexByte(geohashAlphabet, hash[index])
		if value < 0 {
			return Cell{}, fieldError(ErrInvalidGeohash, "character")
		}
		for _, mask := range geohashBits {
			upper := byte(value)&mask != 0
			if evenBit {
				longitudeMinimum, longitudeMaximum = selectHalf(longitudeMinimum, longitudeMaximum, upper)
			} else {
				latitudeMinimum, latitudeMaximum = selectHalf(latitudeMinimum, latitudeMaximum, upper)
			}
			evenBit = !evenBit
		}
	}

	center := Point{
		latitude:  (latitudeMinimum + latitudeMaximum) / 2,
		longitude: (longitudeMinimum + longitudeMaximum) / 2,
	}
	bounds := Bounds{
		west: longitudeMinimum, south: latitudeMinimum,
		east: longitudeMaximum, north: latitudeMaximum,
	}
	cell := Cell{center: center, bounds: bounds, precision: len(hash)}
	if err := cell.Validate(); err != nil {
		return Cell{}, err
	}
	return cell, nil
}

func selectHalf(minimum, maximum float64, upper bool) (float64, float64) {
	midpoint := (minimum + maximum) / 2
	if upper {
		return midpoint, maximum
	}
	return minimum, midpoint
}
```

- [ ] **Step 4: Geohash와 전체 package 테스트를 통과시키기**

Run: `gofmt -w geo/geohash.go geo/geohash_test.go && go test -count=1 ./geo`

Expected: PASS; `u4pruydqqvj`, midpoint `s`, invalid canonical inputs와 zero `Cell` cases are green.

- [ ] **Step 5: Task 4 변경 commit**

```bash
git add geo/geohash.go geo/geohash_test.go
git commit \
  -m "Geohash의 canonical cell 계약을 고정한다" \
  -m "Constraint: standard base32, longitude-first bit, midpoint upper-half, precision 1..12를 지켜야 한다.
Rejected: uppercase/공백 자동 정규화 | 저장 key의 canonical form을 복수로 만든다.
Confidence: high
Scope-risk: moderate
Directive: neighbor, radius cover, hash accessor를 이번 공개 표면에 추가하지 않는다.
Tested: go test -count=1 ./geo
Not-tested: fuzz, benchmark와 reader 문서는 후속 task에서 검증한다."
```

### Task 5: Example, fuzz와 allocation 관찰

**Files:**
- Create: `geo/example_test.go`
- Create: `geo/fuzz_test.go`
- Create: `geo/benchmark_test.go`

- [ ] **Step 1: compile-checked example 작성**

`geo/example_test.go`:

```go
package geo_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/geo"
)

func Example() {
	if err := printExample(); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// 111195
	// true
	// u4pruydqqvj
	// true
}

func printExample() error {
	origin, err := geo.NewPoint(0, 0)
	if err != nil { return err }
	oneDegreeEast, err := geo.NewPoint(0, 1)
	if err != nil { return err }
	distance, err := geo.DistanceMeters(origin, oneDegreeEast)
	if err != nil { return err }
	crossing, err := geo.NewBounds(170, -10, -170, 10)
	if err != nil { return err }
	nearDateLine, err := geo.NewPoint(0, 179)
	if err != nil { return err }
	point, err := geo.NewPoint(57.64911, 10.40744)
	if err != nil { return err }
	hash, err := geo.Encode(point, 11)
	if err != nil { return err }
	cell, err := geo.Decode(hash)
	if err != nil { return err }

	fmt.Printf("%.0f\n", distance)
	fmt.Println(crossing.Contains(nearDateLine))
	fmt.Println(hash)
	fmt.Println(cell.Bounds().Contains(point))
	return nil
}
```

- [ ] **Step 2: 임의 decode와 성공 invariant fuzz target 작성**

`geo/fuzz_test.go`:

```go
package geo

import (
	"math"
	"testing"
)

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{"u4pruydqqvj", "s", "", "UPPER", "u4pru y"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, hash string) {
		cell, err := Decode(hash)
		if err != nil {
			return
		}
		if err := cell.Validate(); err != nil {
			t.Fatalf("successful Decode returned invalid cell: %v", err)
		}
		if !cell.Bounds().Contains(cell.Center()) {
			t.Fatal("successful Decode returned center outside bounds")
		}
	})
}

func FuzzEncodeDecodeContains(f *testing.F) {
	for _, seed := range []struct {
		latitude, longitude float64
		precision           uint8
	}{
		{0, 0, 1}, {57.64911, 10.40744, 11}, {-90, -180, 12}, {90, 180, 12},
		{math.Nextafter(0, -1), math.Nextafter(0, 1), 12},
	} {
		f.Add(seed.latitude, seed.longitude, seed.precision)
	}
	f.Fuzz(func(t *testing.T, latitude, longitude float64, rawPrecision uint8) {
		point, err := NewPoint(latitude, longitude)
		if err != nil {
			return
		}
		precision := int(rawPrecision%maximumPrecision) + minimumPrecision
		hash, err := Encode(point, precision)
		if err != nil {
			t.Fatalf("Encode(valid point) error = %v", err)
		}
		cell, err := Decode(hash)
		if err != nil || !cell.Bounds().Contains(point) {
			t.Fatalf("round trip hash=%q cell=%#v error=%v", hash, cell, err)
		}
	})
}
```

- [ ] **Step 3: 다섯 공개 hot path benchmark 작성**

`geo/benchmark_test.go`:

```go
package geo

import (
	"fmt"
	"testing"
)

var (
	benchmarkPoint    Point
	benchmarkContains bool
	benchmarkDistance float64
	benchmarkHash     string
	benchmarkCell     Cell
)

func BenchmarkNewPoint(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		point, err := NewPoint(37.5665, 126.9780)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPoint = point
	}
}

func BenchmarkBoundsContains(b *testing.B) {
	fixtures := []struct {
		name                         string
		west, south, east, north    float64
		latitude, longitude         float64
	}{
		{name: "ordinary", west: 120, south: 30, east: 130, north: 40, latitude: 37.5665, longitude: 126.9780},
		{name: "antimeridian", west: 170, south: -10, east: -170, north: 10, latitude: 0, longitude: 180},
	}
	for _, fixture := range fixtures {
		b.Run(fixture.name, func(b *testing.B) {
			bounds, err := NewBounds(fixture.west, fixture.south, fixture.east, fixture.north)
			if err != nil { b.Fatal(err) }
			point, err := NewPoint(fixture.latitude, fixture.longitude)
			if err != nil { b.Fatal(err) }
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkContains = bounds.Contains(point)
			}
		})
	}
}

func BenchmarkDistanceMeters(b *testing.B) {
	left, err := NewPoint(37.5665, 126.9780)
	if err != nil { b.Fatal(err) }
	right, err := NewPoint(35.1796, 129.0756)
	if err != nil { b.Fatal(err) }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		distance, err := DistanceMeters(left, right)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDistance = distance
	}
}

func BenchmarkEncode(b *testing.B) {
	point, err := NewPoint(57.64911, 10.40744)
	if err != nil { b.Fatal(err) }
	for _, precision := range []int{1, 12} {
		b.Run(fmt.Sprintf("precision-%d", precision), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hash, err := Encode(point, precision)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkHash = hash
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	for _, hash := range []string{"u", "u4pruydqqvj8"} {
		b.Run(fmt.Sprintf("precision-%d", len(hash)), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cell, err := Decode(hash)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkCell = cell
			}
		})
	}
}
```

- [ ] **Step 4: example/fuzz seed/benchmark를 각각 실행**

Run:

```bash
gofmt -w geo/example_test.go geo/fuzz_test.go geo/benchmark_test.go
go test -count=3 ./geo
go test -race -count=3 ./geo
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime=10s ./geo
go test -run '^$' -fuzz '^FuzzEncodeDecodeContains$' -fuzztime=10s ./geo
{
  git rev-parse HEAD
  git status --short --untracked-files=all
  go version
  uname -a
  printf 'GOMAXPROCS=%s\n' "${GOMAXPROCS:-default}"
  go test -run '^$' -bench 'Benchmark(NewPoint|BoundsContains|DistanceMeters|Encode|Decode)$' \
    -benchmem -count=3 -benchtime=1s ./geo
} | tee /tmp/issue-548-geo-benchmark.txt
```

Expected: unit/example/race 반복 실행이 PASS하고 두 fuzz target이 panic/invariant failure 없이 끝난다. benchmark는 ordinary/antimeridian `BoundsContains`와 precision 1/12 Geohash sub-benchmark를 포함한다. Fixture constructor는 timer 시작 전에 성공을 확인한다. SHA, 전체 dirty-tree 목록, 환경과 raw output을 `/tmp/issue-548-geo-benchmark.txt`에 수집한 뒤 `apply_patch`로 `WIP.md`의 `## 0.22.0 / #548` 아래 `### Benchmark ledger`에 그대로 옮긴다. Ledger는 `Implementation SHA`, `Working tree`, `Go version`, `OS`, `GOMAXPROCS`, `Command`, `Fixture/order`, `Metric direction`, `Three-run verdict`, `Raw output` 필드를 가진다. 세 번의 반복 모두 `NewPoint`, `BoundsContains`, `DistanceMeters`, `Decode`가 `0 allocs/op`, `Encode`가 `2 allocs/op` 이하여야 PASS다. Allocation은 낮을수록 좋고 threshold 초과는 회귀이며, compiler별 `ns/op`은 방향성 gate가 아닌 관찰값이다. Fuzz failure가 나오면 출력이 가리키는 `testdata/fuzz/FuzzDecode` 또는 `testdata/fuzz/FuzzEncodeDecodeContains` corpus를 삭제하지 않고 대응하는 `go test -count=1 -run '^FuzzDecode$' ./geo` 또는 `go test -count=1 -run '^FuzzEncodeDecodeContains$' ./geo`로 재현한 뒤 corpus 경로를 read-back하고 수정한다.

- [ ] **Step 5: Task 5 변경 commit**

```bash
git add geo/example_test.go geo/fuzz_test.go geo/benchmark_test.go WIP.md
git commit \
  -m "좌표 API의 실행 예와 성능 관찰 지점을 남긴다" \
  -m "Constraint: pure value API의 panic 안전성과 allocation을 재현 가능한 명령으로 관찰해야 한다.
Rejected: nanosecond 고정 threshold | Go version과 실행 환경 변동을 기능 회귀로 오판한다.
Confidence: high
Scope-risk: narrow
Directive: benchmark 수치는 관찰 근거로만 사용하고 release 단독 gate로 승격하지 않는다.
Tested: unit/race 3회, 두 10초 fuzz target, precision 1/12 benchmark 3회 with benchmem
Not-tested: 장시간 fuzz corpus 확장은 CI 후속 검증에 맡긴다."
```

### Task 6: package README와 root discoverability

**Files:**
- Create: `geo/README.md`
- Create: `geo/README.ko.md`
- Modify: `README.md` package table
- Modify: `README.ko.md` package table

- [ ] **Step 1: 영어 package README를 완성**

`geo/README.md`:

````markdown
# geo

[English](README.md) | [한국어](README.ko.md)

`geo` provides dependency-free WGS 84 degree values, inclusive bounds,
Haversine distance, and canonical lowercase Geohash encode/decode.

## Import

```go
import (
    "errors"

    "github.com/bluetape4k/bluetape-go/geo"
)
```

## Usage

```go
func cellFromGeoJSON(position [2]float64) (geo.Cell, error) {
    geoJSONLongitude, geoJSONLatitude := position[0], position[1]
    point, err := geo.NewPoint(geoJSONLatitude, geoJSONLongitude)
    if err != nil {
        return geo.Cell{}, err
    }
    hash, err := geo.Encode(point, 11)
    if err != nil {
        return geo.Cell{}, err
    }
    return geo.Decode(hash)
}
```

`NewPoint` accepts `(latitude, longitude)` in degrees. GeoJSON positions use
`(longitude, latitude)`; name the variables when converting between them.
Radian input, implicit clamp, wrapping, and normalization are not supported.

## Bounds and distance

`NewBounds(west, south, east, north)` creates inclusive bounds. `east < west`
means that the bounds cross the antimeridian. Longitudes `-180` and `180` are
equivalent for containment, while the constructor preserves the input value.

`DistanceMeters` uses the Haversine formula and the mean Earth radius
`6,371,008.8m`. It is a spherical approximation, not a surveying, billing, or
legal-boundary calculation.

## Geohash

`Encode` accepts precision 1 through 12 and returns the standard alphabet
`0123456789bcdefghjkmnpqrstuvwxyz`. `Decode` accepts only canonical lowercase
input and returns a center and inclusive bounds. Check the decode error before
using `Cell`; its zero value is safe to access but invalid.

## Errors

`ErrInvalidPoint`, `ErrInvalidBounds`, `ErrInvalidCell`, `ErrInvalidPrecision`,
and `ErrInvalidGeohash` are stable sentinels. Handle wrapped failures with
`errors.Is`; inspect a returned value only when the error is nil.

```go
func decodeUserHash(hash string) (geo.Cell, error) {
    cell, err := geo.Decode(hash)
    if errors.Is(err, geo.ErrInvalidGeohash) {
        return geo.Cell{}, err
    }
    return cell, err
}
```

## Non-goals

This package does not provide datum conversion, projection, polygons, routing,
tiles, geocoding, spatial SQL, Geohash neighbors, radius covers, or indexes.
````

- [ ] **Step 2: 한국어 package README를 같은 계약으로 완성**

`geo/README.ko.md`:

````markdown
# geo

[English](README.md) | [한국어](README.ko.md)

`geo`는 외부 dependency 없이 WGS 84 degree 값, inclusive bounds, Haversine
거리와 canonical lowercase Geohash encode/decode를 제공합니다.

## 가져오기

```go
import (
    "errors"

    "github.com/bluetape4k/bluetape-go/geo"
)
```

## 사용법

```go
func cellFromGeoJSON(position [2]float64) (geo.Cell, error) {
    geoJSONLongitude, geoJSONLatitude := position[0], position[1]
    point, err := geo.NewPoint(geoJSONLatitude, geoJSONLongitude)
    if err != nil {
        return geo.Cell{}, err
    }
    hash, err := geo.Encode(point, 11)
    if err != nil {
        return geo.Cell{}, err
    }
    return geo.Decode(hash)
}
```

`NewPoint`는 degree 단위 `(latitude, longitude)` 순서입니다. GeoJSON position은
`(longitude, latitude)` 순서이므로 변환 코드에서는 변수 이름을 명시하십시오.
Radian 입력, 암묵적 clamp, wrap, normalize는 지원하지 않습니다.

## 경계와 거리

`NewBounds(west, south, east, north)`는 inclusive bounds를 만듭니다.
`east < west`는 antimeridian을 가로지르는 경계를 뜻합니다. 포함 판정에서는
longitude `-180`과 `180`을 같은 meridian으로 취급하지만 constructor 입력값은
그대로 보존합니다.

`DistanceMeters`는 Haversine 공식과 평균 지구 반지름 `6,371,008.8m`를 쓰는
구면 근사입니다. 측량, 과금 또는 법적 경계 판단 계산이 아닙니다.

## Geohash

`Encode`는 precision 1..12와 표준 alphabet
`0123456789bcdefghjkmnpqrstuvwxyz`를 사용합니다. `Decode`는 canonical lowercase
입력만 받고 중심점과 inclusive bounds를 반환합니다. `Cell` zero value는 accessor가
안전하지만 유효하지 않으므로 항상 decode error를 먼저 확인하십시오.

## 오류

`ErrInvalidPoint`, `ErrInvalidBounds`, `ErrInvalidCell`, `ErrInvalidPrecision`,
`ErrInvalidGeohash`는 안정적인 sentinel입니다. 감싼 오류는 `errors.Is`로 판별하고,
error가 nil일 때만 반환값을 사용합니다.

```go
func decodeUserHash(hash string) (geo.Cell, error) {
    cell, err := geo.Decode(hash)
    if errors.Is(err, geo.ErrInvalidGeohash) {
        return geo.Cell{}, err
    }
    return cell, err
}
```

## 비목표

Datum 변환, projection, polygon, routing, tile, geocoding, spatial SQL,
Geohash neighbor/radius cover/index는 제공하지 않습니다.
````

- [ ] **Step 3: root README locale pair의 `money` 다음에 정확한 package row 삽입**

`README.md`:

```markdown
| [`geo`](geo/README.md) | active | Dependency-free WGS 84 coordinate values, inclusive antimeridian-aware bounds, Haversine distance, and canonical lowercase Geohash encode/decode. |
```

`README.ko.md`:

```markdown
| [`geo`](geo/README.ko.md) | active | WGS 84 좌표 값, inclusive antimeridian-aware bounds, Haversine 거리와 canonical lowercase Geohash encode/decode를 제공하는 dependency-free package. |
```

- [ ] **Step 4: locale link, API 이름과 GeoJSON 순서를 정적 검증**

Run:

```bash
test -f geo/README.md && test -f geo/README.ko.md
rg -n 'NewPoint\(latitude, longitude\)|GeoJSON|6,371,008\.8|precision 1' geo/README.md geo/README.ko.md
rg -n '\[`geo`\]\(geo/README(\.ko)?\.md\)' README.md README.ko.md
go test -count=1 ./geo
git diff --check
```

Expected: both locale files exist, each contract term appears in both package READMEs, each root README has one `geo` row, package tests pass, and `git diff --check` exits 0.

- [ ] **Step 5: Task 6 변경 commit**

```bash
git add geo/README.md geo/README.ko.md README.md README.ko.md
git commit \
  -m "좌표 API의 단위와 사용 경계를 독자에게 고정한다" \
  -m "Constraint: package README와 root locale pair가 동일한 공개 동작을 설명해야 한다.
Rejected: GeoJSON 순서를 암묵적으로 가정하는 짧은 예제 | latitude-first API 오용을 부른다.
Confidence: high
Scope-risk: narrow
Directive: degree, 구면 근사와 canonical lowercase 제한을 locale pair에서 함께 유지한다.
Tested: locale link/API term scan; go test -count=1 ./geo; git diff --check
Not-tested: 외부 문서 site rendering은 이 저장소 범위에 없다."
```

### Task 7: 변경 기록과 milestone 상태

**Files:**
- Modify: `CHANGELOG.md` under `## [Unreleased]`
- Modify: `WIP.md` current target release and status

- [ ] **Step 1: `[Unreleased]`에 한국어 독자 대상 추가 항목 기록**

`CHANGELOG.md`의 `## [Unreleased]` 바로 아래에 다음 내용을 넣는다. 이미 `### 추가`가 생겼다면 heading을 중복하지 않고 bullet만 같은 section에 합친다.

```markdown
### 추가

- `geo`에 WGS 84 degree 좌표 값, antimeridian-aware inclusive bounds,
  평균 지구 반지름 Haversine 거리와 precision 1..12 canonical lowercase
  Geohash encode/decode를 추가한다.
```

- [ ] **Step 2: `WIP.md`를 milestone 0.22.0 진행 문서로 갱신**

기존 `v0.21.0` release 완료 근거는 이력으로 보존하고 현재 대상/상태에 다음 #548 항목을 포함한다.

```markdown
## 현재 대상 릴리스

`v0.22.0`은 좌표/Geohash와 graph backend conformance foundation을 묶는
릴리스입니다. Issue #548은 외부 dependency 없는 `geo` package delivery를
담당하며 tag와 publication은 milestone open issue가 0이 된 뒤 별도 gate에서 진행합니다.

## 현재 상태

- #548은 `geo`의 Point, Bounds, Haversine distance, canonical lowercase
  Geohash API와 영어/한국어 package 문서를 구현하고 있습니다.
- #548의 완료 gate는 `go test`, race, benchmark 관찰, formatter, tidy, vet,
  lint와 전체 직렬 test입니다. Testcontainers와 diagram은 이 순수 계산 package에 N/A입니다.
- `v0.22.0` release preparation, tag와 GitHub Release는 아직 실행하지 않았습니다.
```

- [ ] **Step 3: release와 feature gate가 섞이지 않았는지 확인**

Run:

```bash
rg -n '^## \[Unreleased\]|^### 추가|`geo`|#548|v0\.22\.0|tag|publication' CHANGELOG.md WIP.md
git diff --check
```

Expected: `geo`/`#548`가 현재 feature 상태로 보이고 tag/publication은 미실행 별도 gate로 설명되며 whitespace error가 없다.

- [ ] **Step 4: package와 문서 diff를 함께 검토**

Run: `git diff --stat && git diff -- geo README.md README.ko.md CHANGELOG.md WIP.md`

Expected: 쓰기 범위 밖 파일이 없고 새 dependency, release tag 또는 workflow 변경이 없다.

- [ ] **Step 5: Task 7 변경 commit**

```bash
git add CHANGELOG.md WIP.md
git commit \
  -m "0.22.0 좌표 기능의 배포 경계를 기록한다" \
  -m "Constraint: feature 완료와 milestone release side effect를 분리해야 한다.
Rejected: #548 완료 시 즉시 tag 생성 | 다른 0.22.0 issue와 release gate를 건너뛴다.
Confidence: high
Scope-risk: narrow
Directive: tag와 publication은 milestone open issue 0 및 별도 승인 뒤에만 진행한다.
Tested: CHANGELOG/WIP scope scan; git diff --check
Not-tested: release workflow는 이번 feature branch에서 실행하지 않는다."
```

### Task 8: 최종 검증, 위험 점검과 실행 인계

**Files:**
- Verify only: `geo/**`, `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md`

- [ ] **Step 1: 공개 API와 sentinel을 compile surface에서 확인**

Run:

```bash
go doc ./geo
go test -count=3 ./geo
go test -race -count=3 ./geo
```

Expected: `Point`, `Bounds`, `Cell`, `NewPoint`, `NewBounds`, `DistanceMeters`, `Encode`, `Decode`와 다섯 sentinel이 `go doc`에 보이고 두 test command가 PASS한다.

- [ ] **Step 2: fuzz와 benchmark를 재실행**

Run:

```bash
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime=10s ./geo
go test -run '^$' -fuzz '^FuzzEncodeDecodeContains$' -fuzztime=10s ./geo
{
  git rev-parse HEAD
  git status --short --untracked-files=all
  go version
  uname -a
  printf 'GOMAXPROCS=%s\n' "${GOMAXPROCS:-default}"
  go test -run '^$' -bench 'Benchmark(NewPoint|BoundsContains|DistanceMeters|Encode|Decode)$' \
    -benchmem -count=3 -benchtime=1s ./geo
} | tee /tmp/issue-548-geo-benchmark.txt
```

Expected: fuzz panic/invariant failure 0, ordinary/antimeridian `BoundsContains`, precision 1/12 Geohash benchmark와 환경/SHA/dirty-tree raw output을 `/tmp`에 수집한다. `apply_patch`로 `WIP.md`의 `## 0.22.0 / #548` → `### Benchmark ledger`를 최종 raw output, fixture/order, 낮을수록 좋은 allocation 방향, 세 반복 모두 threshold 이하여야 한다는 판정과 함께 갱신하고 다음 evidence commit을 만든다. `NewPoint`, `BoundsContains`, `DistanceMeters`, `Decode`는 `0 allocs/op`, `Encode`는 `2 allocs/op` 이하이며 `ns/op` 변동만으로 실패 처리하지 않는다. Fuzz failure corpus는 출력 경로에 보존하고 `go test -count=1 -run '^FuzzDecode$|^FuzzEncodeDecodeContains$' ./geo`로 회귀 테스트한 뒤 corpus 경로를 read-back한다.

```bash
git add WIP.md
git commit -m "geo benchmark의 최종 재현 근거를 보존한다" \
  -m "Constraint: 임시 파일이 아닌 tracked artifact에 SHA와 raw 결과를 남겨야 한다
Rejected: /tmp 결과만 유지 | 세션 종료 뒤 재현 근거가 사라진다
Confidence: high
Scope-risk: narrow
Directive: allocation 판정과 raw output을 분리하거나 요약만 남기지 않는다
Tested: benchmark 3회; WIP benchmark ledger read-back
Not-tested: exact-head CI와 release gate"
```

- [ ] **Step 3: 저장소 정적/전체 gate를 순서대로 실행**

Run:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make ci
```

Expected: all commands exit 0. `make test`는 저장소 계약대로 `-p 1` 직렬 package scheduling을 사용하며 `make ci`는 canonical local CI gate를 재확인한다. unrelated Testcontainers infrastructure flake가 발생하면 실패 package와 exact error를 보존하고 그 package만 동일 환경에서 한 번 재실행하되, 재실행 성공을 전체 gate 성공으로 바꾸지 않는다.

- [ ] **Step 4: scope, 미완성 표현, terminology와 dependency drift 검증**

Run:

```bash
git diff --check origin/develop...HEAD
git diff --name-only origin/develop...HEAD
git diff --exit-code origin/develop...HEAD -- go.mod go.sum
rg -n 'T[B]D|T[O]DO|implement[[:space:]]+later|fill[[:space:]]+in[[:space:]]+details|Similar[[:space:]]+to[[:space:]]+Task' geo docs/superpowers/plans/2026-09-06-issue-548-geo-coordinate-geohash-plan.md
rg -n 'latitude, longitude|west, south, east, north|canonical lowercase|6,371,008\.8|1\.\.12|1 through 12' geo README.md README.ko.md CHANGELOG.md WIP.md
```

Expected: changed paths are limited to the declared write scope, module files have no diff, 미완성 표현 scan has no match, and public terminology is consistent across code/tests/docs.

- [ ] **Step 5: 실패 시 rollback/rerun 규칙 적용 후 최종 commit 상태 확인**

- Task 단위 실패는 마지막 green commit을 기준으로 해당 task 파일만 수정하고 그 task의 targeted test부터 재실행한다.
- public contract 불일치는 spec을 임의로 바꾸지 않고 구현을 승인된 spec으로 되돌린다.
- `go.mod`/`go.sum` drift는 새 dependency를 채택하지 않고 module diff를 제거한 뒤 `make tidy-check`를 다시 실행한다.
- full-suite infrastructure failure는 로그와 exact command를 남기고 targeted `geo` green과 구분해 `PENDING`으로 보고한다.
- 모든 gate가 green이면 `git ls-files --error-unmatch docs/superpowers/plans/2026-09-06-issue-548-geo-coordinate-geohash-plan.md`, `git status --short`, `git log --oneline origin/develop..HEAD`로 계획 파일이 tracked이고 uncommitted file 0이며 task별 Lore commit이 존재함을 확인한다.

Expected final state: 구현/문서 task가 모두 commit됐고 worktree가 clean이며, `geo` targeted/race/fuzz/benchmark와 formatter/tidy/vet/lint/full test 증거가 수집돼 있다. PR 생성, merge, tag, publication은 이 계획의 stop condition 밖이다.

- [ ] **Step 6: 구현 완료와 delivery gate를 분리해 handoff 증거를 고정**

구현 마지막 commit SHA를 기록하고 `docs/review/2026-09-06-issue-548-implementation-review.md`에 Step 6-R 여섯 관점과 main integration의 `P0=0 P1=0` 결과를 남긴다. PR 생성 뒤에는 다음 exact-head read-back을 실행하고 PR 본문 `## DoD Status`와 Issue #548의 milestone/assignee/linkage를 다시 읽는다.

```bash
head_sha=$(git rev-parse HEAD)
gh pr view --json number,headRefOid,baseRefName,headRefName,mergeable,reviewDecision,statusCheckRollup,body
gh issue view 548 --json number,state,milestone,assignees,labels,body
test "$(gh pr view --json headRefOid --jq .headRefOid)" = "$head_sha"
```

Expected: 구현 종료 시 이 delivery 항목들은 `PENDING`으로 명시되며 PR exact-head CI, Step 6-R, issue metadata와 PR DoD read-back이 모두 확인되기 전에는 merge-ready로 승격하지 않는다. `nightly-tests.yml` smoke는 `geo`의 Testcontainers N/A를 바꾸는 targeted feature gate가 아니라 milestone release gate다. Release 승인 뒤에만 workflow/ref/SHA/job/artifact를 다음 명령으로 고정하고, tag/publication은 계속 별도 authority로 유지한다.

```bash
gh run list --workflow nightly-tests.yml --limit 10 --json databaseId,headSha,status,conclusion,event,createdAt
nightly_run_id=$(gh run list --workflow nightly-tests.yml --branch develop --limit 1 --json databaseId --jq '.[0].databaseId')
test -n "$nightly_run_id"
gh run view "$nightly_run_id" --json headSha,status,conclusion,jobs,artifacts
```

## Spec coverage self-review

| 승인 spec 요구 | 구현 task | 검증 evidence |
| --- | --- | --- |
| `Point`, degree order, finite/range validation, five sentinel precedence | Task 1 | finite/range/다중 오류 table, zero result, `errors.Is`, `go doc` |
| Inclusive `Bounds`, antimeridian, `-180`/`180` 동치, invalid zero value | Task 2 | normal/crossing/full-world/corner/midpoint matrix |
| WGS 84 mean-radius Haversine, identical/antipodal/known-city cases | Task 3 | exact zero, tolerance table, symmetry와 finite result |
| Canonical lowercase Geohash precision 1..12, `Cell` center/bounds, decode precedence | Task 4 | official prefix 1..12, corner/midpoint, malformed length/character matrix |
| Panic safety와 hot-path allocation 관찰 | Task 5 | example, 두 fuzz target, precision 1/12 benchmark와 raw metadata |
| Package/root English·Korean 문서와 coordinate order/error examples | Task 6 | complete locale snippets, link/terminology read-back |
| 0.22.0 change tracking과 release boundary | Task 7 | Korean CHANGELOG/WIP, issue delivery와 tag/publication 분리 |
| Canonical local gate, dependency/scope audit, Step 6-R와 exact-head handoff | Task 8 | fmt/tidy/vet/lint/test/ci, diff scan, review/PR evidence |

Self-review 결과: 승인 spec의 목표, 비목표, 공개 API, 값·거리·Geohash·오류 계약, 실패 모드, 문서, 수용 기준과 DoD가 모두 순서상 선행 artifact에만 의존하는 task로 연결된다. 새 dependency, neighbor/cover query, altitude, projection, storage/index, route/HTTP API, release publication은 계획 범위에 없다.

## 실행 선택 gate

Plan complete and saved to `docs/superpowers/plans/2026-09-06-issue-548-geo-coordinate-geohash-plan.md`. Two execution options:

1. **Subagent-Driven (recommended)** - `superpowers:subagent-driven-development`로 task마다 fresh subagent를 배정하고 각 task 사이에 spec/code review를 수행한다.
2. **Inline Execution** - `superpowers:executing-plans`로 이 session에서 task를 batch 실행하고 checkpoint마다 검토한다.

구현 방식이 선택되기 전에는 implementation file, PR, merge, tag 또는 publication을 변경하지 않는다.
