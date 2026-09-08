# 0.22.0 공간·지오코딩·그래프 확장 Implementation Plan

> **For agentic workers:** Follow this plan task by task. Each step is an independently verifiable change; keep the issue slice and its tests together.

**Goal:** `#547`, `#551`, `#552`, `#554`, `#561`을 독립된 Go package와 serial integration fixture로 구현해 하나의 reviewable 통합 PR로 전달한다.

**Architecture:** `sqlkit/postgis`, `sqlkit/mysqlgis`, `sqlkit/mariadbgis`, `geocoding`, `graph/falkordb`, `graph/gremlin`을 별도 public boundary로 둔다. 기존 `geo`, `graph`, `sqlkit` core에는 범용 dialect·repository·transaction facade를 추가하지 않고, 각 adapter가 caller-owned client/context/credential/lifecycle을 명시한다. Testcontainers와 `httptest`는 package별 test-support 경계를 갖고 `-p 1`로 직렬 실행한다.

**Tech Stack:** Go `1.26.3`, standard library `database/sql`, `net/http`, `httptest`, 기존 `pgx`·MySQL driver·Redis/Testcontainers helpers, `github.com/FalkorDB/falkordb-go/v2@v2.1.0`, `github.com/apache/tinkerpop/gremlin-go/v3@v3.8.1`, PostGIS, MySQL `8.4`, MariaDB `11.x`, TinkerPop `3.8.1`.

**Approved design:** `docs/superpowers/specs/2026-09-06-issue-550-milestone-0220-design.md` (commit `1e5089b`)

---

## 실행 규칙과 공통 증거

- 모든 task는 먼저 table-driven RED 테스트를 추가하고, 실패를 읽은 뒤 최소
  구현을 넣어 GREEN으로 만든다. public 선언마다 정확한 identifier 뒤 ASCII
  공백의 한국어 Go doc을 작성한다.
- 새 의존성은 `go get` 전에 `go list -m -versions`, upstream LICENSE/API,
  현재 module graph와의 충돌을 기록한다. FalkorDB high-level API의
  `context.Background()` 한계와 Gremlin-Go v3의 non-context 경로는 코드와
  README에서 명시하고, context-aware 경계는 기존 Redis client/result
  channel로 증명한다.
- 외부 IO는 caller-owned client, credential, timeout, retry, rate-limit,
  cache, logger를 받는다. raw provider payload·secret·URL을 public error나
  로그에 넣지 않는다. `%w`, `errors.Is`, `errors.As`를 보존한다.
- Testcontainers/real DB는 동시에 실행하지 않는다. readiness, unique
  namespace, body/row/result bounds, cleanup/close를 각 fixture에서
  확인한다. `make test`와 `make race`의 기존 `-p 1` 규칙을 유지한다.
- 각 task 완료 때 `git diff --check`, 대상 package test, 관련 README locale
  parity를 확인하고, 실패하면 원인을 기록한 뒤 해당 task의 RED부터 다시
  실행한다. 재시도만으로 실패를 지우지 않는다.

## Task 0: 브랜치·문서·의존성 기준선 고정

**Files:**

- Modify: `WIP.md`, `CHANGELOG.md` (구현 task가 완료될 때까지 마지막에 갱신)
- Create: `docs/superpowers/reviews/2026-09-06-issue-550-milestone-0220-plan-review.md`
- Modify: `go.mod`, `go.sum` (의존성 task에서만)

- [ ] **Step 0.1: 현재 기준선과 issue map 기록**
  - 실행: `git status --porcelain=v1 -z`, `git rev-parse HEAD`, `git worktree list --porcelain`, `gh issue list --milestone '0.22.0' --state all`.
  - 기대 증거: worktree `feat/milestone-0.22.0-integration`, base `origin/develop@4b0322e`, 열린 구현 이슈 5개와 epic `#550`, 완료된 `#548/#555/#553`의 재작업 제외.
- [ ] **Step 0.2: 의존성 후보를 검증**
  - 실행: `go list -m -versions github.com/FalkorDB/falkordb-go/v2`, `go list -m -versions github.com/apache/tinkerpop/gremlin-go/v3`, upstream API/license 확인.
  - 기대 증거: FalkorDB `v2.1.0`, Gremlin-Go `v3.8.1`을 채택하거나, API/Go 버전/취소 한계로 보류할 경우 issue slice를 `PENDING`으로 되돌릴 수 있는 근거.
