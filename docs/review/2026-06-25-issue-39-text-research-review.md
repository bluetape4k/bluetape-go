# Issue 39 Text Research Review

Date: 2026-06-25
Scope: issue #39 research note and downstream text issue updates for #45 and
#52-#55.

## Verdict

P0: 0
P1: 0

This is a documentation and tracker-alignment change. It does not add Go
package code, exported APIs, dependencies, benchmark claims, or runtime
behavior.

## 7-Tier Review

### Performance

P0: 0
P1: 0

The research avoids adopting a high-performance Aho-Corasick package on claims
alone. It requires benchmark evidence before replacing a small first-party
implementation or publishing throughput claims.

### Stability

P0: 0
P1: 0

The recommendation avoids runtime mutable dictionaries as a first public
contract. Immutable compiled dictionaries and explicit rebuild/swap workflows
reduce race and lifecycle risk.

### Security

P0: 0
P1: 0

Blockword masking is treated as helper behavior, not a moderation security
boundary. The follow-up work must document bypass limits and avoid claiming
policy enforcement from string masking alone.

### Operator/Ops

P0: 0
P1: 0

Language detection and tokenizers are kept behind optional packages because
models and dictionaries affect binary size and memory. Lingua-Go memory caveats
and Kagome dictionary costs remain explicit follow-up concerns.

### Developer/API

P0: 0
P1: 0

The proposed API order is Go-shaped: compiled search and masking first, narrow
core models next, and language-specific packages only after dependency
research. It avoids porting Kotlin/JVM facades directly.

### User/Caller

P0: 0
P1: 0

The first useful caller path is deterministic multi-pattern search and masking.
Full Korean NLP parity is deferred instead of exposing weak or misleading
tokenizer contracts.

### Integration

P0: 0
P1: 0

Evidence sources include current `bluetape4k-text` inventory, issue #39 and
#45/#52-#55 scope, Go module metadata, GitHub repository metadata, and
preserved wiki research notes. GNO docs query attempted to download a large
local model and is recorded as a retrieval gap.
