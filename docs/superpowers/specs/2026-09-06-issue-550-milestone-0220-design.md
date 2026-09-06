# 0.22.0 공간·지오코딩·그래프 확장 설계

## 목적

Milestone `0.22.0`의 열린 구현 이슈 `#551`, `#552`, `#554`, `#547`, `#561`을
하나의 통합 PR로 제공한다. 통합 PR은 한 번의 squash merge로 전달하되, 각
백엔드와 provider는 독립된 작은 Go 패키지와 독립된 테스트·문서 경계를
유지한다. 이미 완료된 `#548`(geo 값)과 `#555`(그래프 적합성 harness)는
재구현하지 않고 계약과 fixture의 기반으로 사용한다.

## 현재 근거와 제약

- 기본 브랜치 `develop`의 기준 commit은 `4b0322e`이며 baseline `go test ./...`
  가 통과한다.
- `geo`는 WGS84 degree 값과 Geohash만 제공하고 spatial SQL·geocoding은
  non-goal로 선언되어 있다. 따라서 provider 코드를 `geo`에 넣지 않는다.
- `graph`는 model-only이고, `graph/graphtest`가 backend conformance test
  지원을 소유한다. repository/session/query 추상화를 추가하지 않는다.
- `sqlkit`은 `database/sql`과 PostgreSQL-first inspectable builder 경계다.
  다중 dialect builder를 core에 추가하지 않고 engine별 helper를 분리한다.
- 기존 `pgx`, MySQL driver, Redis client, Neo4j driver, Testcontainers 모듈을
  우선 재사용한다. 새 직접 의존성은 공식 driver가 필요한 FalkorDB와
  Gremlin-Go로 한정하고, 버전과 module graph를 `go mod tidy`로 검증한다.
- PostGIS는 같은 SRID에서 geometry 거리 단위를 사용하고 `ST_DWithin`의
  bounding-box 인덱스 최적화를 활용한다. MySQL spatial index는 `NOT NULL`
  열을 요구하며 SRID 제약이 optimizer에 영향을 주므로 MariaDB와 공통 SQL로
  뭉뚱그리지 않는다.
- Nominatim reverse API는 WGS84 `lat`/`lon`, `/reverse`, `jsonv2`,
  `accept-language`, `zoom` 계약을 사용한다. 공개 서비스 정책의 1 req/s,
  식별 가능한 User-Agent/Referer, attribution, caching 및 bulk 제한은
  caller가 소유해야 하며 기본 endpoint를 설치하지 않는다.
- 공식 FalkorDB client는 package-global `context.Background()`와 고수준
  transaction/context 부재가 있으므로 caller-owned Redis client의
  context-aware 명령 경계를 우선한다. Gremlin-Go v3 역시 많은 remote
  API가 `context.Context`를 받지 않으므로 결과 채널 선택과 연결 수명만
  보장하고 서버 traversal 취소를 과장하지 않는다.

## 범위와 비범위

### 포함

| 이슈 | 제공 범위 | 기본 검증 |
|---|---|---|
| `#551` | PostGIS 값, SRID 보존, spatial DDL, point round-trip, bbox/거리 helper | PostGIS Testcontainers |
| `#552` | MySQL 및 MariaDB별 GIS 값/DDL/point-distance helper | 기존 MySQL·MariaDB Testcontainers |
| `#554` | `Reverse` provider 계약, 오류 taxonomy, Nominatim 호환 HTTP adapter | `httptest` fake/local server |
| `#547` | 좁은 OpenCypher 실행/result mapping 및 FalkorDB fixture | fake response + opt-in FalkorDB container |
| `#561` | Gremlin-Go remote adapter, 지원 traversal subset, local TinkerPop fixture | fake/channel + local Gremlin server |

### 제외

- 광범위한 GIS topology, projection/datum 변환, shapefile/NetCDF, native GEOS,
  ORM/OGM, graph repository/session/query DSL
