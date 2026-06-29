# Issue #59 Superpowers Review: Audit Example Service

## Scope

- Code: `examples/audit`
- Docs: root README files, `examples/audit` README files, changelog, spec, plan,
  lesson, README diagram SVG/PNG

## 7-Tier Summary

- Performance: P0=0 P1=0. Example code is bounded in-memory recipe code and has
  no unbounded worker loop.
- Stability: P0=0 P1=0. Context cancellation is checked and covered with
  `AsyncJobTester`.
- Security: P0=0 P1=0. No network listener, auth boundary, or secret handling is
  added; README states persistence/outbox caveats.
- Ops: P0=0 P1=0. Durable delivery is explicitly deferred to `audit/sqloutbox`.
- Developer/API: P0=0 P1=0. The code stays under `examples/` and does not create
  a production helper API.
- User/Caller: P0=0 P1=0. README explains this is not event sourcing, JaVers
  diffing, or durable storage, and the diagram shows the source-state,
  audit-history, and outbox boundaries for new readers.
- Integration: P0=0 P1=0. Root README and changelog link the runnable example.

## Evidence

- `go test -count=1 ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `go test -race -count=1 ./examples/audit`
- `go vet ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `git diff --check`
- `make ci`
- `python3 -c "import xml.etree.ElementTree as ET; ET.parse('docs/images/readme-diagrams/audit-example-service-flow.svg')"`
- `~/.local/bin/cairosvg docs/images/readme-diagrams/audit-example-service-flow.svg -o docs/images/readme-diagrams/audit-example-service-flow.png -s 2`
- Manual PNG inspection plus marker-color audit. The skill-referenced diagram
  helper scripts were absent from the installed skill directory, so those helper
  checks were replaced with XML, marker, and rendered-PNG evidence.