- [ ] **Step 0.2a: 값 encoding과 image authority 고정**
  - 결정: 공간 값은 package 공통 `Point`에 **EWKB point + explicit SRID**를 사용하고, MySQL/MariaDB가 EWKB를 직접 반환하지 않는 경로는 `ST_AsBinary` 결과와 별도 `ST_SRID`를 함께 읽어 같은 값 계약으로 복원한다. WKT fallback이나 implicit SRID는 허용하지 않는다.
  - 실행: PostGIS/FalkorDB/TinkerPop/MySQL/MariaDB image의 digest와 upstream release를 기록하고, mutable tag가 남으면 fixture-local immutable reference로 교체한다.
  - 기대 증거: spec의 encoding/SRID 선택과 실제 driver/image version이 plan과 일치하며, 미확정 serializer는 implementation 전에 RED spike에서 차단된다.
- [ ] **Step 0.3: plan review artifact 작성**
  - 실행: 이 plan과 approved spec을 대조해 6개 관점(성능·안정성·보안·운영·개발자/API·caller) 및 main integration의 findings를 기록한다.
  - 기대 증거: `SPW-01..05`와 `P0=0/P1=0`, 모든 spec DoD가 이후 task/command에 매핑된 review artifact.

## Task 1: PostGIS 값과 helper (`#551`)

**Files:**

- Create: `sqlkit/postgis/doc.go`, `sqlkit/postgis/point.go`, `sqlkit/postgis/ddl.go`, `sqlkit/postgis/query.go`
- Test: `sqlkit/postgis/point_test.go`, `sqlkit/postgis/ddl_test.go`, `sqlkit/postgis/query_test.go`, `sqlkit/postgis/example_test.go`, `sqlkit/postgis/integration_test.go`
- Create: `testcontainers/postgis/postgis.go`, `testcontainers/postgis/postgis_test.go`, `testcontainers/postgis/README.md`, `testcontainers/postgis/README.ko.md`
- Create: `sqlkit/postgis/README.md`, `sqlkit/postgis/README.ko.md`

- [ ] **Step 1.1: 값 계약 RED**
  - 테스트: valid/invalid WKB 또는 WKT, SRID 보존, `Valid=false` SQL NULL, `(0,0)` non-NULL, malformed/truncated input, duplicate Scan, driver-owned byte copy, context/row cleanup을 table-driven으로 작성한다.
  - 실행: `go test ./sqlkit/postgis -run 'Test(Point|Scan|Value)' -count=1`.
  - 기대: 아직 package/API가 없어서 RED이며, 실패 메시지가 원하는 계약을 드러낸다.
- [ ] **Step 1.2: 최소 `Point` 구현 GREEN**
  - 구현: `driver.Valuer`/`sql.Scanner`, finite WGS84 좌표 검증, explicit SRID, bounded encoding/decoding, `%w` 기반 typed errors. Scan 실패 시 이전 상태를 지우고 partial 값을 공개하지 않는다.
  - 실행: 위 targeted test와 `go test -race ./sqlkit/postgis -count=1`.
  - 기대: unit/race GREEN, exported doc lint 대상이 모두 한국어 exact-name 형식.
- [ ] **Step 1.3: DDL/query helper RED→GREEN**
  - 테스트/구현: identifier quote/allowlist, bind parameter, extension setup, spatial column/index, point insert/read, bbox·distance predicate와 `context.Context` 실행 helper를 검증한다. SQL fragment에 값을 보간하지 않는다.
  - 실행: `go test ./sqlkit/postgis -run 'Test(DDL|Query|Example)' -count=1`; `go vet ./sqlkit/postgis`.
- [ ] **Step 1.4: PostGIS fixture와 실제 round-trip**
  - 구현: `testcontainers/postgis`에 digest-pinned PostGIS image, readiness, unique DB/schema, connection details, termination cleanup을 추가한다.
  - 테스트: extension creation, SRID/point round-trip, spatial index, `ST_DWithin`/bbox indexed predicate, timeout/rollback/rows close.
  - 실행(직렬): `go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis`.
  - 기대: Docker가 준비되지 않으면 실패를 보존하고 원인을 진단해 `PENDING` 처리하며 skip을 성공으로 세지 않는다.
- [ ] **Step 1.5: 문서와 예제**
  - 구현: 두 locale README, `sqlkit`/root index link, PostGIS 단위·SRID·non-goal·rollback 명령, compile-checked example을 동기화한다.
  - 실행: terminology audit, `go test ./sqlkit/postgis -run Example -count=1`, `git diff --check`.