- AGE, Neptune/cloud credential, TinkerPop embedded/local graph abstraction,
  provider 간 공통 query language 또는 transaction facade
- benchmark parity 주장과 대량 geocoding client
- 기본 public Nominatim endpoint, 암묵적 retry/rate-limit/cache, 전역 logger,
  caller credential/HTTP client 생성

## 대안 비교와 결정

### 대안 A — `geo`, `graph`, `sqlkit` core에 공통 추상화 추가

새 `SpatialStore`/`GraphClient`/dialect interface를 core에 넣으면 표면 API가
한 곳에 모이지만, 백엔드별 SRID·거리·취소·transaction capability를
최소공통분모로 왜곡한다. 아직 실제 call site가 없고 dependency 방향과
호환성 부담이 커서 거부한다.

### 대안 B — 각 backend에 완전 중복된 값·fixture·오류 구현

패키지 경계는 선명하지만 WKB/좌표 검증, Testcontainers readiness, 오류
redaction이 복제되어 수정 누락 가능성이 높다. 필요한 작은 내부 helper를
공유할 수 없게 만들어 거부한다.

### 대안 C — 독립 public package + 제한된 내부 재사용 (채택)

`sqlkit/postgis`, `sqlkit/mysqlgis`, `sqlkit/mariadbgis`, `geocoding`,
`graph/falkordb`, `graph/gremlin`을 각각 소유하고, 값 encoding·fake·fixture는
해당 package 내부 또는 명시적 test-support package로 둔다. 공통 계약은
`database/sql`, `geo.Point`, `graph` model, `context.Context` 같은 이미
검증된 표준 경계만 사용한다. 이 방식은 한 PR로 통합해도 각 issue slice를
독립적으로 review/test할 수 있다.

## API 설계

### 공간 값과 SQL helper

각 spatial package는 해당 엔진의 SRID-aware `Point` 값과 `driver.Valuer`/
`sql.Scanner` 구현을 제공한다. 저장 encoding은 binary WKB/EWKB 또는 엔진이
검증 가능한 WKT 중 하나로 고정하고, SRID와 좌표 순서를 round-trip fixture로
고정한다. 값 타입은 `Valid`와 `SRID`를 분리해 SQL `NULL`과 `(0,0)`을 혼동하지
않으며, Scan 실패 시 이전 값을 지우고 partial state를 공개하지 않는다.

공개 helper는 다음 기능으로 제한한다.

- 식별자 검증을 거친 `CreateSpatialTable`/`AddSpatialIndex` 수준의 명시적
  DDL 조각 생성
- point insert/read를 위한 `Value`와 `Scan`
- caller가 제공한 table/column/predicate를 검사한 bbox 또는 distance SQL
  생성; 값은 bind argument로 유지
- `context.Context`를 받는 database/sql 실행 예제와 README

PostGIS helper는 `CREATE EXTENSION postgis`와 `ST_SetSRID`/`ST_MakePoint`,
`ST_DWithin`을 명시한다. MySQL/MariaDB helper는 package별 함수와 테스트로
`ST_SRID`, `ST_Distance_Sphere` 또는 엔진에서 지원하는 동등 기능의 차이를
기록한다. 공통 이름을 억지로 맞추지 않고 반환 단위와 unsupported capability를
문서화한다. SQL fragment는 caller-owned라도 identifier는 quote/allowlist하고
값은 절대 문자열 보간하지 않는다.

### Reverse geocoding

`geocoding`은 `geo.Point`를 입력으로 받는 작은 인터페이스와 Nominatim adapter를
제공한다.

```go
type Provider interface {
	Reverse(ctx context.Context, point geo.Point, options Options) (Result, error)
}
```

