# Issue #426 Feature Gap Triage

## Scope

- Issue: #426 `triage: Identify missing feature gaps in existing packages`
- Parent: #423
- Implementation bucket: #427
- Worktree: `.worktrees/issue-426-gap-triage`
- Boundary: classification only; no implementation in this issue.

## Method

The triage compared current public package claims, recent review outcomes, and
source-parity research notes. The root README is the package index and project
rule source; package READMEs and research notes are used only when they name a
caller problem or a deferred package family.

## Gap Matrix

| Package/surface | Caller problem | Evidence | Severity | Proposed owner | Decision |
|---|---|---|---|---|---|
| `testcontainers/mongodb` | JWT Mongo repository tests had a private MongoDB launcher, so callers could not reuse the same fixture shape as Redis/Postgres/MySQL/Kafka/NATS packages. | `docs/superpowers/research/2026-06-23-issue-218-db-storage-matrix.md:23,36,59-60` deferred MongoDB until the Mongo backend started; `docs/review/2026-07-07-issue-430-mongodb-testcontainer-review.md:13-31` records the shared fixture and JWT refactor; `README.md:71` now lists the active package. | P1 | #430 under #427 | Implemented before this triage closed. This is the only 0.13.0 must-have gap found. |
| `audit` publisher adapters and example services | The package family is named as planned work, but current audit/sqloutbox surfaces are coherent and no current review found a blocker. | `README.md:102-103` lists active audit and sqloutbox packages; `README.md:110-111` names publisher adapters and example services as planned; `docs/review/2026-07-07-issue-429-cumulative-hardening-review.md:30` found no new P0/P1 in audit/outbox; `docs/review/2026-07-07-0.13.0-retrospective-review.md:148-150` found no new hardening or feature-gap item in that review window. | P2 | 0.14.0+ or narrower audit adapter issues | Defer. Valuable, but broader than retrospective hardening and not a 0.13.0 blocker. |
| `probabilistic/redis` Cuckoo and HyperLogLog/HLL | Redis Bloom is active, but Cuckoo/HLL are separate data structures and should not be hidden inside the Bloom surface. | `README.md:107-112` lists in-memory Bloom, Redis Bloom, and separately tracks Cuckoo/HLL after Redis Bloom. | P2 | Later milestone | Defer. Requires distinct API and compatibility contracts. |
| Messaging fixtures: RabbitMQ, Redpanda, Pulsar | Broader broker fixture coverage may be useful for outbox/audit/examples, but no selected broker semantics are committed yet. | `testcontainers/toxiproxy/README.md:132-135` defers these brokers to #58 until outbox semantics select one; `docs/research/2026-06-21-issue-202-source-parity-matrix.md:66` says to add them only when audit/outbox or examples require them. | P2/P3 | 0.14.0+ after broker semantics issue | Defer. Full broker catalog parity is explicitly excluded until caller evidence exists. |
| HTTP mock fixtures: WireMock, Nginx, Mailpit | Current package tests use local HTTP testing; a global mock-fixture package would add surface before a caller proves `httptest` is insufficient. | `testcontainers/toxiproxy/README.md:136-137` defers these to #224 or another concrete package issue; `docs/research/2026-06-21-issue-202-source-parity-matrix.md:67` says HTTP mock fixtures should wait for IO/HTTP integration recipes. | P3 | Backlog or concrete IO/HTTP issue | Reject for 0.13.0; defer only if a future package proves standard-library tests are insufficient. |
| AWS/storage emulators: MinIO, DynamoDB Local, ElasticMQ, SNS/SQS-compatible emulators | AWS examples exist, but emulator choice belongs to the AWS track and should not be selected by retrospective triage. | `README.md:79-80` lists Floci-backed S3 and SQS/SNS examples; `testcontainers/toxiproxy/README.md:138` defers ElasticMQ and SNS/SQS emulators; `docs/superpowers/research/2026-06-23-issue-218-db-storage-matrix.md:37-38,61-62` routes MinIO and DynamoDB Local to #220/#61-#64. | P2 | 0.14.0+ AWS track | Defer. Existing examples are coherent and no 0.13.0 review found a blocker. |
| SQL dialect and extension fixtures: CockroachDB, ClickHouse, Trino, PostGIS, pgvector | These fixtures depend on SQL dialect breadth and PostgreSQL extension policy, not current package incoherence. | `README.md:101` lists current `sqlkit`; `docs/superpowers/research/2026-06-23-issue-218-db-storage-matrix.md:39-42,63-67` defers dialects/extensions until SQL design or a focused PostgreSQL extension issue. | P2/P3 | Later SQL dialect issues | Defer. A fixture before SQL API selection would be dependency-led rather than caller-led. |
| Graph backend adapters and fixtures: Memgraph, AGE, FalkorDB, TinkerGraph, Neptune | Neo4j is the selected proof adapter; other graph backends need compatibility, setup, or managed-service evidence. | `README.md:104-106` lists graph model, graphio, and Neo4j; `docs/superpowers/research/2026-06-30-issue-50-graph-backend-adapters.md:42-47` selects Neo4j, treats Memgraph as compatibility, defers AGE/FalkorDB, rejects local TinkerGraph, and keeps Neptune research-only; lines 70-80 record the rejection/defer rationale. | P2/P3 | #365/#366, later graph issues, or Backlog | Defer or reject per backend. No extra 0.13.0 feature work is justified. |
| Broad Kotlin/JVM parity helpers, global logger facade, future/executor facade, Apache Commons wrapper layer | These surfaces would add framework-shaped API without repeated Go caller evidence. | `README.md:290-295` requires Go-idiomatic APIs, small packages, and no wrappers around mature SDKs without a bluetape-specific reason; #427 non-goals explicitly exclude Kotlin extension-surface clones, JVM future/executor facades, Apache Commons wrapper layers, global logger facades, and broad helper packages. | P3 | None | Reject. Standard library or direct caller code is the Go-native path until concrete repeated call-site evidence exists. |