## Task 2: MySQL/MariaDB GIS helper (`#552`)

**Files:**

- Create: `sqlkit/mysqlgis/doc.go`, `sqlkit/mysqlgis/point.go`, `sqlkit/mysqlgis/ddl.go`, `sqlkit/mysqlgis/query.go`, `sqlkit/mysqlgis/README.md`, `sqlkit/mysqlgis/README.ko.md`
- Create: `sqlkit/mariadbgis/doc.go`, `sqlkit/mariadbgis/point.go`, `sqlkit/mariadbgis/ddl.go`, `sqlkit/mariadbgis/query.go`, `sqlkit/mariadbgis/README.md`, `sqlkit/mariadbgis/README.ko.md`
- Test: `sqlkit/mysqlgis/*_test.go`, `sqlkit/mariadbgis/*_test.go`
- Modify: `README.md`, `README.ko.md`, `sqlkit/README.md`, `sqlkit/README.ko.md`

- [ ] **Step 2.1: 엔진별 RED 계약**
  - 테스트: MySQL/MariaDB가 각각 요구하는 SRID, `NOT NULL`, spatial index, point round-trip, distance predicate, unsupported function, malformed Scan을 별도 expected SQL로 고정한다.
  - 실행: `go test ./sqlkit/mysqlgis ./sqlkit/mariadbgis -count=1`.
- [ ] **Step 2.2: 값 helper GREEN**
  - 구현: Task 1의 안정된 value 원칙만 재사용하고 public cross-dialect interface는 만들지 않는다. engine-specific errors, unit, capability를 보존한다.
  - 실행: `go test -race ./sqlkit/mysqlgis ./sqlkit/mariadbgis -count=1`; `go vet` 대상 packages.
- [ ] **Step 2.3: 실제 MySQL fixture**
  - 테스트: 기존 `testcontainers/mysql` lifecycle을 재사용해 `SPATIAL INDEX`, `NOT NULL`, SRID column, round-trip, indexed point/distance query와 transaction/cleanup을 확인한다.
  - 실행(직렬): `go test -p 1 -count=1 -timeout=10m ./testcontainers/mysql ./sqlkit/mysqlgis`.
- [ ] **Step 2.4: 실제 MariaDB fixture**
  - 테스트: 기존 `testcontainers/mariadb` lifecycle을 재사용해 MySQL과 다른 function/unit/unsupported behavior를 확인한다.
  - 실행(직렬): `go test -p 1 -count=1 -timeout=10m ./testcontainers/mariadb ./sqlkit/mariadbgis`.
- [ ] **Step 2.5: locale parity와 범위 확인**
  - 구현: engine별 README/예제/단위 문서, root/sqlkit index, broad abstraction을 만들지 않은 이유를 기록한다.
  - 실행: `go test ./sqlkit/mysqlgis ./sqlkit/mariadbgis -run Example -count=1`, terminology audit, `git diff --check`.

## Task 3: Reverse geocoding provider (`#554`)

**Files:**

- Create: `geocoding/doc.go`, `geocoding/provider.go`, `geocoding/nominatim.go`, `geocoding/errors.go`, `geocoding/options.go`
- Test: `geocoding/provider_test.go`, `geocoding/nominatim_test.go`, `geocoding/example_test.go`
- Create: `geocoding/README.md`, `geocoding/README.ko.md`
- Modify: `README.md`, `README.ko.md`, `geo/README.md`, `geo/README.ko.md`

- [ ] **Step 3.1: interface/error RED**
  - 테스트: `Provider.Reverse(ctx, geo.Point, Options)`의 success/no-result/provider error/invalid coordinate/rate-limit/timeout/parse error, `errors.Is/As`, no-call pre-cancel을 fake server로 작성한다.
  - 실행: `go test ./geocoding -count=1`.
- [ ] **Step 3.2: request boundary GREEN**
  - 구현: caller-owned `http.Client`, base URL, User-Agent/app identity, bounded body, `/reverse`, WGS84 `lat/lon`, `format=jsonv2`, `accept-language`, `zoom`, detail/attribution options을 구현한다. 기본 public endpoint·전역 logger·암묵 retry/cache는 두지 않는다.
  - 테스트: request path/query/header, body close, status mapping, malformed/oversized response, provider diagnostics redaction.
