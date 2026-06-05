# Lessons Learned — RESP3 CLIENT TRACKING Research (2026-06-04)

**Related issue**: #110
**Impact modules**: `docs/research`, `cache/redisnear`

## L1: Strategy boundaries need dependency-level proof

### Problem

RESP3 `CLIENT TRACKING` looks like a natural Redis NearCache alternative, but
the current `go-redis/v9` surface provides RESP3 and push notification hooks
without a high-level typed `CLIENT TRACKING` API.

### Lesson

Do not expose a public `redisnear.NewTracking` constructor until the lower-level
`CLIENT TRACKING` command, push handler, connection affinity, and reconnect
behavior are proven together with Testcontainers.

### Evidence

- `docs/research/2026-06-04-issue-110-resp3-client-tracking.md`
- Local dependency inspection of `github.com/redis/go-redis/v9 v9.20.0`

## L2: Research artifacts must live in the searchable research index

### Problem

#23 already had a `docs/superpowers/research` note, but the user explicitly
requested durable `docs/research` material that GNO can search as milestone
knowledge.

### Lesson

Milestone strategy decisions should be written under `docs/research`, linked
from `docs/research/README.md`, and connected back to the milestone research
note before PR creation.

### Evidence

- Added `docs/research/2026-06-04-issue-110-resp3-client-tracking.md`.
- Updated `docs/research/README.md`.
- Updated `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md`.
