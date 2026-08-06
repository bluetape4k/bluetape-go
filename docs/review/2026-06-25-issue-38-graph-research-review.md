# Issue 38 Graph Research Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-25
범위: issue #38 research note and downstream graph issue updates for #44 and #48-#51.

## 판정

P0: 0
P1: 0

This is a documentation and tracker-alignment change. It does not add Go package code, exported APIs, dependencies, benchmark claims, or runtime behavior.

## 7-Tier 검토

### 성능

P0: 0
P1: 0

The research note avoids making new throughput or latency claims for bluetape-go. Existing `bluetape4k-graph` benchmark findings are treated as source evidence for adapter ordering, and the note requires fresh Go measurements before any graph I/O or backend performance claim.

### 안정성

P0: 0
P1: 0

The recommended order defers broad repository/session abstractions and immature adapters until smaller NDJSON/example proofs exist. Testcontainers coverage is called out before backend package work.

### 보안

P0: 0
P1: 0

No credentials, auth flows, or networked services are introduced. Compression/encryption I/O parity is explicitly deferred instead of being implied by the source Kotlin modules.

### 운영/Ops

P0: 0
P1: 0

The research separates drivers with first-party Testcontainers support from drivers that need custom containers. It keeps AGE, FalkorDB, and Memgraph backend work behind local-test proof.

### 개발자/API

P0: 0
P1: 0

The API direction is Go-shaped: start with minimal graph models only when needed by I/O/examples, and avoid porting Kotlin repository DSLs or TinkerPop abstractions directly.

### 사용자/호출자

P0: 0
P1: 0

The next implementation path favors concrete caller value: one domain example and streamable NDJSON/CSV helpers before backend-independent abstractions.

### 통합

P0: 0
P1: 0

Evidence sources include current `bluetape4k-graph` inventory, official/current Go driver documentation where available, and a preserved wiki research note. GNO wiki indexing remains a validation gap if the local model download cannot complete.
