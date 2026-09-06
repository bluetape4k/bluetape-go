# 변경 기록

이 프로젝트의 주요 변경 사항을 이 파일에 기록합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따르며,
첫 태그를 배포한 뒤에는 시맨틱 버전 관리를 사용합니다.

## [Unreleased]

### 추가

- `graph/graphtest`에 backend-neutral semantic fixture, skip 불가 strict core,
  traversal capability, cancellation join, bounded cleanup/close와 redacted
  provider error 검증을 제공하는 공개 conformance harness를 추가한다.
- Neo4j와 Memgraph adapter가 digest-pinned Testcontainers 환경에서 같은
  core 및 traversal suite를 실행하도록 통합한다.

## [v0.21.0] - 2026-09-05

### 추가

- `web`에 RFC 9457 Problem Details 응답과 trusted request context helper를
  추가하고, `webtest`에 `net/http`, Gin, Echo middleware의 status, panic,
  cancellation, body lifecycle, request ID 및 global-state 격리를 검증하는
  conformance harness를 추가한다.
- `web/gin`과 `web/echo`에 request context, rate limit, strict JWT
  authentication, RFC 9457 Problem 및 route-level resilience adapter를 추가한다.
- `jwt/jwks` optional provider를 추가한다. 직접 지정한 JWKS JSON URL에서
  RSA/ECDSA/Ed25519 공개키를 context-aware로 조회하고 TTL, rotation,
  unknown `kid` cooldown, single-flight refresh, defensive copy를 제공한다.
  대칭 key와 JWE, OIDC discovery는 범위에서 제외한다.
- Gin adapter benchmark에 fixture/runtime identity, 회귀 임계값, 실제
  resilience policy 측정, bounded chart renderer watchdog 및 원자적인 실패
  진단 artifact를 추가한다.

### 변경

- `v0.20.0` tag tree에 이미 포함됐지만 해당 릴리스 변경 기록에서 분리해
  설명하지 못한 web helper, Gin/Echo adapter, middleware conformance 및 JWKS
  provider를 `v0.21.0` web API helper 릴리스 범위로 명시한다.
- Echo의 request context, JWT, rate-limit, resilience middleware는 nil
  downstream handler를 모두 HTTP 404로 종료한다.
- legacy `JWTOptions.Parser`가 construction-time에 context-aware capability를
  제공하면 이를 자동 사용하고, 명시적 `ContextParser` 설정을 우선한다.

### 버그 수정

- Echo rate-limit 기본 Problem 응답 쓰기가 실패하면 committed response를
  다시 쓰지 않고 redacted `OnWriteError` observer로 실패를 전달한다.
- legacy Echo JWT parser의 blocking 호출 중 request가 취소되어도
  context-aware provider 경로가 취소를 전달하고 in-flight 호출을 남기지 않게
  한다.

## [v0.20.0] - 2026-09-04

### 추가

- `examples/s3`에 S3 presigned GET/PUT, transfer manager, multipart cleanup,
  checksum 및 SSE-KMS 사용을 보여 주는 compile-checked 예제를 추가한다.
- `encrypt/kms`에 caller-owned AWS KMS client를 사용하는 AES-256 data-key
  envelope provider를 추가한다. strict canonical `BTKMS` envelope, bounded
  local AES-GCM payload, cancellation, redacted error 및 fake-client 검증을
  포함하며 key policy, rotation, IAM 및 data-key cache는 호출자가 소유한다.
- `audit/sqloutbox/eventbridge`에 stable event ID/idempotency key를 보존하고
  partial failure를 결정론적으로 매핑하는 EventBridge publisher adapter를
  추가한다.
- `workflow/stepfunctions`에 Start/Describe/선택적 Stop execution과
  cancellation-aware bounded polling bridge를 추가한다.
- `messaging/sqsextended`에 SQS와 S3를 조합하는 versioned large-payload
  envelope provider를 추가한다. object ownership, cleanup, checksum 및
  cancellation 경계를 호출자에게 노출한다.
- `examples/cloudwatch`에 CloudWatch Metrics와 Logs의 별도 batch,
  cancellation, redaction 및 현재 `PutLogEvents` 병렬성 계약을 보여 주는
  compile-checked 예제를 추가한다.
- `s3vectors`에 caller-owned AWS SDK client를 사용하는 얇은 vector bucket,
  index, put/get/list/query 표면과 fake/live-opt-in 검증을 추가한다.
- `secretsmanager`와 `ssm`에 caller-owned client 기반 lookup provider를,
  `rds/auth`에 RDS IAM database authentication token helper를 추가한다.
- `leader/dynamodb`에 조건부 쓰기, 명시적 lease deadline, owner token,
  renewal/resign 및 stale-owner takeover를 갖춘 Go-native leader provider를
  추가한다. TTL은 cleanup 힌트로만 사용한다.
- `redis` 공통 key/owner-token/lease/script/error 기반 위에 `redis/lock`,
  `redis/semaphore`, `redis/stream`, `redis/bucket`, `redis/mapcache`를
  추가하고, caller-owned context와 typed/redacted provider error를 보존한다.
- `cache/rediscoord/fory`와 `cache/redisfory`에 명시적 native-fast/native-
  compatible Apache Fory profile과 versioned binary envelope를 추가한다.
  `cache/redisvalue`는 bounded direct Redis value-cache 경계를 제공한다.

### 변경

