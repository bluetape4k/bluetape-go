# #550 0.22.0 통합 구현 리뷰 (Step 6-R/7-R)

## 검토 범위와 기준

대상은 `#547`, `#551`, `#552`, `#554`, `#561`의 구현 tree와 통합 문서다.
기준 base는 `origin/develop@4b0322e`이고, 구현 slice의 현재 검토 대상 exact
HEAD는 `7053dae`다. Gremlin 중첩 결과 상한 보정은 `94aa3ef`에, 최종
lint·errcheck·staticcheck 보정은 `d956c97`에, TinkerPop channel readiness
보정은 `7053dae`에 반영됐다. 구현은 `sqlkit/postgis`, `sqlkit/mysqlgis`,
`sqlkit/mariadbgis`, `geocoding`, `graph/falkordb`, `graph/gremlin`과 각
local Testcontainers fixture를 독립 경계로 유지한다.

이번 continuation에서는 native 독립 review lane을 실행하지 못했으므로,
independent/model/architecture provenance를 주장하지 않는다. 아래 결과는
현재 main session이 같은 exact diff를 다시 읽고 수행한 inline six-lens 및
integration fallback review다. PR exact-head에서 독립 lane을 사용할 수 있으면
동일 범위를 다시 확인해야 한다.

## 최종 판정 요약

| 계층 | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | PASS | 0 | 0 | 0 | 0 |
| 2 | Stability | PASS | 0 | 0 | 0 | 0 |
| 3 | Security | PASS | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | Developer/API | PASS | 0 | 0 | 0 | 0 |
| 6 | User/Caller | PASS | 0 | 0 | 0 | 0 |
| Main | Integration | PASS | 0 | 0 | 0 | 0 |

## 관점별 근거

### 1. Performance

- PostGIS는 `sqlkit/postgis/query.go:53-98`에서 `ST_DWithin`과 bounding-box
  predicate를 SQL로 고정하고, `Point.MarshalEWKB`는 25-byte 값만 생성한다
  (`sqlkit/postgis/point.go:61-79`).
- MySQL/MariaDB polygon envelope은 고정된 5-point WKB이고, result/row 확장은
  engine SQL의 명시적 column shape 안에 있다 (`sqlkit/mysqlgis/query.go:37-75`,
  `sqlkit/mariadbgis/query.go:37-75`).
- FalkorDB는 `maxRows`와 deterministic sorted parameter serialization을
  적용한다 (`graph/falkordb/client.go:83-113`, `graph/falkordb/query.go:24-58,
  144-199`). Gremlin은 `maxResults`와 channel collection limit을 적용하고,
  반사된 slice/array와 중첩 path 값도 `maxExpansionDepth`로 제한한다
  (`graph/gremlin/client.go:116-175`, `graph/gremlin/result.go`).
- 따라서 무제한 row/result collection, implicit retry, benchmark parity 주장은
  발견하지 못했다. 전체 repository 성능 benchmark는 이번 범위의 gate가 아니다.

### 2. Stability

- 모든 SQL `Point.Scan` 경로는 decode 전에 receiver를 zero value로 만들고,
  성공 후에만 publish한다 (`sqlkit/postgis/point.go:82-118`,
  `sqlkit/mysqlgis/point.go:82-116`).
- Nominatim은 pre-cache/rate-limit/request/body/result 각 checkpoint와 bounded
  body close를 갖는다 (`geocoding/nominatim.go:68-199`).
- Gremlin은 pre-submit, in-flight select, final publish에서 context를 검사하고
  stream을 defer-close한다 (`graph/gremlin/client.go:116-175`). 중첩 결과도
  무제한 반사 확장을 허용하지 않는다 (`graph/gremlin/result.go`). TinkerPop의
  server-side cancellation을 과장하지 않고 local wait/close 경계로 문서화했다.
- TinkerPop fixture는 TCP port와 `Channel started at port 8182.` startup log를
  모두 기다린다 (`testcontainers/tinkerpop/tinkerpop.go:32-43`). port listening
  직후 Gremlin-Go handshake를 시도하던 PR #738 coverage 실패를 이 경계로
  보정했고, 동일 테스트 5회 반복에서 모두 통과했다.
- 실제 PostGIS, MySQL, MariaDB, FalkorDB, TinkerPop fixture suite가 serial
  실행되었고 cleanup/readiness가 각 fixture에 포함된다.

### 3. Security

- SQL table/column identifier는 각 package에서 허용 문자만 통과시키고 quoted
  output을 만든다 (`sqlkit/postgis/query.go:101-131`,
  `sqlkit/mysqlgis/query.go:95-131`). GIS values와 distance/SRID도 finite,
  bounded validation을 통과해야 한다.
- Nominatim은 명시적인 absolute URL과 식별 가능한 bounded User-Agent를
  요구하며 public endpoint를 설치하지 않는다 (`geocoding/nominatim.go:31-61`).
  오류에는 provider URL, payload, credential를 넣지 않는다 (`errors.go`와
  `nominatim.go:128-199`).
- FalkorDB/Gremlin provider errors는 typed/redacted wrapper로 노출되고,
  Gremlin remote endpoint user-info는 거부하며 auth/TLS는 connection
  configuration 경계에 남긴다 (`graph/gremlin/client.go:301-318`).
