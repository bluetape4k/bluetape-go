# Issue #435 Textsearch benchmark review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #435
날짜: 2026-07-08
범위: benchmark suite and dependency-adoption evidence for `textsearch`.

## 검토한 산출물

- `textsearch/matcher_benchmark_test.go`
- `docs/research/2026-07-08-issue-435-textsearch-bench.md`
- `docs/research/outputs/issue-435/textsearch-bench.txt`
- `docs/research/outputs/issue-435/environment.md`
- `docs/lessons/2026-07-08-issue-435-textsearch-bench.md`

## 발견 사항

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | Benchmark-only external imports do not change production `textsearch` APIs or matcher behavior. |
| P1 | None | Candidate comparison is scoped to raw matching and does not claim parity for Unicode normalization, offsets, boundaries, replacement, or masking. |
| P2 | None | Raw output and environment metadata are preserved for repeatability. |

## 관점 검사

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | `go test -run '^$' -bench . -benchmem ./textsearch` captured compile, contains, first, all matches, replacement, masking, and external raw candidates. |
| Stability | Pass | `go test -count=1 ./textsearch` passed after adding benchmark code. |
| Security | Pass | No masking or blockword semantics changed; benchmark docs explicitly avoid treating masking as a security boundary. |
| Operator/Ops | Pass | Output files include Go version, CPU, commit, module versions, licenses, archive state, stars, and pushed dates. |
| Developer/API | Pass | No public production API change; benchmark-only candidates are isolated to `_test.go`. |
| User/Caller | Pass | Existing first-party behavior remains intact; external candidates are not exposed to callers. |

Final verdict: PASS. P0=0 P1=0.