- AWS adapter와 예제는 SDK client, credentials, IAM, provisioning, retry,
  timeout, connection 및 downstream idempotency를 caller/operator 책임으로
  유지하고, 기본 CI는 fake-client 또는 compile-checked 경로만 사용한다.
- Redis·DynamoDB·cache provider는 context cancellation, lease deadline,
  owner/fencing token, payload bound, cleanup 및 commit-unknown 의미를
  명시적으로 검증하고 raw provider error를 redaction한다.
- Fory provider는 `xlang=false`, 고정 profile/schema, trusted-internal 입력과
  의도적인 schema rotation을 요구하며 JSON 기본값과 기존 coordination
  envelope를 변경하지 않는다.
- leader, JWT, graph, textsearch, image/example, database, audit, crypto,
  AWS, Redis, cache, lock, rate-limit, probabilistic 및 Testcontainers 공개
  Go doc 주석을 한국어로 정비한다. README와 LLM-facing 운영 문서는 기존
  언어·범위를 유지한다.

## [v0.19.0] - 2026-08-06

### 추가

- `leader/leadertest`, `lock/locktest`, `ratelimit/ratelimittest`에 mandatory
  public provider conformance runner를 추가한다. in-memory reference fixture와
  Redis/Mongo/local provider adoption을 포함한다.
- caller-owned `*sql.DB`와 `public.bluetape_leader_leases` row lease 위에서
  동작하는 PostgreSQL-only `leader/sql` single-elector provider를 추가한다.
  mandatory `leader/leadertest` conformance, Testcontainers fault recovery,
  least-privilege role proof, bilingual operations guidance, 검증된 row-lease
  sequence diagram을 포함한다.
- caller-owned etcd v3 client와 official Session/Election primitive 위에서
  동작하는 `leader/etcd` single-leader election을 추가한다. server-granted
  TTL, exact-key/Proclaim fail-closed monitoring, mandatory conformance,
  authenticated range와 lease-revoke proof, bilingual shutdown guidance를
  포함한다. 이 provider는 fencing token을 제공하지 않으며 TTL 경과만으로 remote
  cleanup을 증명하지 않는다.
- moderate-QPS database-only deployment를 위한 `ratelimit/sql` PostgreSQL
  atomic token bucket을 추가한다. caller가 fixed schema, `*sql.DB`, bounded
  cleanup scheduler를 소유하며, Redis는 high-QPS 선택지로 남는다. Redis와 SQL
  failure는 `ratelimit.OperationError`와 `ErrCommitUnknown` inspection을
  공유하고, commit-unknown debit은 자동 replay하면 안 된다.
- caller-owned transaction 하나에서 batch callback과 consumed-input progress를
  함께 commit하는 PostgreSQL durable checkpoint package `batch/sqlcheckpoint`를
  추가한다. revision CAS는 competing writer를 거부하며, commit-unknown은 fresh
  bounded load만 허용하고 `ErrAtomicityUnknown`은 replay 전 quiescing과 manual
  reconciliation을 요구한다.
- explicit profile, BTFV envelope, TTL, schema generation key isolation을 갖춘
  bounded Go-native Apache Fory value를 Redis에 직접 저장하는 `cache/redisfory`를
  추가한다.
- bounded generic serialized Redis L2 value와 reference-preserving process-local
  tiered decorator를 제공하는 `cache/redisvalue`를 추가한다. RESP3-coherent
  invalidation은 제외하고 별도로 추적한다.
- key, owner-token, lease script, TTL, redacted Redis operation error primitive를
  담은 `redis` foundation package를 추가한다.

### 변경

- Redis와 Mongo 전반에서 single-leader campaign waiting, local-state sentinel,
  typed provider failure, commit-unknown cleanup을 통합한다.
- `leader/sql` migration은 fixed `public` relation을 요구하고 mutation,
  observation, reconciliation probe를 하나의 writable primary로 보낸다.
  indeterminate cleanup은 full-lease expiry fallback 전에 같은 elector에서 bounded
  `Resign`을 retry한다. v0.19.0 provider는 fencing, custom schema, group
  election, strategic election을 지원하지 않는다.
- mixed-version constraint, telemetry label, canary threshold, resign/TTL rollback
  completion gate를 포함한 bilingual [v0.19.0 provider rollout runbook](docs/release/v0.19.0-provider-conformance-runbook.md)을
  publish한다.
- nonblank custom Redis lock token byte를 trimming 없이 보존한다. Lock caller는
  non-nil lease와 `redis.ErrCommitUnknown`이 함께 반환될 수 있음을 처리하고,
  같은 release callback을 retry한 뒤 TTL fallback을 사용해야 한다. Redis
  rate-limit caller는 commit-unknown request를 replay하지 말고 full refill
  interval을 기다리거나 가능한 debit 하나를 accounting해야 한다.

## [v0.18.0] - 2026-07-10

### 추가

- bounded slot lease document, concurrent acquisition에서의 exact `MaxLeaders`
  admission, renewal-loss detection, Testcontainers stress coverage, bilingual
  README documentation을 갖춘 `leader/mongo` group leader elector backend를
  추가한다.
- MongoDB candidate registry document, FIFO/random/scored strategy execution,
  atomic result update, stale-candidate pruning, Testcontainers stress coverage,
  bilingual README documentation을 갖춘 `leader/mongo` strategic leader elector
  backend를 추가한다.
- directed property graph subset을 위한 optional bounded GraphML import/export
  package `graph/graphio/graphml`을 추가한다. scalar key/data attribute, explicit
  XML input limit, fail-closed unsupported construct test, bilingual README
  documentation을 포함한다.