`Options`는 language, zoom, detail, attribution 확인과 caller-owned cache key
재료만 전달한다. `Client` 생성자는 base URL, `*http.Client`, User-Agent/app
identity, request/response body limit, timeout, rate-limit/retry/cache policy를
받으며 기본 public endpoint나 전역 상태를 설치하지 않는다. HTTP body는
항상 닫고 bounded decode를 사용한다.

오류는 `ErrInvalidCoordinate`, `ErrNoResult`, `ErrProvider`, `ErrRateLimited`,
`ErrTimeout`, `ErrParse` 같은 안정된 sentinel/type으로 분류하고 provider
payload·URL·identity를 public error string에 넣지 않는다. 요청 전, 응답 후,
최종 결과 반환 직전에 cancellation을 확인해 caller 취소가 late success를
발행하지 않게 한다. non-cooperative legacy HTTP call을 goroutine으로
detached 처리하지 않는다.

### FalkorDB

`graph/falkordb`는 `redis.UniversalClient`와 명시적 graph name을 caller에게
받는다. 고수준 Falkor client의 background context 한계를 숨기지 않고,
좁은 `GRAPH.QUERY`/read-only query/delete 명령을 `Do(ctx, ...)`로 발행한다.
응답은 bounded RESP shape 검증 후 `graph.Vertex`/`graph.Edge`로 변환하고,
malformed/partial result는 typed error로 중단한다. transaction facade,
ORM/OGM, TinkerPop, implicit retry는 제공하지 않는다.

FalkorDB container는 digest-pinned image, readiness probe, unique graph
namespace, cleanup/close 순서를 가지며 기본 unit CI는 fake response를 사용한다.
Docker fixture는 별도 opt-in 또는 serial integration target으로 실행한다.

### Gremlin-Go

`graph/gremlin`은 stable `github.com/apache/tinkerpop/gremlin-go/v3`의
remote connection/client를 caller-owned endpoint, TLS/auth, timeout과 함께
감싼다. 연결 close 소유자는 생성 caller이고, adapter는 shared client를
개별 operation 취소 때문에 닫지 않는다. v3 API가 request context를
지원하지 않는 경로에서는 preflight `ctx.Err`, result channel `select`,
최종 publish checkpoint로 local cancellation을 보장하되 서버 traversal이
즉시 취소된다고 주장하지 않는다.

지원 subset은 `#555` conformance의 connectivity, fixture CRUD, vertex/edge
read, invalid operation, cancellation boundary, cleanup/close로 고정한다.
serializer, server dialect, unsupported traversal은 README와 typed
`ErrUnsupportedCapability`로 드러낸다. local TinkerPop server fixture는
stable `3.8.1` 이미지와 readiness/cleanup을 사용하며 Neptune은 live opt-in
문서만 제공한다.

## 오류·수명·관찰성 계약

- 모든 외부 client, DB handle, HTTP transport, container는 caller-owned인지
  package-owned인지 생성자와 README에 명시한다.
- context, timeout, retry, rate-limit, cache, credentials, TLS policy는
  caller-owned이며 package는 전역 logger/registry를 만들지 않는다.
- 운영 동작이 있는 adapter는 caller-owned `log/slog` 또는 hook을 주입받아
  lifecycle/transport failure/retry/terminal failure를 low-cardinality,
  redacted field로 관찰한다. pure value helper는 오류를 caller가 관찰할 수
  있으므로 내부 logger를 두지 않는다.
- response body, rows, result channel, transaction, timer, container는 모든
  성공·실패·취소 경로에서 닫는다. 공유 client를 operation cancellation의
  부작용으로 닫지 않는다.
- 오류 wrapping은 `%w`를 사용하고 `errors.Is`/`errors.As`가 sentinel/type을
  유지하도록 한다. provider 원문과 raw payload는 반환 문자열과 로그에서
  제거한다.

## 테스트 전략과 수용 기준

각 package는 table-driven unit test와 compile-checked `Example...`를 제공한다.

### `#551` 및 `#552`

