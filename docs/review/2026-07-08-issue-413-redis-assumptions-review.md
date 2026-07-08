# Issue #413 Redis Probabilistic Assumptions Review

P0=0 P1=0

## Scope

- Issue: #413, document Redis probabilistic structure assumptions.
- Branch: `issue-413-redis-assumptions`
- Baseline: `d8c01f1`
- Files: `probabilistic/redis/README.md`, `probabilistic/redis/README.ko.md`,
  `scripts/generate-redis-readme-diagrams.py`, and
  `docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.{svg,png}`.

## Findings

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Docs-only/runtime-diagram change. No production code path changed. P0=0 P1=0. |
| Stability | PASS | README now states Bloom config metadata immutability, namespace rebuild requirements, and HLL approximate-count limits. P0=0 P1=0. |
| Security | PASS | Namespace guidance continues to reject raw user IDs, secrets, credentials, and API keys. No credential or ACL behavior changed. P0=0 P1=0. |
| Operator/Ops | PASS | Docs now name Redis `redis:7.4-alpine`, Lua/hash/bitmap/HLL command requirements, Cuckoo RedisBloom follow-up scope, and concrete diagnostic commands. P0=0 P1=0. |
| Developer/API | PASS | Guidance links assumptions to current exported APIs: `NewStringBloomFilter`, `NewBytesBloomFilter`, `NewHyperLogLog`, `NewStringHyperLogLog`, and `NewBytesHyperLogLog`. P0=0 P1=0. |
| User/Caller | PASS | English and Korean README updates are synchronized and distinguish Bloom membership from HLL approximate distinct count. P0=0 P1=0. |
| Integration | PASS | Diagram and docs match the implemented Redis test/API surface; no RedisBloom `CF*` API is documented as available. P0=0 P1=0. |

## Diagram Evidence

| Gate | Evidence | Result |
|---|---|---|
| Related-set scan | `rg -n "Redis Assumptions|Redis 가정|RedisBloom module|CF\\.ADD|NewStringHyperLogLog|NewStringBloomFilter|redis:7\\.4-alpine|probabilistic redis runtime map" ...` | README, source, tests, generator, and SVG align with #413 scope. |
| XML parse | `xmllint --noout docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg` | PASS |
| PNG render | `/Users/debop/.local/bin/cairosvg docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg -o docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.png -s 2` | PASS |
| PNG dimensions | `file ... && sips -g pixelWidth -g pixelHeight ...` | PASS, `3000 x 1840` |
| Connector audit | `diagram-connector-audit.py docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg` | PASS, `markers=7 connectors=6 cards=6 intrusions=0 crossings=0` |
| Geometry audit | `diagram-geometry-audit.py --fail-diagonal docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg` | PASS, `geometry_failures=0` |
| Endpoint audit | `diagram-endpoint-audit.py docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg` | PASS |
| Mixed-corner audit | `diagram-mixed-corner-audit.py docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.svg` | PASS, `paths=6 q_bends=8 failures=0` |
| Full-size eye check | Visual inspection of rendered PNG after final coordinate fix. | PASS, all labels readable, connectors attach to card edges, no text/card/connector overlap observed. |

## Validation

- `git diff --check`
- `go test -run Example -count=1 ./probabilistic/redis`
- `go test -count=1 ./probabilistic/redis`

Final verdict: PASS. `P0=0 P1=0`.
