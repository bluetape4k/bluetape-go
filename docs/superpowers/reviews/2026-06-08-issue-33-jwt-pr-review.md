# Issue #33 JWT PR Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Task: Step 7-R PR review
이슈: #33
PR: #176
날짜: 2026-06-08
범위: PR #176 `issue-33-jwt` against `origin/develop`

## 통합 판정

PASS.

P0=0 P1=0

The first PR review iteration found one P1 in nested reader copy isolation. The
implementation now recursively copies `[]any` elements and has a nested
array/object mutation regression test.

## PR 메타데이터 증거

| 항목 | 상태 | Evidence |
|---|---|---|
| Assignee | PASS | PR #176 assignee `debop`, matching issue #33. |
| Labels | PASS | `type: task`, `priority: p1`, `area: utilities`, matching issue #33. |
| Milestone | PASS | `0.6.0`, matching issue #33. |
| Body | PASS | Non-empty PR body; final `##` heading is `## DoD Status`. |

## Fixed PR Review Finding

| Priority | Finding | Fix |
|---|---|---|
| P1 | `Reader` copy isolation did not recursively copy nested `[]any` elements, so a caller could mutate nested array/object claim or header state returned by `Header` or `Claim`. | `copyValue` now copies `[]any` element-by-element with recursive `copyValue`; `TestFixedHMACProviderComposesAndParsesClaims` mutates nested header and claim values and verifies the next read remains unchanged. |

## Validation Evidence After Fix

```bash
go test -count=1 ./jwt
go test -race -count=1 ./jwt
go test -count=1 ./...
golangci-lint run ./jwt
golangci-lint config verify
git diff --check origin/develop --
```

## Step 7-R Checklist Completion Report

| 항목 | 상태 | Notes |
|---|---|---|
| PR body verified before merge | Done | Body is non-empty and ends with `## DoD Status`. |
| PR metadata mirrors issue | Done | Assignee, labels, and milestone match #33. |
| Subagent PR review used | Done | Step 7-R code-reviewer lane found the nested copy P1. |
| P0/P1 fixed and revalidated | Done | P0=0 P1=0 after recursive copy fix and validation rerun. |
