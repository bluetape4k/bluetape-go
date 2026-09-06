# Issue #548 Geo Coordinate와 Geohash 설계

Issue: #548
Date: 2026-09-06
Milestone: 0.22.0
Work type: Type A full feature
Target package: `geo`

## 문제와 현재 근거

bluetape-go에는 좌표의 유효 범위, 두 좌표 사이의 거리, 경계 상자 포함 여부를 한곳에서
일관되게 다루는 작은 값 API가 없다. Issue #548은 광범위한 GIS 계층을 만들지 않고
서비스 코드에서 반복되는 계산과 검증을 결정론적으로 제공하도록 요구한다. Geohash도
외부 의존성 없이 작은 공개 표면으로 제공할 수 있을 때만 같은 범위에 포함한다.

설계 기준은 다음과 같다.

- GeoJSON RFC 7946은 지리 좌표를 longitude, latitude 순서로 서술하지만 이 패키지는
  Go 호출부의 가독성을 위해 constructor 인자를 `latitude, longitude`로 명시한다.
- 좌표 기준은 WGS 84의 latitude/longitude 범위다. 이 패키지는 datum 변환이나 투영을
  수행하지 않는다.
- 거리 계산은 평균 지구 반지름 `6,371,008.8m`를 쓰는 Haversine 구면 근사다.
- Geohash는 표준 base32 alphabet `0123456789bcdefghjkmnpqrstuvwxyz`를 쓰며
  precision은 1..12다.
- 새 외부 dependency는 추가하지 않는다. 검증과 계산은 Go 표준 라이브러리만 사용한다.

관련 근거:

- Issue #548: <https://github.com/bluetape4k/bluetape-go/issues/548>
- RFC 7946: <https://www.rfc-editor.org/rfc/rfc7946.html>
- WGS 84 ellipsoid: <https://epsg.org/ellipsoid_7030/WGS-84.html>
- Elasticsearch geo-point reference:
  <https://www.elastic.co/docs/reference/elasticsearch/mapping-reference/geo-point>

## 목표

- 유한하고 범위 안에 있는 latitude/longitude만 표현하는 `Point`를 제공한다.
- 일반 경계와 antimeridian을 가로지르는 경계를 모두 표현하는 `Bounds`를 제공한다.
- 두 유효 좌표 사이의 근사 거리를 meter 단위로 계산한다.
- precision 1..12의 canonical lowercase geohash encode/decode를 제공한다.
- 모든 실패를 `errors.Is`로 판별 가능한 안정적인 sentinel error로 노출한다.
- 공개 API, 예제, 영어/한국어 README가 같은 정밀도와 비-GIS 경계를 설명하게 한다.

## 비목표

- GeoIP, reverse geocoding, routing, map tile, spatial SQL 또는 provider I/O
- 좌표계 변환, map projection, ellipsoidal geodesic 또는 측량 수준 정확도
- polygon, line string, area, nearest-neighbor, geohash neighbor/radius 검색
- 입력 좌표의 암묵적 clamp, wrap 또는 normalize
- JVM utility API의 기계적 이식

## 채택한 접근

하나의 최상위 `geo` 패키지에 `Point`, `Bounds`, `Cell`과 작은 계산 함수를 둔다.
모든 값 타입은 필드를 비공개로 유지하고 constructor와 accessor로 계약을 고정한다.
거리와 geohash 알고리즘은 표준 라이브러리로 직접 구현한다.

이 구성이 첫 slice에서 package 탐색 비용과 공개 표면을 가장 작게 유지한다. Geohash가
이후 neighbor, prefix index 또는 저장소 연동으로 독립적인 생명주기를 갖게 될 때만
별도 하위 패키지를 검토한다.

## 공개 API 계약

구현 계획은 다음 shape를 compile-check 기준으로 사용한다.

```go
package geo

type Point struct { /* private */ }

func NewPoint(latitude, longitude float64) (Point, error)
func (p Point) Latitude() float64
func (p Point) Longitude() float64
func (p Point) Validate() error

type Bounds struct { /* private */ }

func NewBounds(west, south, east, north float64) (Bounds, error)
func (b Bounds) West() float64
func (b Bounds) South() float64
func (b Bounds) East() float64
func (b Bounds) North() float64
func (b Bounds) Validate() error
func (b Bounds) Contains(point Point) bool
func (b Bounds) CrossesAntimeridian() bool

type Cell struct { /* private */ }

func (c Cell) Center() Point
func (c Cell) Bounds() Bounds
func (c Cell) Validate() error

func DistanceMeters(left, right Point) (float64, error)
func Encode(point Point, precision int) (string, error)
func Decode(hash string) (Cell, error)
```

