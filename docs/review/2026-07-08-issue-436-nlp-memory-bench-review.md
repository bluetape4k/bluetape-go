# Issue #436 NLP adapter benchmark review

Issue: #436
Date: 2026-07-08
Scope: benchmark suite and evidence retention for optional Kagome/Lingua
packages.

## Reviewed Artifacts

- `textsearch/japanese/tokenizer_benchmark_test.go`
- `textsearch/language/detector_benchmark_test.go`
- `docs/research/2026-07-08-issue-436-nlp-memory-bench.md`
- `docs/research/outputs/issue-436/nlp-bench.txt`
- `docs/research/outputs/issue-436/nlp-cold-start-isolated.txt`
- `docs/research/outputs/issue-436/nlp-startup-benchtime-1x.txt`
- `docs/research/outputs/issue-436/environment.md`
- `docs/lessons/2026-07-08-issue-436-nlp-memory-bench.md`

## Findings

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | Benchmark-only files do not change production `textsearch/japanese` or `textsearch/language` behavior. |
| P1 | None | Startup/first-use conclusions use isolated one-case-per-process snapshots, and the research note avoids production memory-limit claims from a local run. |
| P2 | None | Raw command output and dependency metadata are preserved under `docs/research/outputs/issue-436/`. |

## Lens Check

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | Acceptance benchmark covers construction, first use, steady tokenization/detection, POS filters, confidences, and mixed-language detection. |
| Stability | Pass | Existing package tests pass together with benchmark compilation. |
| Security | Pass | No security boundary is added; language detection remains preprocessing guidance only. |
| Operator/Ops | Pass | Module cache sizes and isolated `/usr/bin/time -l` RSS snapshots are preserved as local run conditions. |
| Developer/API | Pass | No public API change; no new dependency is introduced beyond existing optional package dependencies. |
| User/Caller | Pass | Existing README guidance is evaluated and remains consistent with measured evidence. |

Final verdict: PASS. P0=0 P1=0.
