Closes #59.

## 요약

- 실행 가능한 `examples/audit` order-service recipe를 `audit.Repository`
  경계를 통해 aggregate command 변경을 기록하도록 추가한다.
- audit history query, in-memory source-state lookup, history replay를 작은
  in-memory outbox fixture로 수행하는 예를 보여 준다.
- source-state, audit-history, outbox 경계를 설명하는 README diagram과
  초심자용 설명을 추가한다.
- example이 full event sourcing이나 JaVers 방식 diff engine이 아님을
  문서화하고, durable delivery use case는 `audit/sqloutbox`를 가리킨다.

## 검증

- RED: `go test -count=1 ./examples/audit`는 구현 전에 필요한 example-service API가 없어 실패했다.
- `go test -count=1 ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `go test -race -count=1 ./examples/audit`
- `go vet ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `make ci`
- Best-practices 기준 이미지: `bluetape4k-wiki/docs/diagrams/best-practices/assets/workflow-image-upload.png`
- `ruby scripts/validate-diagram-best-practices.rb`를 `bluetape4k-wiki`에서 실행
- `python3 -c "import xml.etree.ElementTree as ET; ET.parse('docs/images/readme-diagrams/audit-example-service-flow.svg')"`
- `/Users/debop/.codex/skills/fireworks-tech-graph/scripts/validate-svg.sh docs/images/readme-diagrams/audit-example-service-flow.svg`
- `~/.local/bin/cairosvg docs/images/readme-diagrams/audit-example-service-flow.svg -o docs/images/readme-diagrams/audit-example-service-flow.png -s 2`
- rendered PNG 검사, marker-color audit, endpoint/crossing review, Unicode fallback scan
- `git diff --check`

## DoD Status

- [x] 다음 범위를 위한 tests-first coverage: history query, repository-boundary rollback, concurrent command,
      cancellation-aware outbox replay를 위한 tests-first coverage가 있다.
- [x] example test에서 `GoroutineStressTester`와 `AsyncJobTester`를 사용한다.
- [x] public example 동작을 English 및 Korean README에 문서화했고,
      bluetape4k-wiki Scenario Workflow best-practice baseline과 대조한
      source-backed SVG/PNG diagram을 review했다.
- [x] 이슈에 대한 lesson, spec, plan, review 산출물을 추가했다.
