# Research Output 한국어 Triage

Issue: #620
Parent: #615
Scope: `docs/research/outputs/**`

## 결정

이 directory의 기존 artifact는 benchmark, profile, environment, command output의
재현성 근거다. 따라서 기존 raw artifact를 번역하거나 재작성하지 않는다. 대신 이 문서가
한국어 metadata와 보존 사유를 제공한다.

기존 `README.md`와 `environment.md`도 capture 당시의 evidence ledger 역할을 하므로
원문을 유지한다. 영어 제목, command, version, SHA, checksum, benchmark value, source path,
JSON block, pprof binary는 모두 exact evidence로 취급한다.

## Artifact 처리 규칙

| Artifact kind | 처리 | 이유 |
|---|---|---|
| `*.txt` | 원문 보존 | `go test`, `benchstat`, `pprof -top`, Docker/runtime output이므로 byte-level traceability가 중요하다. |
| `*.pprof` | 원문 보존 | Go profile binary artifact라 번역 대상이 아니다. |
| `*.json`, `*.jsonl`, `*.csv` | 원문 보존 | machine-readable evidence 또는 normalized comparison data다. |
| 기존 `README.md` | 원문 보존 + 이 문서로 한국어 companion 제공 | artifact directory 설명이지만 README 제외 규칙과 evidence-ledger 성격을 함께 만족한다. |
| 기존 `environment.md` | 원문 보존 + 이 문서로 한국어 companion 제공 | host, command, checksum, dirty-tree state는 capture ledger라 정확한 원문이 필요하다. |
| 새 `korean-triage.md` | 한국어 설명 | 기존 artifact를 손상하지 않고 한국어 독자가 범위와 해석 한계를 이해하게 한다. |

## Directory별 처리 현황

| Directory | Files | Existing types | Korean handling |
|---|---:|---|---|
| `issue-168` | 9 | `txt` | ID generator benchmark/test/runtime output 원문 보존. 관련 해석은 narrative research 문서가 담당한다. |
| `issue-173` | 1 | `txt` | distributed JWT Redis benchmark output 원문 보존. |
| `issue-180` | 1 | `txt` | FastMoney evaluation benchmark output 원문 보존. |
| `issue-192` | 24 | `csv`, `md`, `pprof`, `txt` | third comparison table, raw benchmark, profile artifact 원문 보존. `third-comparison-kotlin-go.md`의 table label과 note는 비교 evidence라 그대로 둔다. |
| `issue-195` | 2 | `txt` | compression benchmark environment/output 원문 보존. |
| `issue-401` | 5 | `md`, `txt` | SerDe baseline README/environment와 raw benchmark output 원문 보존. #402 권고 문서와 이 companion가 한국어 해석을 제공한다. |
| `issue-421` | 2 | `md`, `txt` | rules benchmark environment와 raw output 원문 보존. |
| `issue-434` | 9 | `md`, `pprof`, `txt` | UUID v7 benchmark environment, benchstat, profile output 원문 보존. |
| `issue-435` | 2 | `md`, `txt` | textsearch benchmark environment와 raw output 원문 보존. module metadata JSON block은 그대로 둔다. |
| `issue-436` | 4 | `md`, `txt` | NLP benchmark environment와 raw output 원문 보존. dependency/module cache metadata는 그대로 둔다. |
| `issue-437` | 3 | `md`, `txt` | JWT Redis contention environment와 raw benchmark output 원문 보존. |
| `issue-438` | 3 | `md`, `txt` | Neo4j/Memgraph environment와 raw benchmark output 원문 보존. |
| `issue-439` | 3 | `md`, `txt` | audit repository/sqloutbox benchmark environment와 raw output 원문 보존. |
| `issue-455` | 18 | `md`, `pprof`, `txt` | zstd allocation profile README/environment, profile binary, pprof top output 원문 보존. |
| `issue-456` | 16 | `md`, `pprof`, `txt` | JSON repeated profile README/environment, profile binary, pprof top output 원문 보존. |
| `issue-560` | 14 | `md`, `txt` | provider benchmark environment와 raw/failure capture output 원문 보존. 실패 capture도 evidence boundary라 그대로 둔다. |

## 읽는 방법

- benchmark 수치 해석은 각 issue의 narrative research 문서에서 확인한다.
- 이 directory의 artifact는 production ranking, default 변경 권한, regression threshold가
  아니다. 같은 host, 같은 command, 같은 fixture 안에서만 비교한다.
- `environment.md`가 있으면 먼저 Git SHA, dirty tree, runtime, command inventory를 확인한다.
- `-failed-`가 들어간 file은 실패한 development capture다. canonical report input으로 쓰지
  말고, 실패 경계와 capture failure mode를 확인하는 증거로만 사용한다.
- `.pprof` file은 `go tool pprof`로 읽는 binary artifact다. Markdown으로 변환하거나
  번역하지 않는다.

## Validation Contract

이 issue의 검증은 기존 output artifact의 byte-preservation을 증명해야 한다.

- 기존 `docs/research/outputs/**` file은 이 companion 문서 추가 외에는 변경하지 않는다.
- `git diff --name-only -- docs/research/outputs`는 이 파일만 보여야 한다.
- `git diff --check -- docs/research/outputs`가 통과해야 한다.
