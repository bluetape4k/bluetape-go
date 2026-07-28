# Issue #55 Tokenizer Research Step 6-R Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

Scope:

- Research-only diff for Korean/Japanese tokenizer and language detection
  dependency decisions.
- Baseline: `28bc9e6 Keep tokenizer core dependency-free`.

## 7-Tier 발견 사항

| Tier | Lens | P0 | P1 | P2/P3 | Evidence |
|---|---:|---:|---:|---:|---|
| 1 | Security | 0 | 0 | 0 | Research explicitly rejects treating blockword masking or language detection as a security policy. |
| 2 | Performance | 0 | 0 | 0 | Module cache size evidence forces Kagome/Lingua into optional packages and requires benchmark/memory checks in follow-ups. |
| 3 | Stability | 0 | 0 | 0 | Runtime mutable dictionaries remain deferred unless stress/race proof is added by a future package. |
| 4 | Code/API | 0 | 0 | 0 | Decisions reuse #54 `Tokenizer` and `DictionaryProvider` instead of adding broad Kotlin-shaped APIs. |
| 5 | Tests | 0 | 0 | 0 | Follow-up issues require `GoroutineStressTester`, targeted package tests, and race validation for dependency-backed features. |
| 6 | Docs/Ops | 0 | 0 | 0 | Research records license, maintenance, model size, deployment cost, and what Go core will not attempt. |
| 7 | User/Caller | 0 | 0 | 0 | Korean full tokenizer parity is deferred instead of exposing weak caller promises; Japanese/Lingua are isolated as optional packages. |

P0=0 P1=0

## 검증

- `gh issue view 55 --json ...`: PASS.
- `gh issue list --state open --search ...`: PASS, no duplicate Japanese/Lingua follow-up issue found.
- `gh repo view` metadata for Kagome, Lingua-Go, Whatlanggo, Kagome ko dict, gocld3, and MeCab wrapper: PASS.
- `go list -m -versions` for selected candidates: PASS.
- `go get` + `du -sh` module cache size sample: PASS.
- Follow-up issues #336 and #337 created: PASS.
- `git diff --check`: PASS.
- `make fmt-check`: PASS.
- Wiki preservation: PASS, `gno update`, `gno embed --collection
  bluetape4k-wiki`, and `gno search "Kagome Lingua-Go #336 #337" -c
  bluetape4k-wiki`.