- cloud credential, bulk geocoding, ORM/OGM와 embedded graph runtime은 실행하지
  않았다.

### 4. Operator/Ops

- 모든 Testcontainers fixture는 image reference에 digest를 포함하고 bounded
  readiness/termination을 사용한다 (`testcontainers/postgis/postgis.go:18-55`,
  `testcontainers/falkordb/falkordb.go:14-55`,
  `testcontainers/tinkerpop/tinkerpop.go:15-59`). MySQL/MariaDB 기본 image도
  digest로 고정했다.
- Nominatim README는 1 req/s, identity, attribution, cache, service switch와
  no-bulk policy를 caller/operator 책임으로 명시한다. FalkorDB README는
  high-level client의 background context 한계를, Gremlin README는 remote-only
  및 non-context server cancellation 한계를 명시한다.
- Docker-backed 명령은 동시에 실행하지 않았으며, 초기 TinkerPop image cache
  부재는 pull 후 fixture를 재실행해 해결했다. registry/pull 실패를 성공으로
  처리하는 skip 경로는 없다.

### 5. Developer/API

- 각 public package에는 Korean Go doc, example, English/Korean README가 있고,
  root/parent index에도 링크를 추가했다. `sqlkit` core와 `graph` model-only
  package에는 broad dialect/repository abstraction을 추가하지 않았다.
- FalkorDB는 caller-owned `redis.UniversalClient`를 닫지 않고 raw
  `Do(ctx, ...)`를 사용한다 (`graph/falkordb/client.go:33-113`). Gremlin은
  borrowed executor/connection과 internally created remote connection을
  구분한다 (`graph/gremlin/client.go:53-103,257-290`).
- 공식 dependency는 FalkorDB Go `v2.1.0`과 Gremlin-Go `v3.8.1`로 제한했고,
  `go mod tidy` 후 module graph가 정리됐다.

### 6. User/Caller

- 좌표는 `X=longitude`, `Y=latitude`로 문서화하고 PostGIS EWKB/SRID, MySQL
  `axis-order=long-lat`, MariaDB binary cast를 각 package README에 설명했다.
- `geocoding.Provider.Reverse`는 `geo.Point`와 제한된 `Result`만 교환하고,
  rate-limit/cache/attribution 정책을 caller-owned interface로 남긴다
  (`geocoding/provider.go:9-35`, `geocoding/options.go:62-71`).
- `graph/falkordb`와 `graph/gremlin`은 지원 범위와 unsupported capability를
  typed error/README로 공개하며, transaction/session/query DSL을 암묵적으로
  약속하지 않는다.

## Main integration review

`graph/graphtest` conformance가 요구하는 connectivity, empty read,
create/read, cancellation, provider error, cleanup과 traversal을 Gremlin
fixture에서 통과했다. FalkorDB는 CRUD/query/delete를 실제 fixture에서
통과했고, 두 SQL family는 실제 engine에서 point round-trip과 distance/bbox
predicate를 통과했다. Nominatim은 local `httptest`에서 request contract,
status/error taxonomy, body bound, cancellation, cache/rate-limit과 race를
통과했다.

통합 방향은 다음과 같이 일관된다.

1. caller가 client, context, credential, timeout, retry/rate policy와 lifecycle을
   소유한다.
2. adapter는 protocol-specific serialization/query와 typed, bounded result만
   소유한다.
3. backend별 fixture와 README locale pair가 같은 계약을 설명한다.
4. one PR/one squash merge를 유지하되 package slice 간 공통 abstraction은
   추가하지 않는다.

현재 diff에서 P0/P1 및 즉시 수정해야 하는 in-scope P2/P3는 발견하지 못했다.
다만 이 문서는 inline fallback이므로 PR exact-head에서 CI, review/thread
read-back, mergeability와 독립 review lane 증거가 추가되기 전에는 merge 승인
상태가 아니다.

## 검증 명령

- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint` (`0 issues.`)
- `go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis`
- `go test -p 1 -count=1 -timeout=10m ./sqlkit/mysqlgis`
- `go test -p 1 -count=1 -timeout=10m ./sqlkit/mariadbgis`
- `go test -race -count=1 ./geocoding`
- `go test -race -count=1 ./graph/falkordb`
- `go test -race -count=1 ./graph/gremlin`
- `make test`
- `make race`
- `make ci`
- `git diff --check`

위 로컬 검증과 다섯 Docker fixture suite는 `7053dae`에서 통과했다. PR #738의
첫 exact-head run `34046283255`는 `graph/gremlin` factory readiness gap으로
실패했고, 두 번째 `b6fd899` run `34047689484`는 coverage까지 통과한 뒤 serial
race가 25분 job 제한을 초과해 취소되었다. 로컬 최신 head 전체 race는 586초에
통과했으며, 재현 가능한 코드 실패가 아닌 cold-cache 실행 예산 문제로 판단해
CI job timeout을 40분으로 조정한다. 조정 head의 exact-head CI,
review/thread read-back, mergeability, merge와 release/tag는 이 review의 후속
gate다. 독립 review lane은 실행되지 않았으므로 그 provenance를 주장하지 않으며,
main session inline six-lens fallback만 기록한다.
