# Issue #422 OpenTelemetry Bridge Guidance Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: [#422](https://github.com/bluetape4k/bluetape-go/issues/422)
브랜치: `docs/issue-422-otel-bridge`
날짜: 2026-07-09

## 범위

- `docs/research/2026-07-09-issue-422-otel-bridge-guidance.md`
- `docs/research/README.md`
- `resilience/README.md`
- `resilience/README.ko.md`
- `CHANGELOG.md`

## 검토

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | P0=0 P1=0. Docs-only change; no runtime code, exporter, handler, or dependency added. |
| Stability | PASS | P0=0 P1=0. Guidance keeps OpenTelemetry provider/exporter lifecycle in application initialization instead of package hooks. |
| Security | PASS | P0=0 P1=0. No telemetry payload expansion or secret-bearing logging defaults added. |
| Operator/Ops | PASS | P0=0 P1=0. README states applications own provider/exporter setup and that `resilience` remains dependency-free. |
| Developer/API | PASS | P0=0 P1=0. Existing `OnEvent` and `slog` bridge surfaces remain unchanged; official `otelslog` is documented only as an app-level option. |
| User/Caller | PASS | P0=0 P1=0. #139 is linked as `slog` demand but not overstated as OpenTelemetry demand; #422 closes as boundary guidance. |
| Integration | PASS | P0=0 P1=0. Current #275 and #361 decisions are preserved without scheduling a new adapter package. |

## 검증

| Command | Status | Evidence |
|---|---|---|
| `gno query "OpenTelemetry bridge slog context hooks bluetape-go" -c bluetape4k-github --fast --no-rerank` | PASS | Found #361, workshop #139, #275, and PR #321 context. |
| `gno query "OpenTelemetry bridge slog context hooks bluetape-go" -c bluetape4k-docs --no-rerank` | PASS | Found #275 and #361 research decisions. |
| Context7 OpenTelemetry Go docs lookup | PASS | Confirmed official `otelslog` bridge and application-owned provider/exporter setup pattern. |
| `rg -n "OpenTelemetry|otelslog|slog Bridge|global logging|logger registry" resilience docs/research README.md README.ko.md CHANGELOG.md` | PASS | Targeted references stay in docs and preserve boundary wording. |
| `go test -run Example -count=1 ./resilience` | PASS | Compile-checked `slog` bridge example still passes. |
| `git diff --check` | PASS | No whitespace errors. |

## 발견 사항

P0=0 P1=0

- P2 accepted: no OpenTelemetry code example is added. Current demand is
  `slog` adoption rather than a concrete OpenTelemetry deployment, so a prose
  boundary is less misleading than a dependency-bearing example.