## 0.13.0 Must-Have Summary

Only one must-have feature gap was found: the MongoDB Testcontainers fixture
needed by the JWT Mongo repository test surface. That gap was implemented and
reviewed in #430 before this triage closed. No additional #427 implementation
work remains for 0.13.0 after #430.

## Later And Backlog Routing

| Bucket | Items | Reason |
|---|---|---|
| 0.14.0+ | Audit publisher adapters, AWS/storage emulators, SQL dialect fixtures, selected graph compatibility work | Valuable, but needs design/research or selected service semantics. |
| Later package-specific issue | Redis Cuckoo/HLL, PostGIS/pgvector, concrete HTTP mock fixture | Needs a package-local API contract and caller evidence. |
| Backlog / reject | Broad wrapper parity, global logger facade, JVM future/executor facade, local TinkerGraph adapter, Neptune local-fixture work | Go-native rules and existing research exclude these as 0.13.0 work. |

## 7-Tier Triage Verdict

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | No runtime code changed; deferred items would add Docker or SDK cost only when a package-specific issue justifies it. P0=0 P1=0. |
| Stability | PASS | The only P1 fixture gap was already closed by #430; no remaining shipped surface is incoherent or unsafe from current evidence. P0=0 P1=0. |
| Security | PASS | No auth, secret, logger, or trust-boundary gap was found. Broad logger/framework parity is rejected. P0=0 P1=0. |
| Operator/Ops | PASS | Testcontainers serial and emulator selection remain tied to concrete package tracks. P0=0 P1=0. |
| Developer/API | PASS | Decisions preserve Go-native, small-package boundaries and avoid wrapper catalogs. P0=0 P1=0. |
| User/Caller | PASS | Current shipped caller pain was the MongoDB fixture reuse gap, closed by #430. Other items need new caller evidence. P0=0 P1=0. |
| Integration | PASS | #426 output links the only 0.13.0 must-have to #430/#427 and routes later items without blocking 0.13.0. P0=0 P1=0. |

## Validation

- `gh issue view 426 --json number,title,state,milestone,labels,body`: issue
  scope and required matrix schema confirmed.
- `gh issue view 427 --json number,title,state,milestone,labels,body`: #427
  implementation boundary and non-goals confirmed.
- Repository evidence scans read the README, Testcontainers deferred scope,
  source-parity research, graph backend research, #429 review, #430 review,
  retrospective review, and stress coverage review.
- No implementation files changed in this issue.
- `git diff --cached --check`: PASS.
- `golangci-lint cache clean && make ci`: PASS. The cache clean was required
  because the first `make ci` lint pass reported stale diagnostics from the
  removed `issue-430-mongodb-testcontainer` worktree.

P0=0 P1=0.