- [ ] **Step 3.3: cancellation/rate-limit/cache semantics**
  - 구현/테스트: pre/in-flight/late cancellation checkpoint, caller-supplied retry/rate-limit/cache hooks, stable cache key metadata, no detached goroutine, timeout ownership을 검증한다.
  - 실행: `go test -race ./geocoding -count=1`; bounded stress with exact request counts.
- [ ] **Step 3.4: 문서/운영 책임**
  - 구현: Nominatim policy(1 req/s, identity, attribution, switchable service, no bulk)와 provider error recovery/runbook을 양 locale README에 기록하고 `geo` non-goal link를 유지한다.
  - 실행: `go test ./geocoding -run Example -count=1`, terminology audit, `git diff --check`.

## Task 4: FalkorDB adapter (`#547`)

**Files:**

- Modify: `go.mod`, `go.sum` (official FalkorDB module only after Step 0.2)
- Create: `graph/falkordb/doc.go`, `graph/falkordb/client.go`, `graph/falkordb/query.go`, `graph/falkordb/convert.go`, `graph/falkordb/errors.go`, `graph/falkordb/options.go`
- Test: `graph/falkordb/client_test.go`, `graph/falkordb/query_test.go`, `graph/falkordb/convert_test.go`, `graph/falkordb/example_test.go`, `graph/falkordb/integration_test.go`
- Create: `graph/falkordb/README.md`, `graph/falkordb/README.ko.md`
- Create: `testcontainers/falkordb/falkordb.go`, `testcontainers/falkordb/falkordb_test.go`, `testcontainers/falkordb/README.md`, `testcontainers/falkordb/README.ko.md`
- Modify: `graph/README.md`, `graph/README.ko.md`, `README.md`, `README.ko.md`

- [ ] **Step 4.1: official client/context feasibility RED spike**
  - 테스트/조사: official `falkordb-go/v2` constructor, graph query/result shape, Redis client injection, transaction/cancellation limitation을 compile/test로 고정한다.
  - 실행: `go test ./graph/falkordb -run TestFeasibility -count=1`; 실패 시 raw `redis.UniversalClient.Do(ctx, ...)` boundary로 전환하고 decision log를 남긴다.
- [ ] **Step 4.2: fake response/result mapping RED→GREEN**
  - 구현/테스트: graph name escaping, bound query/params, `GRAPH.QUERY` result header/rows, `graph.Vertex`/`graph.Edge` property copy, empty/malformed/partial/output-plus-error, typed redacted errors를 검증한다.
  - 실행: `go test -race ./graph/falkordb -count=1`; no provider call on pre-cancel and no late publish after checkpoint.
- [ ] **Step 4.3: client lifecycle and logging**
  - 구현: caller-owned Redis/Falkor client, explicit close contract, operation timeout/context, injected low-cardinality logger/hook, no shared client close on operation cancel.
  - 테스트: close idempotence/ownership, context pre/in-flight/late, provider error redaction, bounded result and namespace isolation.
- [ ] **Step 4.4: FalkorDB Testcontainers fixture**
  - 구현: official FalkorDB image digest, readiness, unique graph namespace, create/query/delete, termination cleanup.
  - 실행(직렬): `go test -p 1 -count=1 -timeout=10m ./testcontainers/falkordb ./graph/falkordb`.
  - 기대: Docker image/API mismatch는 수정 원인을 기록하고 skip을 성공으로 처리하지 않는다.
- [ ] **Step 4.5: capability 문서**
  - 구현: high-level client의 background context와 transaction 부재, 지원 OpenCypher subset, non-goal(TinkerPop/ORM/benchmark)을 양 locale README와 graph index에 명시한다.
  - 실행: `go test ./graph/falkordb -run Example -count=1`, terminology audit, `git diff --check`.

## Task 5: Gremlin-Go adapter (`#561`)

**Files:**

- Modify: `go.mod`, `go.sum` (stable `github.com/apache/tinkerpop/gremlin-go/v3@v3.8.1`)
- Create: `graph/gremlin/doc.go`, `graph/gremlin/client.go`, `graph/gremlin/query.go`, `graph/gremlin/convert.go`, `graph/gremlin/errors.go`, `graph/gremlin/options.go`
- Test: `graph/gremlin/client_test.go`, `graph/gremlin/query_test.go`, `graph/gremlin/convert_test.go`, `graph/gremlin/example_test.go`, `graph/gremlin/conformance_test.go`, `graph/gremlin/integration_test.go`
- Create: `graph/gremlin/README.md`, `graph/gremlin/README.ko.md`
- Create: `testcontainers/tinkerpop/tinkerpop.go`, `testcontainers/tinkerpop/tinkerpop_test.go`, `testcontainers/tinkerpop/README.md`, `testcontainers/tinkerpop/README.ko.md`
- Modify: `graph/README.md`, `graph/README.ko.md`, `README.md`, `README.ko.md`

