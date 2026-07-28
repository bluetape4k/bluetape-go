# Issue 39 Text Research Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-25
범위: issue #39 research note and downstream text issue updates for #45 and
#52-#55.

## 판정

P0: 0
P1: 0

This is a documentation and tracker-alignment change. It does not add Go
package code, exported APIs, dependencies, benchmark claims, or runtime
behavior.

## 7-Tier 검토

### 성능

P0: 0
P1: 0

The research avoids adopting a high-performance Aho-Corasick package on claims
alone. It requires benchmark evidence before replacing a small first-party
implementation or publishing throughput claims.

### 안정성

P0: 0
P1: 0

The recommendation avoids runtime mutable dictionaries as a first public
contract. Immutable compiled dictionaries and explicit rebuild/swap workflows
reduce race and lifecycle risk.

### 보안

P0: 0
P1: 0

Blockword masking is treated as helper behavior, not a moderation security
boundary. The follow-up work must document bypass limits and avoid claiming
policy enforcement from string masking alone.

### 운영/Ops

P0: 0
P1: 0

Language detection and tokenizers are kept behind optional packages because
models and dictionaries affect binary size and memory. Lingua-Go memory caveats
and Kagome dictionary costs remain explicit follow-up concerns.

### 개발자/API

P0: 0
P1: 0

The proposed API order is Go-shaped: compiled search and masking first, narrow
core models next, and language-specific packages only after dependency
research. It avoids porting Kotlin/JVM facades directly.

### 사용자/호출자

P0: 0
P1: 0

The first useful caller path is deterministic multi-pattern search and masking.
Full Korean NLP parity is deferred instead of exposing weak or misleading
tokenizer contracts.

### 통합

P0: 0
P1: 0

Evidence sources include current `bluetape4k-text` inventory, issue #39 and
#45/#52-#55 scope, Go module metadata, GitHub repository metadata, and
preserved wiki research notes. GNO docs query attempted to download a large
local model and is recorded as a retrieval gap.
