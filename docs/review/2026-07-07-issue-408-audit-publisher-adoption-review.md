# Issue #408 Review: Audit Publisher Adoption

## Scope

- English/Korean root README audit example summary updates.
- English/Korean `examples/audit` README publisher adoption and operator notes.
- `audit-example-service-flow` SVG/PNG refresh.
- Research and lesson notes for the adoption decision.

## Subagent Note

Native subagent spawning was not used because the available subagent tool
surface is gated to explicit user subagent requests. The main session performed
the six independent review lanes and integration verdict as fallback.

## Lane Findings

### Performance

P0: 0
P1: 0

- Documentation-only change; no runtime hot path changed.
- The example still avoids adding a durable broker adapter or new framework
  layer.

### Stability

P0: 0
P1: 0

- README prose references existing public API names only.
- Relay lifecycle distinctions are explicit: `RunOnce` for scheduler polling,
  `Run` for context-owned worker loops.

### Security

P0: 0
P1: 0

- Operator notes preserve bounded/redacted publisher error guidance.
- No credentials, network clients, TLS settings, broker topology, or durable
  transport configuration are introduced.

### Operator/Ops

P0: 0
P1: 0

- Retry behavior, cancellation behavior, persisted failure text, duplicate
  delivery, and downstream idempotency are documented.
- Workshop follow-up is linked instead of implying this repo now owns all
  runnable cross-repo service examples.

### Developer/API

P0: 0
P1: 0

- The adoption path names `Store.Enqueue`, `NewRelay`, `Relay.RunOnce`,
  `Relay.Run`, `Publisher.Publish`, `Record.EventID`, and
  `Record.IdempotencyKey`.
- `sqloutboxtest.RecordingPublisher` and `WithFailures` are documented for
  retry/duplicate test adoption.

### User/Caller

P0: 0
P1: 0

- Root and package README pairs now state how the example maps to production
  outbox relay and publisher handoff.
- The refreshed diagram shows both the example replay path and the production
  adoption path in one image.

## Integration Verdict

P0: 0
P1: 0

The change satisfies #408 without broadening runtime scope. It keeps the audit
example small, documents the relay-to-publisher adoption path with source
checked API names, links the workshop follow-up, and updates one existing
diagram asset with verified rendering.

## Diagram Evidence

| Gate | Evidence | Result |
|---|---|---|
| SVG parse | `python3 - <<'PY' ... ET.parse('docs/images/readme-diagrams/audit-example-service-flow.svg') ... PY` | PASS, `xml ok` |
| PNG render | `cairosvg docs/images/readme-diagrams/audit-example-service-flow.svg -o docs/images/readme-diagrams/audit-example-service-flow.png -s 2` | PASS |
| Connector audit | `diagram-connector-audit.py docs/images/readme-diagrams/audit-example-service-flow.svg` | PASS, `markers=3 connectors=9 cards=0 intrusions=0 crossings=0` |
| Geometry audit | `diagram-geometry-audit.py docs/images/readme-diagrams/audit-example-service-flow.svg` | PASS, `geometry_failures=0` |
| Mixed-corner audit | `diagram-mixed-corner-audit.py docs/images/readme-diagrams/audit-example-service-flow.svg` | PASS, `paths=9 q_bends=0 failures=0` |
| Endpoint audit | `diagram-endpoint-audit.py docs/images/readme-diagrams/audit-example-service-flow.svg` | PASS |
| Full-size eye check | Visual inspection of rendered `audit-example-service-flow.png` after label adjustment | PASS, no text/connector overlap observed |

## Evidence

- `rg -n "type Publisher|func NewRelay|RunOnce|Run\\(|Enqueue|EventID|IdempotencyKey|NewRecordingPublisher|WithFailures" audit/sqloutbox audit/sqloutbox/sqloutboxtest`
- `go test -count=1 ./examples/audit ./audit/sqloutbox ./audit/sqloutbox/sqloutboxtest`
- `go test -race -count=1 ./examples/audit ./audit/sqloutbox/sqloutboxtest`
- `git diff --check`
- `git diff --cached --check`
- `golangci-lint cache clean` after an initial lint run referenced the removed
  #407 worktree from cache
- `make ci`