- [ ] **Step 5.1: stable driver/serializer feasibility RED spike**
  - 실행: stable v3.8.1 API compile, remote connection/client lifecycle, result channel/serializer, timeout/TLS/auth option을 확인한다. v4 beta나 embedded TinkerGraph는 채택하지 않는다.
  - 기대: request context 부재와 `ResultSet.Close` semantics를 명시한 decision evidence.
- [ ] **Step 5.2: fake/channel result mapping RED→GREEN**
  - 구현/테스트: caller-owned endpoint/TLS/auth, supported traversal subset, vertex/edge result mapping, empty/server error/invalid response, pre/in-flight/late cancellation checkpoint, close ownership을 검증한다.
  - 실행: `go test -race ./graph/gremlin -count=1`; bounded channel stress와 no late publish assertion.
- [ ] **Step 5.3: `#555` subset conformance**
  - 구현/테스트: `graph/graphtest`의 connectivity, fixture CRUD/read, invalid operation, cancellation boundary, cleanup/close를 Gremlin-specific query로 연결하고 unsupported capability를 typed error로 드러낸다.
  - 실행: `go test ./graph/gremlin -run TestConformance -count=1`; fake-first before Docker.
- [ ] **Step 5.4: local TinkerPop fixture**
  - 구현: stable `tinkerpop/gremlin-server:3.8.1` digest, readiness, WebSocket endpoint, serializer/config, unique namespace, cleanup.
  - 실행(직렬): `go test -p 1 -count=1 -timeout=10m ./testcontainers/tinkerpop ./graph/gremlin`.
  - 기대: fixture startup/serializer mismatch는 PENDING/repair로 기록하며 Neptune/live credential은 실행하지 않는다.
- [ ] **Step 5.5: README/API 운영 계약**
  - 구현: remote-only, non-context server cancellation limitation, supported dialect/traversal subset, caller close/TLS/auth/timeout, Neptune live-opt-in을 양 locale에 기록한다.
  - 실행: `go test ./graph/gremlin -run Example -count=1`, terminology audit, `git diff --check`.

## Task 6: 통합 문서·WIP·CHANGELOG와 issue traceability

**Files:**

- Modify: `README.md`, `README.ko.md`, `geo/README.md`, `geo/README.ko.md`, `graph/README.md`, `graph/README.ko.md`, `sqlkit/README.md`, `sqlkit/README.ko.md`, `WIP.md`, `CHANGELOG.md`
- Create: `docs/lessons/2026-09-06-issue-550-milestone-0220.md`
- Create: `docs/superpowers/reviews/2026-09-06-issue-550-milestone-0220-implementation-review.md`

- [ ] **Step 6.1: locale/index parity**
  - 실행: 각 package README의 import/API/example/non-goal/command/link를 표로 대조하고 root index와 `geo`/`graph`/`sqlkit` 경계가 약화되지 않았는지 확인한다.
- [ ] **Step 6.2: WIP/CHANGELOG 갱신**
  - 구현: `#548/#555` 완료 및 `#547/#551/#552/#554/#561` 구현/검증 상태, `#550` epic close 조건, `[Unreleased]`의 `추가`/`변경`/`버그 수정`을 한국어로 기록한다. stale `#555` 진행 문구를 제거한다.
- [ ] **Step 6.3: implementation review/lesson**
  - 구현: 각 slice의 acceptance→file/test traceability, P0/P1/P2/P3 disposition, provider/fixture surprise와 recurrence guard를 기록한다.
  - 실행: writer `SPW-01..05`, Korean terminology audit, `git diff --check`.

## Task 7: 통합 검증과 Step 5 verifier

- [ ] **Step 7.1: 저비용 정적 검증**
  - 실행 순서: `gofmt -w`(변경 Go 파일만) → `go mod tidy`(dependency diff가 있을 때) → `make fmt-check` → `make tidy-check` → `make vet` → `make lint`.
  - 기대: 모든 결과가 현재 feature head에 묶이고, linter의 exported comment/SQL/error 경고를 수정한다.
