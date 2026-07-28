# IAM Access Graph Example Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

범위: issue #368, `examples/graph/iamaccess`, README links, and
`graph-iam-access-paths` README diagram.

Baseline: `f05a8c8` (`develop` after #366).

## 6개 관점

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Fixture is 22 vertices and 20 edges; traversals are bounded in memory, with `maxGroupDepth=4` for membership expansion. No backend or goroutine path added. |
| Stability | PASS | `SeedIAMAccessGraph` validates graph IDs/labels via `graph.ParseVertex` and `graph.ParseEdge`; assembly rejects duplicate vertices and missing endpoints. NDJSON round-trip test covers import/export boundaries. |
| Security | PASS | Example documents that it is not a full policy engine; deny paths are evaluated before allow paths in `ExplainAccess`; production omissions call out policy completeness, directory sync, audit retention, expiry, and tenant scope. |
| Operator/Ops | PASS | No runtime service, environment variable, Docker, or external backend dependency added. README lists exact `go test` and race commands. |
| Developer/API | PASS | Public API stays Go-shaped: immutable fixture accessors, explicit `AccessExplanation` and `PrivilegeChain` values, `map[string][]string` approved-action input, and graphio import/export helpers matching the observability example. |
| User/Caller | PASS | Tests and README demonstrate effective access, explicit deny, risky nested admin inheritance, least-privilege drift, temporary access, and NDJSON round-trip. Diagram visually separates member, allow, deny, and temporary grant paths. |

## 다이어그램 증거

| Gate | Evidence | Verdict |
|---|---|---|
| SVG parse | `xml parse: PASS docs/images/readme-diagrams/graph-iam-access-paths.svg` | PASS |
| PNG render | `~/.local/bin/cairosvg docs/images/readme-diagrams/graph-iam-access-paths.svg -o docs/images/readme-diagrams/graph-iam-access-paths.png -s 2` | PASS |
| Endpoint audit | `diagram endpoint audit: PASS files=1` | PASS |
| Geometry audit | `graph-iam-access-paths.svg: geometry_failures=0` | PASS |
| Mixed-corner audit | `diagram mixed-corner audit: PASS files=1 paths=17 q_bends=2 failures=0` | PASS |
| Connector audit | `graph-iam-access-paths.svg: PASS markers=4 connectors=17 cards=17 intrusions=0 crossings=0` | PASS |
| Full-size PNG inspection | Opened `docs/images/readme-diagrams/graph-iam-access-paths.png`; no card/text overlap, no connector crossing, right resource card has enough size for delete/read paths. | PASS |

## 통합 판정

P0=0 P1=0

No P0/P1 blocker found. Residual P2: the diagram intentionally omits Bob's
direct read-only path to keep the visual focused on inherited allow, explicit
deny, nested admin drift, and temporary grant paths; README and tests cover Bob.
