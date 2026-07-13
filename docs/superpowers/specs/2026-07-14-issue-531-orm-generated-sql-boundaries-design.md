# Issue #531 ORM 및 generated SQL 경계 예제 설계

## 배경

`sqlkit`은 caller-owned `database/sql` 연결과 명시적인 transaction 경계를 유지하는
runtime helper다. 기존 SQL generator/migration 가이드는 sqlc, Jet, ent, Bun, GORM,
Atlas의 선택 기준과 격리 원칙을 설명하지만, 어떤 handle을 bluetape-go provider에
전달해야 하는지와 pool 공유와 transaction 공유가 어떻게 다른지는 충분히 구체적이지
않다.

Issue #531은 production package나 dependency를 변경하지 않고 이 경계를 문서와
compile-checked example로 고정한다. ORM lifecycle과 generated code는 application이
소유하고, bluetape-go는 표준 SQL 또는 좁은 caller-owned interface만 받는다.

## 목표

- GORM, ent, Bun, sqlc, Jet, Atlas의 권장 통합 경계를 영어/한국어 가이드에 맞춰
  설명한다.
- provider와 repository가 받을 수 있는 `sqlkit.Session`, `*sql.DB`, `*sql.Tx`, 또는
  caller-owned generated query handle의 선택 기준을 명시한다.
- pool 공유와 transaction 공유를 구분하고 하나의 transaction owner만 commit 또는
  rollback하도록 한다.
- generated package를 application-owned 경로에 격리하고 core package가 generated
  model 또는 ORM state를 노출하지 않게 한다.
- 문서/예제 전용, 선택적 adapter 검토, core dependency 거부의 3단계 정책 매트릭스를
  추가한다.
- 외부 dependency 없이 표현할 수 있는 공통 경계 예제를 실제 Go compiler로 검증한다.

## 비목표

- GORM, ent, Bun, sqlc, Jet, Atlas, goqu 또는 Bob dependency를 추가하지 않는다.
- ORM transaction, hook, identity map, eager loading 또는 migration lifecycle을
  `sqlkit`이 추상화하지 않는다.
- generic repository framework, 공통 ORM adapter interface 또는 generated-code
  framework를 만들지 않는다.
- production Go code, 공개 API, database schema 또는 migration을 변경하지 않는다.
- 특정 ORM 또는 generator가 다른 선택지보다 빠르거나 우수하다고 주장하지 않는다.
- 새 diagram을 만들지 않는다. 이번 변경은 순서나 topology보다 선택 정책 비교가
  중심이므로 표와 짧은 code example이 더 직접적이다.

## 검토한 접근

### 접근 1: 공식 문서 snippet과 dependency-free compile example 분리 (채택)

외부 도구별 section은 현재 공식 문서를 근거로 실제 constructor와 transaction API를
짧게 보여 준다. 별도의 Go example은 `database/sql`, `sqlkit.Session`, `*sql.DB`,
`*sql.Tx`, application-owned interface만 사용해 공통 boundary를 컴파일한다.

장점:

- issue의 무의존성 원칙을 지키면서 공통 예제의 drift를 compiler가 탐지한다.
- ORM별 lifecycle을 core abstraction으로 일반화하지 않는다.
- external API snippet과 bluetape-go 자체 계약의 검증 수준을 정직하게 구분한다.

단점:

- ORM별 snippet은 해당 dependency를 import하지 않으므로 link와 수동 검토가 필요하다.
- upstream API가 바뀌면 공식 문서 재검증이 필요하다.

### 접근 2: Markdown snippet만 추가 (제외)

가장 작지만 기존 가이드의 핵심 약점인 public example compile proof를 보강하지 못한다.
공통 transaction owner와 generated handle isolation은 dependency 없이도 검증할 수 있으므로
문서만 수정하는 것은 증거가 부족하다.

### 접근 3: 실제 ORM dependency 또는 adapter package 추가 (제외)

