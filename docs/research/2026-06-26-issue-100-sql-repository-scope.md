# Issue 100 SQL DSL And Repository Research Scope

Issue #100은 0.7.0 implementation epic #101에 앞서 Go relational
data-access 방향을 결정한다. 결론은 runtime-first이다. 작은
`database/sql` transaction, row mapping, inspectable SQL builder helper를
구현하되 code generation은 optional external workflow로 유지한다.

## 소스 인벤토리

Source repositories:

- `/Users/debop/work/bluetape4k/bluetape4k-exposed`
- `/Users/debop/work/bluetape4k/bluetape4k-projects/data`

Relevant source capabilities:

- `bluetape4k-exposed`는 repository patterns, transaction DSLs,
  JDBC/R2DBC repositories, CTE helpers, cache decorators, JSON columns, Tink
  encrypted columns, measured columns, dialect modules, Spring Boot wiring,
  real database examples를 제공한다.
- `data/jdbc`는 connection/statement helpers, typed result access,
  transaction support, batch processing, row mapping helpers를 제공한다.
- `data/r2dbc`는 coroutine/Flow data access, pool tuning, transaction
  support, query helpers, Spring auto-configuration을 제공한다.
- `data/mongodb`와 `data/cassandra`는 comparison evidence일 뿐이며
  relational 0.7.0 scope에는 포함하지 않는다.

Current bluetape-go evidence:

- 기존 Testcontainers fixture는 PostgreSQL, MySQL, MariaDB를 다룬다.
- 0.7.0 research note는 이미 Kotlin Exposed clone, full ORM layer,
  mandatory generated code default를 기각했다.
- 기존 Go package는 explicit `context.Context`, visible errors,
  caller-owned resource lifecycle을 사용한다.

## 외부 Go 근거

- `database/sql`은 여전히 standard-library execution boundary이며
  context-aware query, exec, transaction, row, rows API를 소유한다.
- sqlc는 hand-written SQL에서 type-safe Go code를 생성하며, generated
  source를 workflow 일부로 받아들이는 project에 가장 적합하다.
- Jet은 database schema에서 type-safe SQL builder/model code를 제공하고
  complex dynamic SQL에 강하지만 builder file이 generated이다.
- ent는 schema-as-code, graph-style edges, migrations를 가진 code-generation
  entity framework다. bluetape-go의 첫 SQL package에는 너무 넓다.
- Bob은 PostgreSQL, MySQL/MariaDB, SQLite용 query/model/ORM generation을
  지원하지만, 그 성격 때문에 core default가 아니라 later comparison
  candidate로 둔다.
- goqu는 expressive runtime SQL builder/executor candidate이지만,
  bluetape-go가 자체 minimal SQL contract를 증명하기 전에 dependency를
  추가하게 된다.
- Atlas는 credible external migration tool이지만 migration execution은 첫
  library package 밖에 둔다.

Sources:

- https://pkg.go.dev/database/sql
- https://docs.sqlc.dev/
- https://github.com/go-jet/jet
- https://entgo.io/docs/getting-started/
- https://bob.stephenafamo.com/docs/
- https://doug-martin.github.io/goqu/
- https://atlasgo.io/

## 후보 순위

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| `database/sql` transaction helpers | High | Medium | Implement in #317. |
| Row mapping/cardinality helpers | High | Medium | Implement in #317. |
| Minimal inspectable SQL builder | Medium/high | Medium/high | Implement after foundation in #318. |
| Repository example conventions | High | Medium | Implement with #318. |
| PostgreSQL Testcontainers anchor | High | Medium | Use first; MySQL/MariaDB later. |
| sqlc optional generated-code workflow | Medium/high | Medium | Document as optional in #319. |
| Jet optional generated builder workflow | Medium/high | Medium | Document as optional in #319. |
| Atlas migration workflow | Medium | Medium | Recommend externally in #319; do not wrap. |
| ent | Medium | High | Defer; full entity framework is too broad. |
| Bob | Medium | High | Defer; generator/ORM surface is too broad for first slice. |
| Bun/GORM | Medium | High | Defer/reject for core; too ORM-shaped for this milestone. |
| goqu | Medium | Medium | Defer dependency adoption until minimal builder proves insufficient. |
| JSON/encrypted/measured columns | Medium | High | Defer to later child issues after base API lands. |
| Cache-backed repositories | Medium | High | Defer until repository contracts and invalidation semantics are proven. |
| Non-relational MongoDB/Cassandra | Low | High | Out of #101 scope. |

## 구현

0.7.0 child issue 세 개를 만든다.

- #317: runtime SQL transaction and row mapping foundation.
- #318: inspectable SQL builder and repository prototype.
- #319: optional sqlc/Jet generator and Atlas migration guidance.

## #101 방향

- SQL을 visible하게 유지하고 driver behavior를 caller-owned로 두며 dependency를
  작게 유지하기 위해 `database/sql`로 시작한다.
- 기존 Testcontainers fixture가 PostgreSQL을 다루고 PostgreSQL이 약한
  abstraction을 초기에 드러낼 만큼 충분한 SQL feature를 제공하므로,
  PostgreSQL을 첫 integration anchor로 둔다.
- MySQL/MariaDB compatibility는 API shape가 안정된 뒤 follow-up으로 둔다.
- `pgx`는 compatible driver/runtime candidate로 취급하되, 첫 package의
  required public dependency로 만들지 않는다.
- Code generation은 optional로 유지한다. sqlc와 Jet은 좋은 workflow
  example이지만 mandatory generated code는 runtime-first user story와
  충돌한다.
- Migration은 external로 유지한다. Atlas는 docs에서 추천할 수 있지만
  bluetape-go가 migration execution을 숨기면 안 된다.

## 보류

- concrete consumer가 minimal runtime API가 부족함을 증명하기 전까지 ent,
  Bob, Bun, GORM, goqu adoption을 보류한다.
- Serialization/envelope behavior가 명확해질 때까지 JSON column helper를
  보류한다.
- #315가 착지하고 SQL column consumer가 key material과 associated data
  rule을 소유하기 전까지 encrypted column을 보류한다.
- SQL package user가 `measure`/`money` persistence helper를 필요로 할 때까지
  measured column을 보류한다.
- Transaction boundary, stale-read behavior, invalidation, retry semantics가
  설계되기 전까지 cache-backed repository를 보류한다.
- Base builder/repository prototype이 착지하기 전까지 CTE, upsert, batch,
  schema metadata, dialect module을 보류한다.

## 필요한 이슈 업데이트

- #100은 이 결론을 기록하고 research PR로 닫아야 한다.
- #101은 #317, #318, #319를 첫 implementation child로 나열해야 한다.
- #7은 SQL research가 완료되어 구현이 #101 / 0.7.0으로 이동했음을
  기록해야 한다.

## 검증 계획

- Documentation-only PR: `git diff --check` and targeted `rg`.
- #100, #101, #7, #317, #318, #319 issue body에 #100 research outcome 또는
  research note path가 들어 있는지 확인한다.
- External evidence는 `bluetape4k-wiki`에 보존하고 `gno update`,
  `gno embed --collection bluetape4k-wiki`, representative `gno search`로
  검증한다.
- Go code change가 없으므로 이 PR에는 Go test가 필요하지 않다.

## 후속 권고

0.7.0에서는 #317을 먼저 진행한다. Transaction과 row mapping contract가
real PostgreSQL container로 검증되기 전에는 SQL builder를 시작하지 않는다.
