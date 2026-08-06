# Audit Output 한국어 Triage

Issue: #620
Parent: #615
Scope: `docs/audits/outputs/**`

## 결정

`docs/audits/outputs`의 기존 artifact는 audit closeout 당시의 command output,
GitHub issue/milestone snapshot, package inventory다. 이 파일들은 재현성과 추적성을 위해
원문 그대로 보존한다. 한국어 설명은 이 companion 문서에만 둔다.

## Artifact 처리 현황

| Directory | Files | Existing types | Korean handling |
|---|---:|---|---|
| `issue-200` | 8 | `json`, `jsonl`, `txt` | 모든 기존 file을 원문 보존한다. `go test`, `make ci`, package list, GitHub issue/milestone export는 exact evidence다. |

## File별 보존 사유

| File | 처리 | 이유 |
|---|---|---|
| `issue-200/go-test-all.txt` | 원문 보존 | 전체 `go test` output으로, package/result line을 정확히 유지해야 한다. |
| `issue-200/go-test-race-all.txt` | 원문 보존 | 전체 race test output으로, race detector 결과와 package line을 정확히 유지해야 한다. |
| `issue-200/go-test-race-targeted.txt` | 원문 보존 | targeted race test output이다. command result evidence로 번역하지 않는다. |
| `issue-200/issues-0.1.0-0.6.1.json` | 원문 보존 | GitHub issue export의 machine-readable JSON snapshot이다. |
| `issue-200/make-ci.txt` | 원문 보존 | `make ci` output이다. step/result text를 그대로 보존한다. |
| `issue-200/milestones.json` | 원문 보존 | GitHub milestone export의 machine-readable JSON snapshot이다. |
| `issue-200/named-issues.jsonl` | 원문 보존 | issue mapping JSONL snapshot이다. line 단위 machine-readable evidence라 번역 대상이 아니다. |
| `issue-200/package-list.txt` | 원문 보존 | package inventory output이다. import/package path를 정확히 유지해야 한다. |

## 읽는 방법

- audit decision과 한국어 해석은 상위 audit/research narrative 문서를 따른다.
- 이 directory의 output은 evidence ledger이며 사용자-facing prose 문서가 아니다.
- JSON/JSONL은 formatting과 key/value를 보존해야 하므로 번역하지 않는다.
- `.txt` command output은 tool output 그대로 유지해야 하며, 필요하면 별도 Korean note를
  추가한다.

## Validation Contract

이 issue의 검증은 기존 audit output artifact의 byte-preservation을 증명해야 한다.

- 기존 `docs/audits/outputs/**` file은 이 companion 문서 추가 외에는 변경하지 않는다.
- `git diff --name-only -- docs/audits/outputs`는 이 파일만 보여야 한다.
- `git diff --check -- docs/audits/outputs`가 통과해야 한다.
