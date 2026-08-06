# Issue #276 Geo And Coordinate Scope

Issue: #276
Parent: #7
Date: 2026-06-26

## 결정

#276은 research-only로 닫는다. 0.7.0 research gate에서는 `geo`,
`geohash`, `geocode`, `geoip`, `projection`, GIS 패키지를 추가하지
않는다.

유효한 Go 방향은 의도적으로 분리한다.

- pure coordinate/geohash helper는 구체적인 bluetape-go package가 spatial
  indexing 또는 location value를 필요로 할 때만 재검토한다.
- provider-backed Google/Bing reverse-geocoding은 shared utility facade가
  아니라 application example 또는 provider-specific package에 둔다.
- GeoIP2 support는 database download/licensing과 lookup lifecycle을 소유할
  구체 security, analytics, networking consumer가 있을 때 다룬다.
- projection, shapefile, NetCDF, geometry operation은 domain/GIS 작업이며
  general utility milestone로 끌어오지 않는다.

이 이슈에서 구현 follow-up은 만들지 않는다. 향후 consumer는 package를
추가하기 전에 input data, precision needs, coordinate-system rules,
dependency candidates를 담은 좁은 이슈를 열어야 한다.

## 소스 인벤토리

| Source module | Capability | Go decision |
|---|---|---|
| `bluetape4k-projects/utils/geo/geohash` | WGS84 point, geohash encode/decode, neighbors, bounding-box and circle queries, Vincenty distance helpers | Defer. 잠재적으로 재사용 가능한 유일한 pure slice이지만, 현재 bluetape-go caller가 필요로 하지 않는다. |
| `bluetape4k-projects/utils/geo/geocode` | Google and Bing reverse geocoding over HTTP/Feign/coroutines | Provider-specific application work로 보류한다. credentials, quotas, retry/timeout/error contracts, service-specific mapping이 필요하다. |
| `bluetape4k-projects/utils/geo/geoip2` | MaxMind database readers for city/country/ASN lookup | 구체 networking/security/analytics consumer로 보류한다. 소유 패키지가 database lifecycle, licensing, update, lookup failure behavior를 정의해야 한다. |
| `bluetape4k-projects/utils/science` | WGS84/UTM projection, EPSG handling, shapefile/NetCDF, geometry operations, Exposed persistence | Generic utility scope에서는 기각한다. 작은 Go helper port가 아니라 domain GIS/data 작업이다. |

## Go 생태계 후보

| Candidate | Use if future consumer appears | Current decision |
|---|---|---|
| `github.com/mmcloughlin/geohash` | String and integer geohash encode/decode when only geohash cells are needed | Candidate only. 채택 전 architecture support, maintenance, precision behavior를 검토한다. |
| `github.com/paulmach/orb` | 2D geo and planar/projected geometry values, GeoJSON subpackages, simple spatial types | Candidate only. 실제 패키지가 필요로 한다면 geometry value interoperability용으로 선호한다. |
| `github.com/twpayne/go-geom` | OGC-style geometry types, GeoJSON/WKB/KML and database integration | Candidate only. 현재 repo needs보다 무겁다. |
| `github.com/twpayne/go-geos` | GEOS-backed topology operations | Hard GIS requirement가 생기기 전까지 기각한다. native GEOS headers/libraries가 필요하기 때문이다. |

## 기각

- Kotlin `utils/geo` module을 하나의 Go package로 port하는 것.
- caller-owned use case와 credential/lifecycle contract 없이 Google Maps,
  Bing Maps, MaxMind provider client를 추가하는 것.
- general utilities milestone에서 projection 또는 NetCDF/shapefile support를
  추가하는 것.
- source JVM module에 있다는 이유만으로 coordinate math helper를 추가하는 것.
- SQL milestone이 spatial storage/query requirement를 정의하기 전에 spatial
  SQL integration을 만드는 것.

## 향후 Geo 작업 필수 패턴

향후 이슈는 다음을 명시해야 한다.

1. scope가 pure value/geohash, provider IO, GeoIP database lookup,
   projection, full GIS geometry 중 무엇인지.
2. coordinate ordering과 naming rules(`lat/lon` versus `x/y`).
3. invalid latitude/longitude, empty geometry, out-of-range precision에 대한
   validation behavior.
4. 관련이 있다면 poles, antimeridian, bounding boxes, radius queries의
   boundary cases.
5. dependency, license, native-library, database-file lifecycle decisions.
6. hidden credentials나 global state 없이 caller가 package를 사용할 수 있음을
   증명하는 examples.

## 검증

- 현재 bluetape-go search에서 geo/coordinate helper가 필요한 package는 없다.
- Source inventory는 `utils/geo`가 pure geohash, provider IO, local GeoIP
  database work를 섞고 있음을 확인한다. 이들은 하나의 Go utility facade를
  공유하면 안 된다.
- `utils/science` projection과 GIS function은 first-party helper package보다
  넓으며 향후 domain-specific issue에만 속한다.