`MustPoint`와 다른 panic constructor는 첫 공개 표면에 넣지 않는다. 호출자가 입력 실패를
명시적으로 처리하게 하고, 정적 fixture 편의만으로 중복 API를 만들지 않는다.

## 값과 유효성 계약

### `Point`

- latitude는 유한한 `[-90, 90]`, longitude는 유한한 `[-180, 180]` 값이어야 한다.
- NaN, positive/negative infinity와 범위 밖 값은 거절한다.
- `Point{}`는 `(0, 0)`을 나타내는 유효한 zero value다.
- `+0`과 `-0`은 같은 좌표로 취급한다.
- longitude `-180`과 `180`은 서로 다른 입력 표현으로 보존하지만 지리적으로 같은
  meridian이므로 거리 결과는 0이어야 한다.
- 공개 field가 없고 zero value도 유효하므로 현재 외부 caller는 invalid `Point`를 직접
  만들 수 없다. `Validate`, `DistanceMeters`, `Encode`의 point validation은 package 내부
  생성 경로와 향후 decoding 확장에서도 invariant를 잃지 않기 위한 방어 계약이다.

### `Bounds`

- constructor 순서는 `[west, south, east, north]`다.
- west/east는 유한한 `[-180, 180]`, south/north는 유한한 `[-90, 90]`이어야 한다.
- `south > north`는 잘못된 경계다. `east < west`는 오류가 아니라 antimeridian을
  가로지르는 경계다.
- `Bounds{}`는 `[0, 0, 0, 0]`의 유효한 degenerate boundary다.
- `Contains`는 네 경계를 모두 포함한다. 일반 경계는 `west <= lon && lon <= east`,
  antimeridian 경계는 `lon >= west || lon <= east`로 판정한다.
- `-180`과 `180`은 포함 판정에서도 같은 meridian으로 취급한다. 한 표현이 경계에
  포함되면 다른 표현도 포함된다. 예를 들어 `[-180, -170]`은 longitude `180`을,
  `[170, 180]`은 longitude `-180`을 포함한다.
- Package 내부의 유효하지 않은 `Point`가 전달되면 `Contains`는 `false`를 반환한다.
  `Contains`는 predicate로 유지하고 별도 오류를 만들지 않는다.
- longitude 전체 범위를 나타내려면 `west=-180, east=180`을 사용한다.

### `Cell`

- `Decode`가 반환하는 `Cell`은 hash가 나타내는 중심점과 포함 경계를 함께 보존한다.
- `Center`는 항상 `Bounds` 안에 있으며 encode된 원점도 같은 `Bounds` 안에 있다.
- `Cell{}`는 유효하지 않다. 빈 hash를 성공한 decode 결과와 혼동하지 않도록 내부에
  precision을 보존하고 `Validate`가 이를 검사한다.
- `Cell{}`에서 `Center`와 `Bounds`를 호출해도 panic하지 않고 각각 유효한 zero-value
  `Point{}`와 `Bounds{}`를 반환한다. 반환값만으로 decode 성공을 판단하면 안 되며 caller는
  항상 `Decode` error를 먼저 확인한다. `Cell`은 원래 hash를 보존하거나 `Hash`/`Precision`
  accessor를 노출하지 않는다. 내부 precision은 `Validate`용 invariant다.

## 거리 계약

- `DistanceMeters`는 두 입력을 먼저 검증한다.
- 결과는 finite, non-negative meter 값이며 같은 점은 0이다.
- 함수는 대칭이어야 하고 antimeridian 양쪽의 가까운 점을 장거리로 계산하지 않는다.
- Haversine과 평균 지구 반지름을 사용하므로 ellipsoid의 정밀 geodesic 결과와 차이가
  있을 수 있다. API와 README는 측량/과금/법적 경계 판단 용도로 보증하지 않는다.
- 부동소수점 반올림 때문에 검증은 exact equality가 아니라 명시된 tolerance를 사용한다.

## Geohash 계약

- `Encode`는 유효한 `Point`와 precision 1..12를 받아 정확히 해당 길이의 lowercase
  canonical hash를 반환한다.