- narrow `XADD` client surface, stable sqloutbox event/idempotency metadata,
  Testcontainers-backed duplicate attempt와 relay retry coverage, bilingual
  README documentation을 갖춘 `audit/sqloutbox/redisstreams` Redis Streams
  publisher provider를 추가한다.

## [v0.17.0] - 2026-07-09

### 추가

- caller-owned MongoDB collection, owner-token lease document, optional TTL
  cleanup index support, renewal-loss detection, contention test, bilingual
  README coverage를 갖춘 `leader/mongo` single leader elector backend를 추가한다.

### 변경

- root/package README documentation은 source-checked workshop adoption example,
  active cross-repo workshop issue, 0.17.0 workshop adoption release-readiness
  note를 link한다.
- `resilience` README guidance는 official application-level `otelslog` bridge
  path를 명시하면서 OpenTelemetry exporter를 `bluetape-go` library package 밖에
  둔다.

## [v0.16.0] - 2026-07-08

### 추가

- `probabilistic/redis`에 Redis HyperLogLog support를 추가한다. `NewHyperLogLog`,
  `NewStringHyperLogLog`, `NewBytesHyperLogLog` constructor, SHA-256 value digest,
  `Add`, `Count`, `Merge` operation, example, bilingual README coverage를 포함한다.
- Bloom filter와 HyperLogLog에 대한 Testcontainers-backed Redis probabilistic
  coverage를 추가한다. bounded container startup, live Redis cleanup,
  cancellation check, stress coverage, race validation evidence를 포함한다.
- Redis Cuckoo와 HyperLogLog support에 대한 research/release lesson을 추가한다.
  첫 follow-up structure로 core Redis HLL을 선택하고 RedisBloom `CF*` Cuckoo
  support는 module-gated 상태로 둔다.

### 변경

- `probabilistic/redis` README documentation과 runtime diagram은 현재 core Redis
  Bloom/HLL assumption과 future RedisBloom module Cuckoo support를 분리한다.
- 0.16.0 Redis probabilistic work를 계속하기 전에 root release state를 `v0.15.0`
  main release tree와 reconcile한다.

## [v0.15.0] - 2026-07-08

### 추가

- `audit/sqloutbox`의 audit publisher adoption 트랙을 추가한다. documented
  `Publisher` retry contract, stable `Record.EventID`와
  `Record.IdempotencyKey` handoff guidance, duplicate-safe at-least-once delivery
  example을 포함한다.
- relay test, example, retry assertion, duplicate-delivery evidence를 위한
  deterministic `DiscardPublisher`, `PublisherFunc`, goroutine-safe
  `RecordingPublisher` helper를 제공하는 `audit/sqloutbox/sqloutboxtest`를
  추가한다.
- 첫 audit publisher adapter target에 대한 research와 lesson을 추가하고, Kafka,
  NATS, Redis Streams 등 durable transport adapter보다 standard-library
  test/example helper를 먼저 선택한다.
- 0.15.0 SerDe 후속 트랙에서 JSON repeated-collection decoding과 zstd
  compression allocation cost profiling evidence를 보존한다.

### 변경

- `serialization.JSONSerializer`는 default decode path에서 `json.Unmarshal`을
  사용하면서 strict trailing-payload rejection과 `WithDisallowUnknownFields`
  behavior를 decoder path로 보존한다.
- `compression.Zstd().Compress`는 byte-slice compression에 internal zstd stream
  encoder를 재사용하면서 `NewWriter`는 caller-owned와 independent 상태로 유지한다.
- root/package README guidance는 sqloutbox test publisher를 link하고 durable
  broker topology를 약속하지 않은 채 audit publisher adoption boundary를 기록한다.

## [v0.14.0] - 2026-07-07

### 추가

- `serialization`, `codec`, `compression`을 위한 cross-repo SerDe/compression
  benchmark baseline을 추가한다. shared fixture/scenario definition, Go benchmark
  runner, raw `-benchmem` output, environment metadata를 포함한다.
- Go, Rust, JVM serialization/compression behavior를 비교하는 evidence-scoped
  recommendation matrix를 추가하고 measured evidence와 follow-up hypothesis를
  분리한다.
- 재현 가능한 future benchmark report를 위한 benchmark artifact retention
  template과 issue-specific output directory를 추가한다.

### 변경

- root, serialization, codec, compression, research README는 production ranking
  claim 대신 0.14.0 benchmark snapshot과 raw evidence를 link한다.
- benchmark runner는 timing 전에 round-trip behavior를 검증하고 downstream
  analysis를 위한 deterministic scenario name을 포함한다.

## [v0.13.0] - 2026-07-07

### 추가

- 0.1.0부터 0.12.0까지 release-readiness retrospective audit을 추가한다. 기록된
  7-tier review evidence, final P0/P1 count, deferred P2/P3 routing, release
  preflight state를 포함한다.
- 기존 concurrency, resilience, DynamoDB batchwrite, testing-helper contract에
  빠져 있던 stress/async cancellation coverage를 추가하고 race-detector
  validation을 포함한다.
- caller-owned MongoDB client와 environment-exportable connection detail을 갖춘
  Testcontainers for Go 기반 reusable MongoDB integration fixture package
  `testcontainers/mongodb`를 추가한다.

### 변경

- cumulative lesson hardening은 Testcontainers, cache, Redis coordination, JWT
  documentation 전반에 bounded cleanup context와 errcheck-shaped cleanup example을
  기록한다.