각 snippet을 완전히 컴파일할 수 있지만 dependency graph와 maintenance surface가 크게
늘고 issue의 non-goal을 직접 위반한다. 반복되는 실제 caller 수요와 별도 승인이 생기기
전에는 검토하지 않는다.

## 문서 구조

`docs/sql-generator-migration-guidance.md`와 한국어 pair에 같은 순서로 다음 내용을
추가한다.

1. **Boundary contract**: provider/repository가 받아야 할 handle과 선택 기준.
2. **Policy matrix**: documentation/example-only, optional adapter, rejected core dependency.
3. **Transaction ownership**: pool sharing과 transaction sharing의 차이, 단일
   commit/rollback owner, context/error propagation.
4. **Tool-specific examples**: GORM, ent, Bun, sqlc, Jet, Atlas의 가장 작은 통합 형태.
5. **Generated-code isolation**: application-owned output과 core import 방향.
6. **Verification boundary**: compile-checked 공통 예제와 link-reviewed external snippet의
   구분.

영문과 한국어 문서는 제목, 표 row, example, warning, reference를 동일하게 유지한다.
한국어는 technical identifier를 번역하지 않고 자연스러운 engineer-to-engineer 문장으로
작성한다.

## Handle 선택과 transaction 소유권

| Caller 상황 | 전달할 경계 | 소유권 |
|---|---|---|
| `sqlkit` query/helper 조합 | `sqlkit.Session` | caller가 session lifecycle과 transaction을 소유한다. |
| pool-level 작업 | `*sql.DB` | caller가 pool 설정과 close를 소유한다. |
| 이미 열린 transaction 내부 작업 | `*sql.Tx` | transaction을 시작한 layer만 commit/rollback한다. |
| generated query package | application-owned narrow interface 또는 transaction-bound generated handle | generated package와 binding은 application이 소유한다. |
| ORM lifecycle 작업 | ORM 자체 client/session | bluetape-go provider에 ORM state를 전달하지 않는다. |

같은 unit of work에서 ORM과 direct SQL을 혼합할 때는 단순히 같은 `*sql.DB`를 쓰는 것을
transaction 공유라고 부르지 않는다. 한 layer가 transaction을 시작하고, 다른 작업은 그
transaction에 공식적으로 bind될 수 있을 때만 atomicity를 주장한다. 공식 API가 그 bind를
지원하지 않거나 불명확하면 작업을 분리하고 atomicity를 문서화하지 않는다.

## Tool별 경계

- **GORM**: caller-owned `*sql.DB`로 GORM connection을 초기화할 수 있음을 보여 주되,
  pool 공유가 기존 `*sql.Tx` 공유를 자동으로 의미하지 않는다고 경고한다. GORM-owned
  transaction에는 GORM callback/session을 사용한다.
- **ent**: caller-owned `*sql.DB`를 ent SQL driver에 연결하는 형태와 ent-owned
  transactional client를 보여 준다. ent entity/client를 bluetape-go provider API로
  확산하지 않는다.
- **Bun**: caller-owned `*sql.DB`로 Bun DB를 구성하고 Bun transaction이 `sql.Tx`를
  감싸는 경계를 설명한다. commit/rollback은 Bun transaction owner 한 곳에서 수행한다.
- **sqlc**: generated `DBTX`/`Queries`와 transaction-bound query handle을
  application package에 유지한다. provider에는 필요한 method만 가진 caller-owned
  interface를 넘긴다.
- **Jet**: generated table/model import와 statement execution을 application edge에 둔다.
  destination directory는 격리하고 generator가 core package를 덮어쓰지 않게 한다.
- **Atlas**: runtime handle이 없는 external migration tool로 유지한다. schema planning,
  lint, apply는 application CI/CD 또는 operator runbook이 소유한다.

구현 시 각 이름과 snippet은 현재 official/upstream documentation에서 다시 검증한다.
검증되지 않은 내부 field 또는 unsupported transaction binding은 예제로 제시하지 않는다.