- `Decode`는 길이 1..12의 canonical lowercase hash만 허용한다. uppercase, 공백,
  padding과 alphabet 밖 문자는 normalize하지 않고 거절한다.
- decode는 cell 중심과 inclusive bounds를 반환한다. encode/decode round trip은 원래
  좌표가 cell 안에 있음을 보장하지만 원래 좌표와 중심점의 exact equality는 보장하지 않는다.
- longitude와 latitude interval은 표준 geohash 순서대로 longitude bit부터 번갈아
  이분한다. 값이 midpoint와 같으면 upper interval을 선택한다.
- neighbor, prefix cover, radius query와 저장소 index 설계는 제공하지 않는다.

## 오류 계약

다음 sentinel을 공개하고 모든 세부 오류는 `%w`로 감싸 `errors.Is`를 지원한다.

- `ErrInvalidPoint`
- `ErrInvalidBounds`
- `ErrInvalidCell`
- `ErrInvalidPrecision`
- `ErrInvalidGeohash`

오류 문자열은 안정적인 종류와 field만 설명한다. 좌표나 hash 전체를 오류에 복제하지 않아
로그 cardinality를 불필요하게 늘리지 않는다. Validation은 panic하거나 부분적으로
normalize한 값을 반환하지 않는다.

함수별 실패 precedence와 zero result는 다음과 같다.

| 함수 | 검증 순서 | 실패 결과 |
|---|---|---|
| `NewPoint` | latitude, longitude | `Point{}`, `ErrInvalidPoint` |
| `NewBounds` | west, south, east, north, south/north ordering | `Bounds{}`, `ErrInvalidBounds` |
| `DistanceMeters` | left, right | `0`, `ErrInvalidPoint` |
| `Encode` | point, precision | `""`, `ErrInvalidPoint` 또는 `ErrInvalidPrecision` |
| `Decode` | length, character/case | `Cell{}`, `ErrInvalidGeohash` |
| `Cell.Validate` | internal precision, center, bounds | `ErrInvalidCell` |

한 호출에서 여러 입력이 잘못돼도 표의 첫 실패 하나만 반환한다. Caller는 error가 nil일 때만
zero result를 정상값으로 해석한다.

## 실패 모드와 대응

1. **NaN/Inf가 비교를 우회한다.** 범위 비교 전에 `math.IsNaN`과 `math.IsInf`로 모든
   좌표를 거절한다.
2. **antimeridian 경계를 빈 경계로 오판한다.** `east < west`를 명시적인 crossing
   상태로 취급하고 OR longitude predicate를 독립 테스트한다.
3. **decode가 비표준 입력을 조용히 정규화한다.** uppercase와 whitespace를 거절해
   저장/캐시 key의 canonical form을 하나로 유지한다.
4. **고위도 또는 거의 antipodal 좌표에서 반올림으로 NaN이 생긴다.** Haversine의
   intermediate 값을 `[0, 1]`로 제한한 뒤 inverse trigonometric function을 적용한다.
5. **거리 결과가 정밀 GIS 계산으로 오인된다.** 함수명, Go doc, README에 meter 단위의
   구면 근사와 비목표를 반복해서 명시한다.
6. **cell 경계 반올림으로 원점 포함 검증이 깨진다.** encode와 decode가 같은 이분 규칙과
   inclusive boundary를 공유하고 known vector/round-trip으로 고정한다.

## 테스트 전략

- `Point`: ±90/±180, zero value, signed zero, NaN, Inf, 범위 밖 입력
- `Bounds`: 일반/crossing/degenerate/full-longitude/pole 경계와 inclusive edge
- `Bounds`: `-180`/`180` 동일 meridian의 양방향 포함
- 거리: 동일점, 대칭성, known city pair tolerance, antimeridian 인접점, antipodal 안정성
- Geohash: `(57.64911, 10.40744)`의 알려진 `u4pruydqqvj` 벡터, precision 1과 12,
  invalid length/character/case, encode/decode cell containment
- 오류: 각 sentinel의 `errors.Is`, 잘못된 값에서 zero result와 non-nil error
- Example: 좌표 생성, 거리, crossing bounds, geohash encode/decode
- Fuzz: `Decode`가 임의 문자열에서 panic하지 않고, 성공 시 valid `Cell`만 반환함을 검증
- Benchmark: `NewPoint`, `Bounds.Contains`, `DistanceMeters`, `Encode`, `Decode`에
  `ReportAllocs`를 적용한다. 순수 predicate/거리의 zero allocation과 encode/decode의
  bounded allocation을 관찰하되 Go compiler version에 민감한 nanosecond threshold는
  release gate로 고정하지 않는다.