- feature-gap triage는 후속 audit, probabilistic, messaging, AWS, SQL, graph,
  HTTP fixture idea를 0.13.0 릴리스 train을 막지 않는 방식으로 분류한다.

### 버그 수정

- `cache.Memory.GetOrLoad`는 same-key caller cancellation isolation을 보존하여
  late canceled loader result를 cache에 쓰지 않는다.
- `ratelimit/redis`는 distinct key를 같은 Redis storage key로 normalize하지 않고
  caller-owned key를 보존한다.

## [v0.12.0] - 2026-07-06

### 추가

- `core`, `collections`, `codec`, `concurrency`, observability convention,
  rule-engine boundary에 대해 source-backed Go-native decision을 갖춘 core
  foundation parity pass를 추가하고 JVM-shaped broad helper surface를 명시적으로
  거부한다.
- blank check, string predicate, canonical UUID parsing/rendering, 좁은
  caller-owned text utility behavior를 위한 `core` string validation과 UUID
  helper를 추가한다.
- copied-output behavior, deterministic example, table-driven coverage를 갖춘
  작은 slice-oriented primitive용 `collections` helper를 추가한다.
- non-canonical 또는 oversized alias를 거부하고 round-trip compatibility evidence를
  보존하는 `codec` canonical UUID URL62 helper를 추가한다.
- goroutine-safe selection behavior, deterministic example, stress coverage, race
  validation을 갖춘 `concurrency` round-robin primitive를 추가한다.
- immutable fact, deterministic rule execution, composite rule, bounded inference,
  typed non-convergence error, YAML/JSON expression-backed reader, bilingual
  README diagram을 갖춘 first-party `rules` package primitive를 추가한다.
- 누락된 package docs를 위한 package README diagram coverage를 paired SVG/PNG
  asset과 visual/audit review evidence로 추가한다.

### 변경

- public example과 package-local hook은 global bluetape-go logger registry를
  추가하지 않고 caller-owned `log/slog` pattern을 사용한다.
- root/package README는 0.12.0 rule/core foundation scope를 설명하고 Korean docs를
  English package behavior와 정렬한다.

## [v0.11.0] - 2026-07-03

### 추가

- dependency-light pure-Go resize, thumbnail, format conversion, bounded image
  decode/encode limit, explicit option validation, benchmark evidence, README
  usage docs, checked transform-flow diagram을 갖춘 `imagekit` package를 추가한다.
- caller가 core module dependency로 `govips`를 추가하지 않고 libvips-backed image
  processing을 둘 수 있는 위치를 증명하는 optional
  `examples/imagekit-govips` adapter를 추가한다.
- random nonce generation, AAD-bound authenticated encryption,
  nonce/ciphertext framing, key-size validation, tamper/error coverage를 갖춘
  stdlib AES-GCM facade `encrypt` package를 추가한다.
- Neo4j-driver client option, graph value conversion, redacted connection/query
  error, bilingual package docs, Memgraph compatibility test를 갖춘 `graph/neo4j`
  adapter proof를 추가한다.
- principal, role, policy, resource edge, bounded path analysis, root README link,
  source-backed architecture diagram을 갖춘 runnable IAM access graph example
  `examples/graph/iamaccess`를 추가한다.
- JVM shape를 import하지 않고 Go-style evaluation boundary를 증명할 수 있을 때까지
  rule execution을 core 밖에 두는 rule-engine primitive research를 추가한다.

## [v0.10.0] - 2026-07-01

### 추가

- graph I/O helper와 example을 위한 model-only vertex, edge, path, label, ID,
  shallow property, validated JSON value를 갖춘 `graph` package를 추가한다.
  graph repository/session/schema/query/transaction/backend contract는 follow-up
  I/O, backend, example issue가 shared behavior를 증명할 때까지 defer한다.
- stream-oriented NDJSON와 paired CSV import/export helper를 제공하는
  `graph/graphio` package를 추가한다. bounded read default, duplicate/missing
  endpoint policy, CSV formula escaping, redacted error, stateful reader/writer
  API를 포함한다.
- Neo4j adapter proof를 먼저 선택하고 Memgraph를 Neo4j-driver compatibility
  coverage로 routing하며 AGE, FalkorDB, TinkerPop/TinkerGraph, Neptune은 Go
  driver 또는 local-test boundary가 증명될 때까지 defer하는 graph backend adapter
  feasibility research를 추가한다.
- seed data, blast-radius query, alert-boundary/ownership lookup, NDJSON graph
  I/O round-trip coverage, bilingual README docs, topology diagram을 갖춘 runnable
  incident-response graph example `examples/graph/observability`를 추가한다.

## [v0.9.0] - 2026-06-29

### 추가

- aggregate ID, monotonic revision, caller-owned domain event ID, idempotency key,
  validated JSON audit entry, pending event recorder, storage-neutral history
  reconstruction, repository/query interface, reusable adapter conformance test,
  goroutine-safe non-durable in-memory repository를 갖춘 `audit` package를 추가한다.
- durable publisher의 첫 target으로 SQL outbox store와 relay contract를 선택하는
  audit outbox design을 추가한다. durable outbox boundary가 증명될 때까지 Kafka,
  NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar, direct Redis audit storage는
  defer한다.
- PostgreSQL-backed enqueue, claim, claim-attempt-guarded publish/failure marking,
  claim lease, retry/dead-letter state, per-aggregate claim ordering,
  context-cancellable at-least-once relay를 갖춘 `audit/sqloutbox` package를
  추가한다.