설계 시 확인한 primary source는 다음과 같다.

- [GORM existing database connection](https://gorm.io/docs/connecting_to_the_database.html)
- [ent `sql.DB` integration](https://entgo.io/docs/sql-integration/)
- [ent transactions](https://entgo.io/docs/transactions/)
- [Bun transactions](https://bun.uptrace.dev/guide/transactions.html)
- [sqlc transactions](https://docs.sqlc.dev/en/latest/howto/transactions.html)
- [Jet generator](https://github.com/go-jet/jet/wiki/Generator)
- [Atlas migration planning](https://atlasgo.io/versioned/diff)
- [Atlas migration apply](https://atlasgo.io/versioned/apply)

## Dependency 정책

| 단계 | 허용 조건 | #531 결정 |
|---|---|---|
| Documentation/example-only | 표준 SQL 경계를 설명하고 core dependency가 필요 없음 | GORM, ent, Bun, sqlc, Jet, Atlas 모두 허용 |
| Optional adapter | 반복되는 실제 caller와 좁은 stable boundary, 별도 module/dependency 승인, 독립 test evidence가 있음 | 이번 issue에서는 구현하지 않고 별도 제안 대상으로만 기록 |
| Rejected core dependency | ORM/generated model/lifecycle을 core API로 노출하거나 runtime dependency를 강제함 | 모두 거부 |

## Compile-checked example

새 external dependency를 import하지 않는 `sqlkit` example test를 추가한다. 이 파일은
다음을 보여 준다.

- repository function이 `sqlkit.Session`을 받아 `*sql.DB`와 `*sql.Tx` 모두에서
  호출될 수 있음;
- `sqlkit.WithTx` callback 안에서 application-owned binder가 generated query handle을
  `*sql.Tx`에 연결함;
- nested layer는 commit/rollback하지 않고 error를 transaction owner에게 반환함;
- generated method surface는 예제 전용 narrow interface로 제한됨.

예제는 실제 database connection을 열지 않고 compile만 수행한다. ORM-specific snippet은
dependency가 없으므로 compile-checked라고 표시하지 않으며, official link와 identifier
검토로 검증한다.

## 오류 및 lifecycle 계약

- 모든 operation은 caller의 `context.Context`를 그대로 전달한다.
- nested repository/generated handle은 error를 감추거나 commit으로 바꾸지 않고 반환한다.
- transaction owner만 commit/rollback하며 nested helper는 transaction 종료를 시도하지
  않는다.
- generated package와 ORM client의 close/lifecycle은 application이 소유한다.
- migration failure와 runtime query failure를 하나의 generic provider error로 합치지 않는다.
- 예제에는 실제 DSN, credential 또는 production schema identifier를 넣지 않는다.

## 검증 전략

- English/Korean heading, table row, code example, warning, reference parity를 확인한다.
- 추가된 모든 external link가 official/upstream source인지 확인하고 link 접근성을 검사한다.
- 새 example에 대해 targeted `go test ./sqlkit`를 실행하고 최종 repository gate로
  `make ci`를 실행한다.
- `git diff --check`, `make fmt-check`, `make tidy-check`로 whitespace와 Go module 무변경을
  확인한다.
- production `.go` 파일, `go.mod`, `go.sum`, schema, migration, diagram asset이 변경되지
  않았음을 diff로 확인한다.
- main session에서 performance, stability, security, operator/Ops, developer/API,
  user/caller 관점과 integration verdict를 직접 검토한다. 이 maintenance 범위에서는
  별도 subagent를 생성하지 않는다.

## 전달 경계

설계 승인 후 상세 implementation plan을 작성한다. 구현과 검증이 끝나면 English public
commit/PR metadata를 사용해 `develop` 대상 PR을 생성하고 CI를 기다린다. CI green,
최종 review thread 확인, P0/P1=0까지 확인한 뒤 merge하지 않고 사용자 승인을 요청한다.