- `go test -race`는 mutable shared state가 없는 공개 계약을 확인한다.
- `AsyncJobTester`와 Testcontainers는 goroutine/I/O/container가 없는 순수 계산 패키지라
  N/A다.

구현 후 최소 검증 명령:

```bash
go test -count=1 ./geo
go test -race -count=1 ./geo
go test -run '^$' -bench 'Benchmark(NewPoint|BoundsContains|DistanceMeters|Encode|Decode)$' -benchmem ./geo
make fmt-check
make tidy-check
make vet
make lint
make test
```

## 문서와 패키지 경계

- `geo/README.md`와 `geo/README.ko.md`를 함께 추가한다.
- root `README.md`와 `README.ko.md` package table을 함께 갱신한다.
- 모든 exported identifier에 구체적인 Go doc comment를 작성한다.
- Go doc와 README example은 degree 단위, `NewPoint(latitude, longitude)`, GeoJSON의
  `(longitude, latitude)` 순서 차이를 named variable로 보여준다. Radian 입력은 지원하지 않는다.
- `CHANGELOG.md`의 `[Unreleased]` 아래 독자 대상 항목은 한국어로 기록한다.
- `WIP.md`에 milestone 0.22.0의 #548 진행 상태를 기록한다.
- 작은 순수 계산 API이므로 diagram은 N/A다. 표와 실행 예제가 독자 흐름을 더 직접적으로
  설명한다.

## 호환성과 migration

새 패키지 추가이므로 기존 caller ABI/API를 깨지 않는다. 첫 release 전에 field를 비공개로
유지해 representation을 고정하지 않는다. 이후 constructor 인자 순서, antimeridian 의미,
precision 범위, canonical lowercase 규칙 또는 지구 반지름을 바꾸는 것은 observable
behavior change이므로 별도 issue와 migration note가 필요하다.

이 issue의 DoD는 package delivery까지다. Milestone open issue 0, release-preparation branch,
tag와 publication은 모든 0.22.0 issue가 끝난 뒤 별도 release gate에서 확인한다.

## 거절한 대안

- **`geo/geohash` 하위 패키지 분리:** 첫 slice의 타입과 알고리즘이 작고 함께 사용되므로
  import와 문서 표면만 늘어난다.
- **외부 geohash dependency:** 작은 결정론적 알고리즘에 공급망과 API adapter 비용을
  추가할 근거가 없다.
- **좌표 자동 clamp/normalize:** 잘못된 caller 입력을 숨기고 cache/storage key 의미를
  바꾼다.
- **ellipsoidal geodesic:** 정확도와 구현 복잡도가 issue의 non-GIS 범위를 넘는다.

## 수용 기준

- 위 공개 API가 `geo` package에 구현되고 zero-value/validation 계약을 따른다.
- 일반 경계와 antimeridian crossing 경계의 포함 테스트가 모두 통과한다.
- 거리 결과가 known tolerance, symmetry와 finite/non-negative invariant를 만족한다.
- precision 1..12의 known vector와 round-trip 테스트가 통과하고 noncanonical 입력을
  거절한다.
- 외부 dependency가 추가되지 않고 `go mod tidy` 결과가 clean하다.
- 영어/한국어 package README와 root package table이 같은 공개 경계를 설명한다.
- targeted test, race, 정적 검사와 canonical repository gate가 통과한다.
- Step 6-R 7-Tier review가 `P0=0 P1=0`을 기록하고 exact-head GitHub CI가 성공한다.

## DoD

- `[ ]` 공개 API와 sentinel error 구현
- `[ ]` boundary/distance/geohash unit, fuzz, example test 구현
- `[ ]` `geo/README.md`, `geo/README.ko.md`, root README, CHANGELOG와 WIP 동기화
- `[ ]` `go test -count=1 ./geo` 및 `go test -race -count=1 ./geo` 성공
- `[ ]` `make ci`, exact-head GitHub CI와 release checklist가 요구하는 smoke Nightly scope 성공
- `[ ]` Step 6-R 7-Tier review `P0=0 P1=0`
- `[ ]` Issue #548 metadata와 PR DoD read-back 완료