- invalid coordinate/SRID/NULL/zero value와 malformed Scan이 partial state를
  남기지 않는지 확인
- DDL, `NOT NULL`, SRID, spatial index, point round-trip, bbox/distance
  predicate를 실제 DB에서 확인
- 같은 이름의 helper가 두 엔진의 단위를 잘못 공유하지 않는지 engine-specific
  expected SQL과 unsupported test로 확인
- transaction/rows/connection cleanup와 context timeout을 검증

### `#554`

- 성공, no-result, provider status error, invalid coordinate, timeout,
  pre-cancel/in-flight cancel, rate-limit(429), malformed JSON, language/zoom,
  stable cache-key metadata를 `httptest`로 확인
- request count, User-Agent, endpoint path(`/reverse`), body limit, close,
  retry/rate-limit caller policy를 fake recorder로 검증
- cancellation 뒤 late response가 Result를 발행하지 않는지 확인

### `#547`

- fake RESP success/empty/malformed/partial/output-plus-error와 context
  pre-cancel/late-cancel을 확인
- graph name/parameter escaping, bounded result, vertex/edge property copy,
  close/cleanup을 검증
- digest-pinned FalkorDB image의 readiness, create/query/delete를 serial
  Testcontainers test로 확인하거나 환경 미충족 시 명시적 `PENDING`으로 기록

### `#561`

- fake result channel의 success/empty/server error/invalid response와
  pre-cancel/in-flight/late-cancel을 확인
- caller-owned client/connection close, timeout, TLS/auth option, unsupported
  traversal을 검증
- local TinkerPop fixture에서 `#555` subset을 serial conformance로 실행하고
  readiness·cleanup·namespace 충돌을 확인

### 통합 기준

- `gofmt`, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`
- 각 신규 package targeted test, examples, `go test -race`, bounded stress
- Testcontainers/real DB는 `-p 1`로 하나씩 실행하고 flaky retry가 있으면
  lifecycle 원인을 조사한 뒤 재검증
- 마지막에 `make ci`를 exact local head에서 실행하며, CI가 skip한 fixture는
  release proof로 간주하지 않는다.

## 문서·호환성

- 신규 package마다 `README.md`와 `README.ko.md`를 같은 API/비범위/실행
  명령으로 유지한다. exported Go doc은 정확한 identifier 뒤 ASCII 공백과
  한국어 설명을 사용한다.
- root README와 `graph`/`sqlkit` index에는 신규 package를 링크하되 기존
  core의 model-only/non-goal 선언을 약화하지 않는다.
- `WIP.md`는 #548/#555 완료 상태를 반영하고 열린 #547/#551/#552/#554/#561
  진행 상태를 기록한다. `CHANGELOG.md`의 `[Unreleased]`에 `추가`/`변경`/
  `버그 수정` 범주로 0.22.0 후보를 기록한다.
- Go module의 public API는 additive이며 기존 package import와 동작을
  변경하지 않는다. 새 driver 버전은 `go.mod`/`go.sum` diff와 `go list -m all`
  로 검토한다. 실패 시 dependency를 되돌리고 해당 slice를 pending으로
  남길 수 있다.

## 실패 모드와 완화

1. **SRID/거리 단위 혼동** — 엔진별 expected SQL·round-trip·단위 문서를
   고정하고 cross-dialect 공통 helper를 금지한다.
2. **외부 API cancellation 오해** — FalkorDB/Gremlin의 non-context API를
   raw context 경계와 local checkpoint로 감싸며 server-side cancellation을
   약속하지 않는다. pre/in-flight/late 취소 테스트를 필수로 한다.
3. **provider payload·secret 누출** — bounded decoder, redacted typed error,
   caller-owned logger, fake malformed/output-plus-error 테스트를 사용한다.
4. **Testcontainers readiness/flaky cleanup** — immutable digest, unique
   namespace, readiness probe, serial execution, deterministic cleanup을
   fixture contract로 고정한다.
5. **한 PR의 broad matrix 회귀** — package별 source/test/docs 경계를 유지하고
   Step 6-R을 core 영향이 작은 순서로 slice별 수행한다. 한 slice 실패는
   다른 slice의 코드를 임의로 합치지 않고 해당 slice를 repair/reverify한다.

## 롤백과 정리

구현 중 dependency 또는 fixture가 기준을 만족하지 않으면 해당 package와
문서를 feature branch에서 되돌리고 나머지 독립 slice의 범위를 재승인한다.
PR 전에는 exact diff와 patch-id를 보존한다. merge 후에만 `develop` 동기화,
merged tree/remote ref 확인, recovery bundle 보존, feature worktree/branch
정리를 수행한다. 이 설계 자체는 release tag·GitHub Release·public endpoint
dispatch를 포함하지 않는다.

## 설계 DoD

- [ ] 다섯 열린 구현 이슈가 독립 package·테스트·README locale로 매핑된다.
- [ ] API/zero value/error/context/resource ownership과 non-goal이 명시된다.
- [ ] dependency, SRID/거리, Nominatim policy, Falkor/Gremlin limitation이
      primary source와 local evidence에 연결된다.
- [ ] fake-first와 serial Testcontainers acceptance가 각 failure/cancel/
      cleanup risk를 덮는다.
- [ ] 단일 통합 PR은 issue map, slice별 review evidence, `## DoD Status`를
      포함하고 fresh exact-head 승인 전에는 merge하지 않는다.

