# Issue #415 Workshop Adoption Matrix

## 맥락

Issue [#415](https://github.com/bluetape4k/bluetape-go/issues/415)는 epic
[#414](https://github.com/bluetape4k/bluetape-go/issues/414) 아래에서
`0.17.0` workshop adoption sync를 시작한다. 목표는 #416에서 library-side
README pointer를 추가하거나 #417에서 cross-repo follow-up comment를 남기기 전에,
library readiness와 workshop-only backlog를 분리하는 것이다.

현재 source evidence는 다음과 같다.

- `bluetape-go` `CHANGELOG.md`에는 `v0.16.0`까지 published section이 있다.
- `bluetape-go-workshop/go.mod`는 현재
  `github.com/bluetape4k/bluetape-go v0.16.0`을 요구한다.
- `bluetape-go-workshop`에는 SQL, AWS/Floci, Redis probabilistic, cache,
  JWT, money, resilience, leader, state, workflow, batch, testcontainers track에
  대해 runnable example이 이미 있다.
- 열려 있는 workshop issue에는 text, audit/outbox, graph, slog,
  cross-release integration flow를 위한 umbrella track과 focused example이 남아 있다.

## Version Semantics

`Workshop issue prefix` column은 workshop issue title에서 나온 historical
source-release lineage를 기록한다. `Current dependency`는 workshop module이 현재
실제로 소비하는 `bluetape-go` version이다. 새 workshop work는 오래된 library milestone에서
도입된 package를 가르치더라도 보통 current dependency(`v0.16.0`)를 기준으로 실행해야 한다.

## 기존 Workshop Coverage

| Track | Library packages | Existing workshop examples | Minimum source line | Current dependency | Owner repo | Status |
|---|---|---|---|---|---|---|
| SQL access and repositories | `sqlkit`, `testcontainers/postgres` | `examples/sql-access-strategy-decision`, `examples/sql-order-repository`, `examples/sql-transaction-boundary`, `examples/gin-sql-crud-api`, `examples/gin-sql-order-service` | `v0.8.0` issue line | `v0.16.0` | `bluetape-go-workshop` | Covered. 추가 SQL issue는 library blocker가 아니라 integration 또는 content-specific issue로 본다. |
| AWS local emulator examples | `testcontainers/floci`, `dynamodb/batchwrite`, AWS SDK v2 examples | `examples/s3-floci-storage`, `examples/sqs-floci-worker`, `examples/dynamodb-batchwrite-materializer`, `examples/dynamodb-conditional-repository`, `examples/s3-sqs-dynamodb-document-workflow` | `v0.9.0` and later issue lines | `v0.16.0` | `bluetape-go-workshop` | S3, SQS, DynamoDB, multi-service document workflow는 covered 상태다. |
| Redis probabilistic admission | `probabilistic`, `probabilistic/redis`, `testcontainers/redis` | `examples/probabilistic-dedupe-admission`, `examples/shared-redis-bloom-admission` | `v0.16.0` follow-up line for HLL | `v0.16.0` | `bluetape-go-workshop` | Bloom coverage는 존재한다. HLL workflow는 workshop #151에 남아 있다. |
| Audit library examples | `audit`, `audit/sqloutbox`, `audit/sqloutbox/sqloutboxtest` | Library repo에는 `examples/audit`가 있고, workshop에는 dedicated audit/outbox application example이 아직 없다. | `v0.9.0`, `v0.11.0`, `v0.15.0` issue lines | `v0.16.0` | Mostly `bluetape-go-workshop` | Library side는 ready다. workshop audit/outbox backlog는 남아 있다. |
| Graph library examples | `graph`, `graph/graphio`, `graph/neo4j` | Library repo에는 `examples/graph/observability`와 `examples/graph/iamaccess`가 있고, workshop에는 graph-domain runnable example이 아직 없다. | `v0.10.0` / `v0.12.0` issue lines | `v0.16.0` | Mostly `bluetape-go-workshop` | Library side는 ready다. workshop graph backlog는 남아 있다. |
| Text search/tokenizer/language | `textsearch`, `textsearch/japanese`, `textsearch/language` | text search, Japanese tokenization, language detection을 다루는 workshop example은 아직 없다. | `v0.8.0` / `v0.10.0` issue lines | `v0.16.0` | `bluetape-go-workshop` | Workshop gap이다. library docs는 package contract를 이미 소유한다. |
| Go-native logging | standard-library `log/slog`; no `bluetape-go` logging facade | cross-cutting workshop logging pattern은 아직 없다. | `v0.9.0` issue line from observability boundary | `v0.16.0` | `bluetape-go-workshop` | Workshop-only backlog다. library logging package를 만들지 않는다. |

## 열려 있는 Workshop Issue Matrix

| Workshop issue | Track | Package thread | Workshop issue prefix | Required library line | Current dependency | Owner repo | Matrix decision |
|---|---|---|---|---|---|---|---|
| [#34](https://github.com/bluetape4k/bluetape-go-workshop/issues/34) | Text umbrella | `textsearch`, `textsearch/japanese`, `textsearch/language` | `v0.8.0` | Text packages from `v0.8.0` / `v0.10.0` line | `v0.16.0` | `bluetape-go-workshop` | umbrella로 유지한다. Children #53, #54, #55, #67은 아직 open이며 existing example과 중복되지 않는다. |
| [#53](https://github.com/bluetape4k/bluetape-go-workshop/issues/53) | Text | `textsearch` | `v0.8.0` | Multi-pattern search/masking | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused example이며 #67의 prerequisite다. |
| [#54](https://github.com/bluetape4k/bluetape-go-workshop/issues/54) | Text HTTP | `textsearch`, Gin | `v0.8.0` | Search/masking service boundary | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused example이며 #67의 prerequisite다. |
| [#55](https://github.com/bluetape4k/bluetape-go-workshop/issues/55) | Tokenizer/language feasibility | `textsearch/japanese`, `textsearch/language` | `v0.8.0` | Optional tokenizer/detector boundary | `v0.16.0` | `bluetape-go-workshop` | #118과 #119가 concrete Japanese 및 language-routing example로 분리되었으므로 broad umbrella로는 stale하다. parent/feasibility note가 될 때만 유지한다. |
| [#67](https://github.com/bluetape4k/bluetape-go-workshop/issues/67) | Text integration | `textsearch`, tokenizer/language packages, Gin | `v0.8.0` | Text package composition | `v0.16.0` | `bluetape-go-workshop` | #53/#54/#118/#119가 수렴한 뒤 유효한 integration issue다. |
| [#118](https://github.com/bluetape4k/bluetape-go-workshop/issues/118) | Japanese tokenizer | `textsearch/japanese` | `v0.8.0` | Kagome-backed optional tokenizer | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue이며 #55를 좁힌다. |
| [#119](https://github.com/bluetape4k/bluetape-go-workshop/issues/119) | Language routing | `textsearch/language` | `v0.8.0` | Lingua-backed optional detector | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue이며 #55를 좁힌다. |
| [#35](https://github.com/bluetape4k/bluetape-go-workshop/issues/35) | Audit umbrella | `audit`, `audit/sqloutbox` | `v0.9.0` | Audit/event and outbox packages | `v0.16.0` | `bluetape-go-workshop` | umbrella로 유지한다. Children #56, #57, #58, #68은 여전히 유용하지만 #150이 relay-test evidence를 좁힌다. |
| [#56](https://github.com/bluetape4k/bluetape-go-workshop/issues/56) | Audit history | `audit` | `v0.9.0` | Audit repository/history APIs | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue다. |
| [#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57) | Outbox publisher | `audit/sqloutbox` | `v0.9.0` | Outbox persistence and publisher handoff | `v0.16.0` | `bluetape-go-workshop` | application-shaped issue로 유효하다. relay-test helper coverage를 중복하지 말고 #150을 cross-link해야 한다. |
| [#58](https://github.com/bluetape4k/bluetape-go-workshop/issues/58) | Audit query API | `audit` | `v0.9.0` | Audit query behavior | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue다. |
| [#68](https://github.com/bluetape4k/bluetape-go-workshop/issues/68) | Audit integration | `audit`, `audit/sqloutbox`, Gin | `v0.9.0` | Audit/outbox composition | `v0.16.0` | `bluetape-go-workshop` | focused audit example이 수렴한 뒤 유효한 integration issue다. |
| [#150](https://github.com/bluetape4k/bluetape-go-workshop/issues/150) | Relay-test evidence | `audit/sqloutbox/sqloutboxtest` | `v0.15.0` | Publisher adoption helpers | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue다. #57/#68을 보완하며 그 안에 합치면 안 된다. |
| [#36](https://github.com/bluetape4k/bluetape-go-workshop/issues/36) | Graph umbrella | `graph`, `graph/graphio`, optional adapters | `v0.10.0` | Graph model / graph I/O line | `v0.16.0` | `bluetape-go-workshop` | umbrella로 유지한다. Children #50, #51, #52, #69가 open 상태다. |
| [#50](https://github.com/bluetape4k/bluetape-go-workshop/issues/50) | Graph abuse cluster | `graph` | `v0.10.0` | Graph model/traversal | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue다. |
| [#51](https://github.com/bluetape4k/bluetape-go-workshop/issues/51) | Graph recommendation | `graph` | `v0.10.0` | Graph model/traversal | `v0.16.0` | `bluetape-go-workshop` | 유효한 focused issue다. |
| [#52](https://github.com/bluetape4k/bluetape-go-workshop/issues/52) | Graph import/export | `graph/graphio` | `v0.10.0` | Graph I/O helpers | `v0.16.0` | `bluetape-go-workshop` | graph I/O가 library repo에 존재하므로 유효한 focused issue다. |
| [#69](https://github.com/bluetape4k/bluetape-go-workshop/issues/69) | Graph integration | `graph`, `graph/graphio` | `v0.10.0` | Graph workflow composition | `v0.16.0` | `bluetape-go-workshop` | #50/#51/#52가 수렴한 뒤 유효한 integration issue다. |
| [#139](https://github.com/bluetape4k/bluetape-go-workshop/issues/139) | Logging | standard `log/slog` | `v0.9.0` | Observability boundary only; no library facade | `v0.16.0` | `bluetape-go-workshop` | 유효한 workshop-only issue다. non-goal을 유지한다: logging dependency 없음, library-owned global logger 없음, MDC facade 없음. |
| [#151](https://github.com/bluetape4k/bluetape-go-workshop/issues/151) | Redis HLL | `probabilistic/redis`, `testcontainers/redis` | `v0.16.0` | Redis HyperLogLog | `v0.16.0` | `bluetape-go-workshop` | 유효한 다음 probabilistic workshop slice다. #415 범위는 아니지만 #417/#418 release-readiness note에 중요하다. |
| [#152](https://github.com/bluetape4k/bluetape-go-workshop/issues/152) | Media intake integration | `imagekit`, `encrypt`, `rules`, `audit/sqloutbox/sqloutboxtest`, `serialization`, `compression`, `codec` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | 유효한 cross-release integration issue다. focused issue #146/#147/#149/#150을 link해야 한다. |
| [#153](https://github.com/bluetape4k/bluetape-go-workshop/issues/153) | Campaign telemetry integration | `probabilistic/redis`, `serialization`, `compression`, `audit/sqloutbox/sqloutboxtest`, `log/slog` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | 유효한 cross-release integration issue다. #151과 #150 style evidence에 의존한다. |
| [#154](https://github.com/bluetape4k/bluetape-go-workshop/issues/154) | Tenant security control plane | `jwt/mongo`, `testcontainers/mongodb`, `rules`, `codec`, `audit/sqloutbox/sqloutboxtest`, `log/slog` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | 유효한 cross-release integration issue이며 library blocker가 아니다. |

## 중복 및 Stale Scope

- #55는 이 slice에서 유일하게 materially stale한 scope다. feasibility parent로는
  여전히 유용하지만, Japanese tokenization과 language-routing behavior 구현에는 #118과
  #119가 더 명확하다.
- #57/#68과 #150은 중복이 아니다. #57/#68은 application outbox flow이고,
  #150은 `sqloutboxtest` helper에 대한 relay-test evidence이므로 더 좁게 유지해야 한다.
- #34, #35, #36은 umbrella이며 직접 구현 대상이 아니다. child issue를 먼저
  완료하거나 닫고, umbrella는 checklist roll-up에 사용한다.
- #139는 의도적으로 workshop-only다. library logging facade는 현재 Go-native
  observability boundary를 위반한다.
- #151-#154는 유효한 cross-release adoption issue이지만 workshop backlog로 다룬다.
  구현 중 concrete library defect가 발견되지 않는 한 `bluetape-go` library release를
  막지 않는다.

## 권장 0.17.0 순서

1. 이 matrix를 #416 README pointer의 source로 사용한다.
2. #417에서 focused workshop issue에서 관련 package README와 이 matrix로 돌아오는
   cross-repo comment 또는 link를 추가한다.
3. 구현 순서는 `bluetape-go-workshop`에 위임한다. library repo는 missing public
   contract가 발견될 때만 새 package work를 만든다.
4. #418은 남은 workshop backlog와 독립적으로 `bluetape-go`가 release 가능하다는
   release-readiness note로 마무리한다.

## 검증 메모

- `gh issue list --repo bluetape4k/bluetape-go-workshop --state open`으로 위에 매핑한
  workshop issue들을 확인했다.
- #34, #35, #36, #50-#58, #67-#69, #118, #119, #139, #150-#154는 `gh issue view`로
  확인했다.
- workshop example inventory는
  `/Users/debop/work/bluetape4k/bluetape-go-workshop/examples`에서 확인했다.
- 현재 workshop dependency는
  `/Users/debop/work/bluetape4k/bluetape-go-workshop/go.mod`에서 확인했다.