- [ ] **Step 7.2: targeted unit/example/race**
  - 실행: `go test -count=1 ./sqlkit/postgis ./sqlkit/mysqlgis ./sqlkit/mariadbgis ./geocoding ./graph/falkordb ./graph/gremlin`; 이후 각 새 package `go test -race -count=1`.
  - 기대: fake-first success/failure/cancel/cleanup와 examples가 모두 GREEN.
- [ ] **Step 7.3: heavyweight serial integration**
  - 실행 순서: PostGIS → MySQL → MariaDB → FalkorDB → TinkerPop. 각 명령 사이에 결과·Docker 상태·resource cleanup을 읽고 다음으로 이동한다.
  - 명령: `go test -p 1 -count=1 -timeout=10m ./testcontainers/postgis ./sqlkit/postgis`; 동일한 형태로 mysql/mariadb/falkordb/tinkerpop 및 adapters.
  - 기대: readiness와 cleanup evidence가 각 fixture에 존재하며, flake는 원인 조사 후 affected RED부터 재실행한다.
- [ ] **Step 7.4: full repository proof**
  - 실행: `make test`, `make race`, `make ci` (각각 독립 실행, Docker-backed target은 동시에 실행하지 않음).
  - 기대: 전체 패키지·race·coverage/lint가 current head에서 통과한다. skip/old-SHA 결과는 성공 근거로 쓰지 않는다.
- [ ] **Step 7.5: Step 5 verifier**
  - 실행: approved spec/plan, diff, tests, docs를 `A-VER-01..07`로 대조한다.
  - 기대: requirement-to-file/test table, planned task disposition, scope/locale/risk/fixture evidence, gaps(PENDING/issue)와 `PASS` verdict.

## Task 8: Step 6-R/7-R review와 lesson commit

- [ ] **Step 8.1: slice별 7-Tier review**
  - 실행: `sqlkit/postgis`, `sqlkit/mysqlgis+mariadbgis`, `geocoding`, `graph/falkordb`, `graph/gremlin` 순으로 성능·안정성·보안·운영·개발자/API·caller 관점과 main integration을 읽는다.
  - 기대: 각 slice의 P0/P1=0, P2/P3는 수정 또는 명시적 후속 이슈. Go pattern, context, resource cleanup, redaction, docs parity를 exact file:line으로 기록한다.
- [ ] **Step 8.2: review repair**
  - 실행: P0/P1 또는 in-scope P2/P3가 있으면 최소 수정 후 해당 targeted/race/integration proof와 affected review lane을 재실행한다.
  - 기대: 마지막 통합 표가 P0=0/P1=0/P2=0/P3=0 또는 명시적 후속 issue를 갖는다. unresolved review thread는 PR 전 0건이다.
- [ ] **Step 8.3: durable lesson commit**
  - 구현: `docs/lessons/2026-09-06-issue-550-milestone-0220.md`에 context, 결정, 결과, 검증, 실패 가정, recurrence guard를 기록한다.
  - 실행: writer `SPW-01..05`, terminology audit, `git diff --check`; Lore trailer 형식으로 lesson 포함 commit.

## Task 9: PR·CI·merge-ready gate

- [ ] **Step 9.1: final diff/authority**
  - 실행: `git status --short`, `git diff --stat`, `git diff --check`, `for issue in 547 550 551 552 554 561; do gh issue view "$issue" --json number,title,state,milestone,labels,assignees; done`.
  - 기대: 변경 파일이 승인 scope에 있고, PR target `bluetape4k/bluetape-go`, base `develop`, head `feat/milestone-0.22.0-integration`이 명확하다.
- [ ] **Step 9.2: guidance refresh**
  - 실행: PR 직전 user/workspace/repo `AGENTS.md`, `bluetape-workflow`, `bluetape-go-patterns`, common gates, PR body template와 linked issue metadata를 다시 읽는다.
  - 기대: CG-12A no-drift evidence와 current exact head.
- [ ] **Step 9.3: push/create/read back**
  - 실행: `git push -u origin feat/milestone-0.22.0-integration`; remote head SHA 확인; Korean PR body를 issue map, tests, risks, `## DoD Status` 마지막 section으로 작성해 `gh pr create --base develop --head feat/milestone-0.22.0-integration`.
  - 기대: PR metadata가 issue milestone/labels/assignee `debop`과 일치하고 live body/read-back이 성공한다.