- aggregate change, audit repository history query, in-memory outbox replay
  boundary를 보여 주는 runnable order-service recipe `examples/audit`를 추가한다.

## [v0.8.0] - 2026-06-27

### 추가

- immutable Aho-Corasick multi-pattern matcher, first/all match mode, overlap
  policy, Unicode normalization, word-boundary filtering, replacement, masking,
  concurrency stress coverage를 갖춘 `textsearch` package를 추가한다.
- severity metadata, deterministic detection/masking response, static rebuild
  semantics, Korean/Japanese/ASCII stress coverage를 갖춘 `textsearch` blockword
  dictionary를 추가한다.
- byte-span token, normalized text helper, coarse POS extension point, dictionary
  provider, dependency-free deterministic tokenizer를 갖춘 `textsearch` tokenizer
  core interface를 추가한다.
- IPA dictionary default, byte-span preservation, Kagome POS metadata, noun/verb
  filter, blockword example, goroutine stress coverage를 갖춘 optional
  `textsearch/japanese` Kagome v2 adapter를 추가한다.
- all/subset detector builder, lazy/preloaded/low-accuracy mode,
  mixed-language section, Unicode script helper, goroutine stress coverage를 갖춘
  optional `textsearch/language` Lingua-Go adapter를 추가한다.

## [v0.7.0] - 2026-06-26

### 추가

- runtime-first `database/sql` transaction helper, 작은 `Session`/`Queryer`/
  `Execer` interface, explicit row mapping helper, cardinality-aware `QueryAll`,
  `QueryOptional`, `QueryOne` function을 갖춘 `sqlkit` package를 추가한다.
- copied argument slice, validated quoted identifier, full-table update/delete
  guard, context-aware `Statement.Exec`을 포함한 PostgreSQL-first inspectable SQL
  builder for `SELECT`, `INSERT`, `UPDATE`, `DELETE`를 추가한다.
- `sqlkit`을 통한 create/read/update/delete/rollback/relational query behavior를
  다루는 Testcontainers-backed PostgreSQL repository example을 추가한다.
