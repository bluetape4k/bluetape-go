# IAM Access Graph Example Review

Scope: issue #368, `examples/graph/iamaccess`, README links, and
`graph-iam-access-paths` README diagram.

Baseline: `f05a8c8` (`develop` after #366).

## Six Lanes

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Fixture is 22 vertices and 20 edges; traversals are bounded in memory, with `maxGroupDepth=4` for membership expansion. No backend or goroutine path added. |
| Stability | PASS | `SeedIAMAccessGraph` validates graph IDs/labels via `graph.ParseVertex` and `graph.ParseEdge`; assembly rejects duplicate vertices and missing endpoints. NDJSON round-trip test covers import/export boundaries. |
| Security | PASS | Example documents that it is not a full policy engine; deny paths are evaluated before allow paths in `ExplainAccess`; production omissions call out policy completeness, directory sync, audit retention, expiry, and tenant scope. |
| Operator/Ops | PASS | No runtime service, environment variable, Docker, or external backend dependency added. README lists exact `go test` and race commands. |
| Developer/API | PASS | Public API stays Go-shaped: immutable fixture accessors, explicit `AccessExplanation` and `PrivilegeChain` values, `map[string][]string` approved-action input, and graphio import/export helpers matching the observability example. |
| User/Caller | PASS | Tests and README demonstrate effective access, explicit deny, risky nested admin inheritance, least-privilege drift, temporary access, and NDJSON round-trip. Diagram visually separates member, allow, deny, and temporary grant paths. |

## Diagram Evidence

| Gate | Evidence | Verdict |
|---|---|---|
| SVG parse | `xml parse: PASS docs/images/readme-diagrams/graph-iam-access-paths.svg` | PASS |
| PNG render | `~/.local/bin/cairosvg docs/images/readme-diagrams/graph-iam-access-paths.svg -o docs/images/readme-diagrams/graph-iam-access-paths.png -s 2` | PASS |
| Endpoint audit | `diagram endpoint audit: PASS files=1` | PASS |
| Geometry audit | `graph-iam-access-paths.svg: geometry_failures=0` | PASS |
| Mixed-corner audit | `diagram mixed-corner audit: PASS files=1 paths=17 q_bends=2 failures=0` | PASS |
| Connector audit | `graph-iam-access-paths.svg: PASS markers=4 connectors=17 cards=17 intrusions=0 crossings=0` | PASS |
| Full-size PNG inspection | Opened `docs/images/readme-diagrams/graph-iam-access-paths.png`; no card/text overlap, no connector crossing, right resource card has enough size for delete/read paths. | PASS |

## Integrated Verdict

P0=0 P1=0

No P0/P1 blocker found. Residual P2: the diagram intentionally omits Bob's
direct read-only path to keep the visual focused on inherited allow, explicit
deny, nested admin drift, and temporary grant paths; README and tests cover Bob.