- [ ] **Step 9.4: exact-head CI/review**
  - 실행: required GitHub checks 완료까지 기다리고, exact head SHA의 review/thread/mergeability와 local review artifact를 다시 읽는다.
  - 기대: required checks X/Y all green, unresolved threads 0, P0/P1 0, single-developer human-review subgate만 concrete evidence와 함께 `N/A`.
- [ ] **Step 9.5: merge-ready report**
  - 실행: CG-15/A-11 결과를 사용자에게 보고한다.
  - 기대: PR URL/number, exact head, CI/review/lesson/docs evidence, `Required checks: X/Y; N/A: N; Blocked: 0`, 미실행 CG-16/17/18이 명시된다. 여기서 멈추고 fresh merge approval을 기다린다.

## Task 10: fresh approval 후 단일 merge·sync·cleanup

- [ ] **Step 10.1: fresh exact-head merge approval**
  - 조건: 사용자가 Task 9.5 merge-ready report 이후 exact PR/head에 대해 새로 승인할 때만 진행한다. 이전 계획 승인과 현재 승인된 merge wording은 대체 근거가 아니다.
- [ ] **Step 10.2: squash merge와 live read-back**
  - 실행: `gh pr merge <number> --squash --match-head-commit <exact-head>`; PR merged state, merge SHA, issue timeline을 확인한다.
  - 기대: 다섯 구현 issue가 실제 변경으로 닫히고, 모든 child가 완료된 뒤 `#550` epic을 닫는다. #548/#555/#553은 기존 상태를 보존한다.
- [ ] **Step 10.3: local sync**
  - 실행: main checkout에서 `git fetch --prune origin`, `git switch develop`, `git pull --ff-only origin develop`, `git rev-parse HEAD origin/develop`.
  - 기대: local `develop`와 `origin/develop`가 merge SHA에서 일치하고 dirty 상태/사용자 변경을 보존한다.
- [ ] **Step 10.4: preservation-first cleanup**
  - 실행: merged ancestry/patch-id/tree equivalence, remote branch/PR 상태와 recovery bundle을 확인한 후에만 feature worktree와 local branch를 삭제한다.
  - 기대: 삭제 대상이 정확히 feature worktree/branch이고, main checkout은 clean 또는 사전 dirty 상태를 그대로 보존한다. ambiguous target은 삭제하지 않는다.

## 요구사항 추적 표

| Spec requirement | Plan task | Fresh proof |
|---|---|---|
| PostGIS SRID/value/DDL/round-trip/index | 1.1–1.5, 7.3 | sqlkit/postgis unit + PostGIS container |
| MySQL/MariaDB engine-specific GIS | 2.1–2.5, 7.3 | separate package tests + two containers |
| Nominatim reverse/error/policy/cancel | 3.1–3.4, 7.2 | httptest fake, race, docs audit |
| FalkorDB OpenCypher/result/context boundary | 4.1–4.5, 7.2–7.3 | fake RESP + digest fixture |
| Gremlin remote/conformance/TinkerPop | 5.1–5.5, 7.2–7.3 | result channel fake + local server |
| Go ownership/redaction/resource/race | every task, 7.1–7.4, 8 | lint, race, review file:line |
| README locales/WIP/CHANGELOG/lesson | 6, 8.3, 9.3 | locale matrix, terminology audit, PR body |
| one integration PR/one merge | 9–10 | exact head, `--match-head-commit`, merge/sync proof |

## Rollback과 중단 기준

- 새 driver가 compile/fixture/CI 계약을 만족하지 않으면 해당 dependency와
  package를 되돌리고 나머지 slice를 임의로 합치지 않는다. 필요한 범위 변경은
  spec/plan review와 사용자 승인을 다시 거친다.
- provider fixture가 flaky하면 retry 횟수만 늘리지 않고 readiness, port,
  cleanup, image digest 원인을 조사한다. 원인이 해결되지 않은 동안 PR/merge는
  `PENDING`이다.
- P0/P1은 PR 전에 반드시 수정한다. P2/P3도 이 통합 범위에서 싸게 수정할 수
  있으면 수정하고, 아니면 후속 issue URL과 impact를 PR/lesson에 남긴다.
- merge 전에는 tag/release/dispatch를 실행하지 않는다. merge 후 cleanup은
  patch-equivalence와 recovery bundle 증거가 없으면 보류한다.