- direct `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM, goqu, Atlas 선택
  기준을 문서화하는 SQL generator/migration guidance를 추가한다. sqlc, Jet,
  Atlas는 core runtime dependency boundary 밖에 둔다.

### 변경

- root README와 Korean README는 `sqlkit`을 active data-access package로 나열하고
  optional SQL generator/migration guide를 link한다.
- 0.7.0 relational SQL epic은 #100의 runtime-first direction을 기록하고 mandatory
  code generation, broad ORM behavior, hidden migration, cross-database abstraction을
  첫 package slice 밖에 둔다.

## [v0.6.8] - 2026-06-25

### 추가

- untrusted compressed payload를 다루고 기존 `Compressor` interface를 바꾸지 않은
  채 expanded-output hard limit이 필요한 caller를 위해
  `compression.DecompressLimit`와 `ErrDecompressedSizeExceeded`를 추가한다.
- public helper API에서 caller-input validation failure를 표현하는
  `core.ErrInvalidArgument`와 `collections.ErrInvalidArgument` sentinel contract를
  추가한다.

### 변경

- root README release status는 published `v0.6.7` 릴리스 train과 MongoDB-backed JWT
  KeyChain repository scope를 반영한다.
- Redis leader와 lock example은 bounded campaign/acquire context와 별도 bounded
  cleanup context를 사용한다.
- AWS S3, SQS/SNS, DynamoDB batchwrite example은 bounded context를 보여 주고 SDK
  error를 버리지 않는다.
- Docker-backed test는 PostgreSQL, MySQL, MariaDB, NATS, Redis Bloom, JWT
  Redis/Mongo fixture에서 explicit startup context를 사용한다.

### 버그 수정

- ECB exchange-rate XML fetch는 XML decoding 전에 response body를 제한한다.
- MongoDB JWT repository trim cursor cleanup은 bounded cleanup context를 사용한다.
- Redis leader와 group elector `Resign`은 renewal worker를 기다리는 동안 caller
  cancellation을 존중하고 renewal Redis call을 operation별로 제한한다.
- Redis near-cache `Close`는 `OnError` reporter goroutine을 추적하고 bounded
  shutdown failure를 surface한다.

## [v0.6.7] - 2026-06-25

### 추가

- MongoDB-backed distributed JWT key-chain storage를 위한 `jwt.MongoRepository`와
  `jwt/mongo` facade를 추가한다. shared-provider rotation, `kid` lookup, capacity
  trimming, expiry handling, cancellation, Testcontainers MongoDB coverage를
  포함한다.

## [v0.6.6] - 2026-06-25

### 추가

- developer experience parity를 위한 focused testing fixture example, assertion
  pattern, golden-file data, bilingual testing README update를 추가한다.
- Go standard-library behavior가 더 명확한 곳에서는 이를 우선한다는 logging,
  time, math helper utility parity boundary documentation을 추가한다.
- batch, workflow, cache, resilience, id, JWT, Redis lock/leader, Testcontainers
  Redis 전반의 `examples/integration` recipe를 추가한다. service-free, race,
  Docker-backed smoke command를 포함한다.
- rechecked 0.6.x parity matrix, `P0=0 P1=0` state, deferred follow-up, explicit
  Go non-goal을 문서화하는 corrective-series closure audit를 추가한다.

### 변경

- root README release roadmap은 completed corrective 0.6.3부터 0.6.6 series를
  반영하고 closed parity hardening과 later roadmap work를 분리한다.

## [v0.6.5] - 2026-06-25

### 추가

- bounded startup error reporting과 service connection metadata helper를 갖춘
  shared Testcontainers server/property export abstraction을 추가한다.
- S3, SQS, SNS, DynamoDB를 위한 service config smoke coverage와 함께 MariaDB,
  Toxiproxy, Floci Testcontainers wrapper를 추가한다.
- S3, SQS/SNS, DynamoDB batch write retry helper를 위한 direct AWS SDK for Go
  example을 추가한다. bilingual README coverage와 explicit wrapper boundary
  decision을 포함한다.

### 변경

- 더 많은 service fixture를 추가하기 전에 기존 Testcontainers lifecycle과
  connection contract를 harden한다. serial execution guidance와 cleanup/startup
  diagnostic을 포함한다.

## [v0.6.4] - 2026-06-25

### 추가

- context-aware timeout behavior, interval control, example, focused test를 갖춘
  `testing` async await/polling helper를 추가한다.
- caller-owned cancellation behavior example과 success/failure helper를 포함한
  context-aware API용 `testing` cancellation contract assertion을 추가한다.
- cleanup coverage와 bilingual README documentation을 갖춘 scoped temporary output
  및 environment helper를 추가한다.
- current milestone에서 broad fixture dependency를 거부하는 random data parameter
  source와 test reporting helper research note를 추가한다.

### 변경

- `testing/concurrency` helper reporting을 harden하여 stress failure가
  race-compatible execution을 약화하지 않고 caller-visible evidence를 보존하게 한다.

## [v0.6.3] - 2026-06-25

### 추가

- Go-native API, table-driven test, bilingual README coverage를 갖춘
  `collections` bounded stack, ring buffer, pagination, permutation helper를
  추가한다.
- Go standard library가 더 단순한 contract를 제공하지 않는 영역에서
  bluetape4k-core에서 영감을 받은 `core` range helper, wildcard matching, XXH64
  hashing, resource-style helper documentation, quarter/time helper를 추가한다.

### 변경

- invalid UTF-8, nil/empty, malformed input, documentation boundary를 포함해
  `core`, `collections`, `codec`, `serialization` text/binary contract를 harden한
  뒤 더 많은 foundation parity API를 추가한다.

## [v0.6.2] - 2026-06-21

### 추가

- IMF Exchange Rates SDMX-backed reference rate provider `money.NewIMFProvider`를
  추가한다. configurable period-average/end-of-period family, frequency, cache와
  stale fallback metadata, USD/EUR domestic pivot support, cancellation test,
  bilingual README/research documentation을 포함한다. currency backend가 XDR 값을
  안전하게 구성할 수 있을 때까지 SDR/XDR exposure는 defer한다.
- `money`를 위한 Bloomberg-backed exchange-rate provider evaluation을 추가한다.
  SAPI, B-PIPE, Data License, BLPAPI, entitlement, credential, freshness,
  failure-mapping, test-strategy boundary를 문서화하고 Bloomberg dependency와 paid
  access는 default `money` behavior 및 CI 밖에 둔다.

## [v0.6.1] - 2026-06-21

### 추가

- Redis-backed shared Bloom filter package `probabilistic/redis`를 추가한다.
  Cluster-safe hash-tagged key pair, immutable config metadata, static Lua bitmap
  operation, cancellation/race/stress coverage, compile-checked example,
  bilingual README/runbook documentation을 포함한다. Redis-backed Cuckoo와
  HLL/HyperLogLog constructor는 #182 이후 follow-up scope로 둔다.
- `NewCachedProvider`, `NewCachedDistributedProvider`, scoped token-digest cache
  key, trusted `cache.Cache[string,*jwt.Reader]` backend, warm-hit key
  revalidation, same-key miss coalescing, cancellation/race/stress coverage,
  compile-checked example, diagram-backed bilingual README documentation,
  process-local clear scope와 unsupported untrusted shared/external cache에 대한
  operator caveat를 갖춘 optional JWT provider cache adapter를 추가한다.
- `ExchangeRateProvider`, `ConvertWithProvider`, `NewECBProvider`,
  caller-visible source/freshness/stale fallback metadata, cancellation/retry/cache
  coverage, stress/race test, diagram-backed bilingual README documentation을
  갖춘 `money` provider-backed exchange-rate conversion을 추가한다. IMF와 Bloomberg
  provider는 follow-up issue #231, #232로 남긴다.
- explicit-region BCP47 tag에 대한 `money.CurrencyByLocale` CLDR-backed locale
  currency mapping을 추가한다. missing/no-tender/multi-tender rejection,
  stress/race coverage, diagram-backed bilingual README documentation을 포함한다.
- raw benchmark output, chart-backed bilingual README guidance, `Money`,
  `NewMinor`, `MinorUnits`를 public minor-unit path로 유지한다는 decision을 갖춘
  `money` FastMoney evaluation benchmark evidence를 추가한다.

### 변경

- #174 JWT compression/JOSE decision을 문서화한다. signed JWT compression은 현재
  `jwt` helper의 non-goal이고, `zip=DEF`는 future explicit JWE boundary에 속하며,
  optional JWE scope가 구현된다면 `go-jose/go-jose/v4`가 preferred candidate다.

## [v0.6.0] - 2026-06-09

### 추가

- repo-owned UUID v4/v7 string generator, random/monotonic ULID generator,
  standard seconds-precision KSUID generation, parsing, timestamp extraction,
  Snowflake int64 generation/decoding, sentinel/typed error contract,
  stress/race coverage, benchmark smoke, bilingual package README coverage를 갖춘
  `id` package를 추가한다. Kotlin-compatible millisecond KSUID는 #171로 defer한다.
- explicit HS/RS/PS algorithm provider, fixed/in-memory rotating KeyChain, typed
  claim/header reader, issuer/subject/audience/exp validation helper, `kid`
  lookup, weak-secret rejection, unsupported JOSE header rejection, sentinel
  error contract, stress/race coverage, bilingual package README coverage를 갖춘
  `jwt` package를 추가한다. distributed repository, JOSE compression, provider
  cache adapter는 #173, #174, #175로 defer한다.
- typed `Unit[D]`와 `Measure[D]`, built-in length/time/mass/area/volume/storage/
  binary size/frequency/energy/power/pressure/angle/graphics length/velocity/
  acceleration/affine temperature, generic/family parser, compound unit helper,
  source-parity named helper, sentinel error contract, stress/race coverage,
  bilingual README coverage를 갖춘 `measure` package를 추가한다. Decimal money
  precision은 future money package로 defer한다.
- ISO 4217 currency wrapper, decimal-backed `Money` value, same-currency
  arithmetic, half-even rounding, minor-unit helper, JSON/text serialization,
  caller-supplied `ExchangeRate` conversion, typed sentinel error,
  goroutine stress/race coverage, bilingual package README coverage를 갖춘 `money`
  package를 추가한다. provider-backed exchange rate, full locale mapping,
  separate long-backed FastMoney는 #178, #179, #180으로 defer한다.
- goroutine-safe in-memory Bloom filter, deterministic config sizing, SHA-256
  double hashing, explicit generic hasher key, compatible filter merge,
  false-positive/no-false-negative contract test, sentinel error,
  stress/race coverage, opt-in benchmark smoke, bilingual package README coverage를
  갖춘 `probabilistic` package를 추가한다. Redis-backed Bloom, Cuckoo,
  HyperLogLog는 #182로 defer한다.

## [v0.5.1] - 2026-06-08

### 버그 수정

- `SkipPolicy`와 일치하는 checkpointed `batch.Step` writer failure는 unsafe
  skipped writer chunk 이후 checkpoint를 advance하지 않고
  `ErrUnsafeWriterSkipCheckpoint`로 실패한다. restart는 마지막 safe checkpoint부터
  replay하고 `errors.Is` check를 위해 original writer error를 보존한다.

## [v0.5.0] - 2026-06-08

### 추가

- first-party reader/processor/writer chunk step, sequential job, report,
  filtering, context cancellation, resource cleanup, stress/cancellation coverage를
  갖춘 `batch` package를 추가한다.
- processor/write failure를 위한 batch retry와 skip policy를 추가한다. explicit
  context-cancellation preservation과 retry/skip count reporting을 포함한다.
- `CheckpointReader`, `CheckpointStore`, in-memory checkpoint storage, restart
  coverage, committed progress 이후 checkpoint persistence를 갖춘 pluggable
  checkpoint support를 추가한다.
- current Redis leader 아래에서만 scheduled batch work와 migration workload를
  실행하는 leader-guarded batch example을 `leader/redis`에 추가한다.
- leader-guarded batch example을 위한 runnable Redis Testcontainers command와
  bilingual README coverage를 추가한다.
- batch retry/skip policy와 checkpoint restart scope를 보여 주는 README
  architecture diagram refresh를 추가한다.

### 변경

- root README architecture asset은 completed 0.5.0 batch recovery scope를 반영한다.
- WIP와 release guide는 0.5.0 release-preparation state를 반영한다.

## [v0.4.0] - 2026-06-06

### 추가

- first-party finite state machine primitive, explicit transition, context-aware
  guard, final state, deterministic transition error, stress/cancellation coverage,
  compile-checked example을 갖춘 `state` package를 추가한다.
- workflow status value, failure policy, report tree, deterministic aggregation,
  zero-value safety check, stress/cancellation coverage, compile-checked example을
  갖춘 `workreport` package를 추가한다.
- `context.Context`와 `workreport` 기반 sequential, conditional, all-branches
  parallel runner를 제공하는 `workflow` package를 추가한다. cancellation, stress,
  race, compile-checked example coverage를 포함한다.
- `state`, `workreport`, `workflow`에 필요한 race-compatible coverage를 문서화하는
  0.4.0 stress/cancellation gate를 추가한다.
- 0.4.0 `state`, `workreport`, `workflow` package surface를 위한 package README
  coverage와 root README index를 추가한다.
- 0.4.0 `state`, `workreport`, `workflow` API용 compile-checked runnable example
  link를 package README에 추가한다.
- 0.4.0 workflow primitive와 complex Redis coordination package를 위한 README
  diagram asset을 추가한다. PNG-only README embed와 adjacent SVG source를 포함한다.

### 변경

- 모든 package-level `README.md`는 sibling `README.ko.md`와 일관된
  `English | 한국어` language switch를 갖는다.
- root README, WIP, release guide는 closed `0.4.0` milestone과 `v0.4.0`
  release-preparation state를 반영한다.

## [v0.3.0] - 2026-06-05

### 추가

- generic cache interface, process-local TTL `Memory`, `ErrCacheMiss`,
  context-aware loader, `GetOrLoad` same-key stampede protection을 갖춘 `cache`
  package를 추가한다.
- Redis Pub/Sub invalidation for process-local loading cache를 제공하는
  `cache/redisnear` package를 추가한다. close semantics, malformed-message
  reporting, Testcontainers peer invalidation coverage, stress testing,
  cancellation coverage를 포함한다.
- single-Redis-instance owner-token locking, TTL acquisition, owner-safe Lua
  unlock, Testcontainers contention/expiration coverage, stress/cancellation
  test를 갖춘 `lock/redis` package를 추가한다.
- cross-process cache stampede protection을 위한 opt-in Redis coordination
  package `cache/rediscoord`를 추가한다. owner-token load lease, short-lived
  shared result envelope, Testcontainers NearCache collapse coverage,
  lease-expiry test, stress/cancellation test를 포함한다.
- local/Redis-backed token-bucket limiting, HTTP middleware, Redis Lua atomic
  consume/refill, Testcontainers concurrency coverage, stress/cancellation test,
  local benchmark coverage를 갖춘 `ratelimit`와 `ratelimit/redis` package를
  추가한다.
- FIFO, seed-stable random, scored strategy를 갖춘 candidate-registry leader
  election을 위한 `leader` strategy API와 Redis-backed
  `redisleader.NewStrategic`을 추가한다. Testcontainers coverage와
  stress/cancellation test를 포함한다.
- `GoroutineStressTester`와 `AsyncJobTester`를 사용한 cache stress/cancellation
  coverage와 zero-value `Memory` safety coverage를 추가한다.
- initial cache contract를 위한 Type A research, spec, plan, review, lesson
  artifact를 추가한다.
- native coverage profile, package subtotal summary, function-level text summary,
  HTML report, GitHub Step Summary output, uploaded workflow artifact를 통해 CI와
  Nightly용 Go coverage reporting을 추가한다.

### 변경

- package documentation은 package-level `README.md`에 위치하고 root README는 link가
  있는 high-level index로 남는다.
- README와 WIP documentation은 completed `0.3.0` 릴리스 train, merged package
  surface, open cache/coordination follow-up issue를 반영한다.
- `make bench-ratelimit`는 opt-in local rate limiter benchmark run을 노출한다.

## [v0.2.0] - 2026-06-04

### 추가

- ZSET slot token을 사용하는 semaphore-style multi-leader election을 위해
  `leader.GroupElector`와 Redis-backed `redisleader.NewGroup`을 추가한다.
- first-party `resilience` package에 circuit breaker와 bulkhead policy를 추가한다.
- stable policy type, event category, error category, retry attempt, circuit
  transition, timeout, bulkhead data를 갖춘 structured resilience event를 추가한다.
- resilience policy를 `net/http`와 compose하기 위한 HTTP client/server adapter를
  추가한다.
- reusable Redis fixture를 위한 Redis Testcontainers smoke coverage를 추가한다.

### 변경

- README example은 retry, timeout, circuit breaker, bulkhead, observability hook,
  HTTP adapter, leader group election을 보여 준다.

## [v0.1.1] - 2026-06-03

### 추가

- composable typed policy, retry, timeout, deterministic backoff, event hook,
  example을 갖춘 initial first-party `resilience` package를 추가한다.
- research, spec, plan, 7-tier review, lesson artifact를 포함한 `0.1.0` foundation
  surface의 retrospective milestone evidence를 추가한다.

### 버그 수정

- JSON deserialization은 첫 valid JSON value 이후 trailing payload를 거부한다.

## [v0.1.0] - 2026-06-03

### 추가

- `core`, `testing`, `testcontainers/redis`, `leader`, `leader/redis` package를
  갖춘 initial Go module을 추가한다.
- Testcontainers smoke coverage를 갖춘 Redis-backed leader election을 추가한다.
- `docs/research/` 아래 milestone research note를 추가한다.
- roadmap, hero image, architecture overview diagram을 갖춘 English/Korean README
  file을 추가한다.
- `Makefile`, lint configuration, WIP log, package layout policy, release guide 등
  project management scaffolding을 추가한다.
- scheduled smoke/full cadence로 Testcontainers-backed test를 실행하는 Nightly
  workflow를 추가한다.
- validation, zero/default handling, pointer, string, small numeric check를 위한
  core support helper를 추가한다.
- chunking, grouping, distinct value, error-aware slice transformation을 위한
  collections helper를 추가한다.
- duplicate campaign, repeated resign, renewal loss, renewal failure, leader
  lookup semantics에 대한 Redis leader lifecycle test를 추가한다.
- `core`, `collections`, `codec`, `compression`, `concurrency`, `serialization`,
  `testing/concurrency` package용 testable Go example을 추가한다.
- PostgreSQL, MySQL 8.4, NATS, Kafka Testcontainers fixture와 smoke test를
  추가한다.
- eventual/consistent condition을 위한 Gomega-backed asynchronous test helper를
  추가한다.
- batch scheduling과 migration gate를 위한 Redis leader coordination example을
  추가한다.
- Kotlin/Go mixed participant를 위한 Redis leader key compatibility decision을
  추가한다.

### 변경

- CI는 real Testcontainers dependency를 대상으로 formatting, module tidiness,
  vet, lint, test, race test를 검증한다.
- `make test`와 `make race`는 integration test가 Go test cache 때문에 skip되지
  않도록 `-count=1`을 전달한다.
- `leader` API docs는 ownership, cancellation, idempotent resign, lost-leadership,
  `errors.Is` comparison semantics를 정의한다.
