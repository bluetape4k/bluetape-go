# Issue #415 Workshop Adoption Matrix

## Context

Issue [#415](https://github.com/bluetape4k/bluetape-go/issues/415)
starts the `0.17.0` workshop adoption sync under epic
[#414](https://github.com/bluetape4k/bluetape-go/issues/414). The goal is to
separate library readiness from workshop-only backlog before adding library-side
README pointers in #416 or cross-repo follow-up comments in #417.

Current source evidence:

- `bluetape-go` `CHANGELOG.md` contains published sections through `v0.16.0`.
- `bluetape-go-workshop/go.mod` currently requires
  `github.com/bluetape4k/bluetape-go v0.16.0`.
- `bluetape-go-workshop` already has runnable examples for SQL, AWS/Floci,
  Redis probabilistic, cache, JWT, money, resilience, leader, state, workflow,
  batch, and testcontainers tracks.
- Open workshop issues still contain several umbrella tracks and focused
  examples for text, audit/outbox, graph, slog, and cross-release integration
  flows.

## Version Semantics

The `Workshop issue prefix` column records the historical source-release lineage
from the workshop issue title. The `Current dependency` is the actual
`bluetape-go` version consumed by the workshop module today. New workshop work
should usually run against the current dependency (`v0.16.0`) even when it
teaches a package introduced by an older library milestone.

## Existing Workshop Coverage

| Track | Library packages | Existing workshop examples | Minimum source line | Current dependency | Owner repo | Status |
|---|---|---|---|---|---|---|
| SQL access and repositories | `sqlkit`, `testcontainers/postgres` | `examples/sql-access-strategy-decision`, `examples/sql-order-repository`, `examples/sql-transaction-boundary`, `examples/gin-sql-crud-api`, `examples/gin-sql-order-service` | `v0.8.0` issue line | `v0.16.0` | `bluetape-go-workshop` | Covered. Treat additional SQL issues as integration or content-specific, not library blockers. |
| AWS local emulator examples | `testcontainers/floci`, `dynamodb/batchwrite`, AWS SDK v2 examples | `examples/s3-floci-storage`, `examples/sqs-floci-worker`, `examples/dynamodb-batchwrite-materializer`, `examples/dynamodb-conditional-repository`, `examples/s3-sqs-dynamodb-document-workflow` | `v0.9.0` and later issue lines | `v0.16.0` | `bluetape-go-workshop` | Covered for S3, SQS, DynamoDB, and multi-service document workflow. |
| Redis probabilistic admission | `probabilistic`, `probabilistic/redis`, `testcontainers/redis` | `examples/probabilistic-dedupe-admission`, `examples/shared-redis-bloom-admission` | `v0.16.0` follow-up line for HLL | `v0.16.0` | `bluetape-go-workshop` | Bloom coverage exists; HLL workflow remains open in workshop #151. |
| Audit library examples | `audit`, `audit/sqloutbox`, `audit/sqloutbox/sqloutboxtest` | Library repo has `examples/audit`; workshop has no dedicated audit/outbox application example yet. | `v0.9.0`, `v0.11.0`, `v0.15.0` issue lines | `v0.16.0` | Mostly `bluetape-go-workshop` | Library side is ready; workshop audit/outbox backlog remains. |
| Graph library examples | `graph`, `graph/graphio`, `graph/neo4j` | Library repo has `examples/graph/observability` and `examples/graph/iamaccess`; workshop has no graph-domain runnable example yet. | `v0.10.0` / `v0.12.0` issue lines | `v0.16.0` | Mostly `bluetape-go-workshop` | Library side is ready; workshop graph backlog remains. |
| Text search/tokenizer/language | `textsearch`, `textsearch/japanese`, `textsearch/language` | No workshop example currently covers text search, Japanese tokenization, or language detection. | `v0.8.0` / `v0.10.0` issue lines | `v0.16.0` | `bluetape-go-workshop` | Workshop gap; library docs already own package contracts. |
| Go-native logging | standard-library `log/slog`; no `bluetape-go` logging facade | No cross-cutting workshop logging pattern yet. | `v0.9.0` issue line from observability boundary | `v0.16.0` | `bluetape-go-workshop` | Workshop-only backlog. Do not create a library logging package. |

## Open Workshop Issue Matrix

| Workshop issue | Track | Package thread | Workshop issue prefix | Required library line | Current dependency | Owner repo | Matrix decision |
|---|---|---|---|---|---|---|---|
| [#34](https://github.com/bluetape4k/bluetape-go-workshop/issues/34) | Text umbrella | `textsearch`, `textsearch/japanese`, `textsearch/language` | `v0.8.0` | Text packages from `v0.8.0` / `v0.10.0` line | `v0.16.0` | `bluetape-go-workshop` | Keep as umbrella. Children #53, #54, #55, #67 are still open and not duplicated by existing examples. |
| [#53](https://github.com/bluetape4k/bluetape-go-workshop/issues/53) | Text | `textsearch` | `v0.8.0` | Multi-pattern search/masking | `v0.16.0` | `bluetape-go-workshop` | Valid focused example; prerequisite for #67. |
| [#54](https://github.com/bluetape4k/bluetape-go-workshop/issues/54) | Text HTTP | `textsearch`, Gin | `v0.8.0` | Search/masking service boundary | `v0.16.0` | `bluetape-go-workshop` | Valid focused example; prerequisite for #67. |
| [#55](https://github.com/bluetape4k/bluetape-go-workshop/issues/55) | Tokenizer/language feasibility | `textsearch/japanese`, `textsearch/language` | `v0.8.0` | Optional tokenizer/detector boundary | `v0.16.0` | `bluetape-go-workshop` | Stale as a broad umbrella now that #118 and #119 split concrete Japanese and language-routing examples. Keep only if it becomes the parent/feasibility note. |
| [#67](https://github.com/bluetape4k/bluetape-go-workshop/issues/67) | Text integration | `textsearch`, tokenizer/language packages, Gin | `v0.8.0` | Text package composition | `v0.16.0` | `bluetape-go-workshop` | Valid integration issue after #53/#54/#118/#119 converge. |
| [#118](https://github.com/bluetape4k/bluetape-go-workshop/issues/118) | Japanese tokenizer | `textsearch/japanese` | `v0.8.0` | Kagome-backed optional tokenizer | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue; narrows #55. |
| [#119](https://github.com/bluetape4k/bluetape-go-workshop/issues/119) | Language routing | `textsearch/language` | `v0.8.0` | Lingua-backed optional detector | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue; narrows #55. |
| [#35](https://github.com/bluetape4k/bluetape-go-workshop/issues/35) | Audit umbrella | `audit`, `audit/sqloutbox` | `v0.9.0` | Audit/event and outbox packages | `v0.16.0` | `bluetape-go-workshop` | Keep as umbrella. Children #56, #57, #58, #68 remain useful, but #150 narrows relay-test evidence. |
| [#56](https://github.com/bluetape4k/bluetape-go-workshop/issues/56) | Audit history | `audit` | `v0.9.0` | Audit repository/history APIs | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue. |
| [#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57) | Outbox publisher | `audit/sqloutbox` | `v0.9.0` | Outbox persistence and publisher handoff | `v0.16.0` | `bluetape-go-workshop` | Valid application-shaped issue; should cross-link #150 instead of duplicating relay-test helper coverage. |
| [#58](https://github.com/bluetape4k/bluetape-go-workshop/issues/58) | Audit query API | `audit` | `v0.9.0` | Audit query behavior | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue. |
| [#68](https://github.com/bluetape4k/bluetape-go-workshop/issues/68) | Audit integration | `audit`, `audit/sqloutbox`, Gin | `v0.9.0` | Audit/outbox composition | `v0.16.0` | `bluetape-go-workshop` | Valid integration issue after focused audit examples converge. |
| [#150](https://github.com/bluetape4k/bluetape-go-workshop/issues/150) | Relay-test evidence | `audit/sqloutbox/sqloutboxtest` | `v0.15.0` | Publisher adoption helpers | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue; complements #57/#68 and should not be merged into them. |
| [#36](https://github.com/bluetape4k/bluetape-go-workshop/issues/36) | Graph umbrella | `graph`, `graph/graphio`, optional adapters | `v0.10.0` | Graph model / graph I/O line | `v0.16.0` | `bluetape-go-workshop` | Keep as umbrella. Children #50, #51, #52, #69 remain open. |
| [#50](https://github.com/bluetape4k/bluetape-go-workshop/issues/50) | Graph abuse cluster | `graph` | `v0.10.0` | Graph model/traversal | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue. |
| [#51](https://github.com/bluetape4k/bluetape-go-workshop/issues/51) | Graph recommendation | `graph` | `v0.10.0` | Graph model/traversal | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue. |
| [#52](https://github.com/bluetape4k/bluetape-go-workshop/issues/52) | Graph import/export | `graph/graphio` | `v0.10.0` | Graph I/O helpers | `v0.16.0` | `bluetape-go-workshop` | Valid focused issue now that graph I/O exists in the library repo. |
| [#69](https://github.com/bluetape4k/bluetape-go-workshop/issues/69) | Graph integration | `graph`, `graph/graphio` | `v0.10.0` | Graph workflow composition | `v0.16.0` | `bluetape-go-workshop` | Valid integration issue after #50/#51/#52 converge. |
| [#139](https://github.com/bluetape4k/bluetape-go-workshop/issues/139) | Logging | standard `log/slog` | `v0.9.0` | Observability boundary only; no library facade | `v0.16.0` | `bluetape-go-workshop` | Valid workshop-only issue. Keep the non-goals: no logging dependency, no library-owned global logger, no MDC facade. |
| [#151](https://github.com/bluetape4k/bluetape-go-workshop/issues/151) | Redis HLL | `probabilistic/redis`, `testcontainers/redis` | `v0.16.0` | Redis HyperLogLog | `v0.16.0` | `bluetape-go-workshop` | Valid next probabilistic workshop slice; not part of #415 scope, but important for #417/#418 release-readiness notes. |
| [#152](https://github.com/bluetape4k/bluetape-go-workshop/issues/152) | Media intake integration | `imagekit`, `encrypt`, `rules`, `audit/sqloutbox/sqloutboxtest`, `serialization`, `compression`, `codec` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | Valid cross-release integration issue; should link focused issues #146/#147/#149/#150. |
| [#153](https://github.com/bluetape4k/bluetape-go-workshop/issues/153) | Campaign telemetry integration | `probabilistic/redis`, `serialization`, `compression`, `audit/sqloutbox/sqloutboxtest`, `log/slog` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | Valid cross-release integration issue; depends on #151 and #150 style evidence. |
| [#154](https://github.com/bluetape4k/bluetape-go-workshop/issues/154) | Tenant security control plane | `jwt/mongo`, `testcontainers/mongodb`, `rules`, `codec`, `audit/sqloutbox/sqloutboxtest`, `log/slog` | `v0.16.0` | Cross-release integration | `v0.16.0` | `bluetape-go-workshop` | Valid cross-release integration issue; not a library blocker. |

## Duplicates and Stale Scopes

- #55 is the only materially stale scope in this slice. It is still useful as a
  feasibility parent, but #118 and #119 are the clearer implementation issues
  for Japanese tokenization and language-routing behavior.
- #57/#68 and #150 are not duplicates. #57/#68 are application outbox flows;
  #150 is relay-test evidence for `sqloutboxtest` helpers and should remain
  narrower.
- #34, #35, and #36 are umbrellas and should not be implemented directly.
  Complete or close their child issues first, then use the umbrellas for
  checklist roll-up.
- #139 is intentionally workshop-only. A library logging facade would violate
  the current Go-native observability boundary.
- #151-#154 are valid cross-release adoption issues, but they should be treated
  as workshop backlog. They do not block a `bluetape-go` library release unless
  a concrete library defect is discovered while implementing them.

## Recommended 0.17.0 Order

1. Use this matrix as the source for #416 README pointers.
2. In #417, add cross-repo comments or links from focused workshop issues back
   to the relevant package README and this matrix.
3. Defer implementation sequencing to `bluetape-go-workshop`; library repo
   should not create new package work unless an issue uncovers a missing public
   contract.
4. Finish #418 with a release-readiness note that states `bluetape-go` can
   release independently of remaining workshop backlog.

## Verification Notes

- `gh issue list --repo bluetape4k/bluetape-go-workshop --state open`
  returned the workshop issues mapped above.
- `gh issue view` was used for #34, #35, #36, #50-#58, #67-#69, #118, #119,
  #139, and #150-#154.
- The workshop example inventory was checked from
  `/Users/debop/work/bluetape4k/bluetape-go-workshop/examples`.
- The current workshop dependency was checked from
  `/Users/debop/work/bluetape4k/bluetape-go-workshop/go.mod`.
