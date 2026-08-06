# Issue #436 NLP adapter benchmark review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #436
날짜: 2026-07-08
범위: benchmark suite and evidence retention for optional Kagome/Lingua
packages.

## 검토한 산출물

- `textsearch/japanese/tokenizer_benchmark_test.go`
- `textsearch/language/detector_benchmark_test.go`
- `docs/research/2026-07-08-issue-436-nlp-memory-bench.md`
- `docs/research/outputs/issue-436/nlp-bench.txt`
- `docs/research/outputs/issue-436/nlp-cold-start-isolated.txt`
- `docs/research/outputs/issue-436/nlp-startup-benchtime-1x.txt`
- `docs/research/outputs/issue-436/environment.md`
- `docs/lessons/2026-07-08-issue-436-nlp-memory-bench.md`

## 발견 사항

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | Benchmark-only files do not change production `textsearch/japanese` or `textsearch/language` behavior. |
| P1 | None | Startup/first-use conclusions use isolated one-case-per-process snapshots, and the research note avoids production memory-limit claims from a local run. |
| P2 | None | Raw command output and dependency metadata are preserved under `docs/research/outputs/issue-436/`. |

## 관점 검사

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | Acceptance benchmark covers construction, first use, steady tokenization/detection, POS filters, confidences, and mixed-language detection. |
| Stability | Pass | Existing package tests pass together with benchmark compilation. |
| Security | Pass | No security boundary is added; language detection remains preprocessing guidance only. |
| Operator/Ops | Pass | Module cache sizes and isolated `/usr/bin/time -l` RSS snapshots are preserved as local run conditions. |
| Developer/API | Pass | No public API change; no new dependency is introduced beyond existing optional package dependencies. |
| User/Caller | Pass | Existing README guidance is evaluated and remains consistent with measured evidence. |

Final verdict: PASS. P0=0 P1=0.