## 출처와 추적성

설계 판단은 다음 live issue, repository source, 이전 결정 문서와 공식 문서를
기준으로 한다.

- GitHub issues: [#547](https://github.com/bluetape4k/bluetape-go/issues/547),
  [#550](https://github.com/bluetape4k/bluetape-go/issues/550),
  [#551](https://github.com/bluetape4k/bluetape-go/issues/551),
  [#552](https://github.com/bluetape4k/bluetape-go/issues/552),
  [#554](https://github.com/bluetape4k/bluetape-go/issues/554),
  [#561](https://github.com/bluetape4k/bluetape-go/issues/561)
- local source anchors: `geo/README.md`, `graph/README.md`,
  `graph/graphtest/types.go`, `graph/neo4j/conformance_test.go`,
  `sqlkit/README.md`, `sqlkit/session.go`, `testcontainers/*`
- prior decisions: `docs/superpowers/research/2026-06-30-issue-50-graph-backend-adapters.md`,
  `/Users/debop/work/bluetape4k/bluetape4k-wiki/research/2026-07-09-bluetape-go-spatial-graph-leader-research-gate.md`,
  `/Users/debop/work/bluetape4k/bluetape4k-wiki/research/2026-07-09-bluetape-go-reverse-geocoding-provider-decision.md`
- official references: [PostGIS `ST_DWithin`](https://postgis.net/docs/ST_DWithin.html),
  [MySQL spatial indexes](https://dev.mysql.com/doc/refman/8.4/en/creating-spatial-indexes.html),
  [Nominatim reverse API](https://nominatim.org/release-docs/latest/api/Reverse/),
  [Nominatim usage policy](https://operations.osmfoundation.org/policies/nominatim/),
  [FalkorDB Go client](https://github.com/FalkorDB/falkordb-go),
  [Gremlin-Go](https://tinkerpop.apache.org/docs/current/reference/#gremlin-go),
  [TinkerPop downloads](https://tinkerpop.apache.org/download.html)

현재 설계에서 아직 구현으로 증명하지 않은 항목은 FalkorDB와 TinkerPop
이미지의 최종 digest, 실제 serializer/응답 shape, MariaDB 거리 함수의
버전별 차이다. 이 항목은 구현 단계의 RED spike와 serial fixture 검증에서
확정하며, 증명하지 못하면 해당 slice를 `PENDING`으로 남긴다.
