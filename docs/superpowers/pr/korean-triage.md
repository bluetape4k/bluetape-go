# PR Handoff 문서 한국어 Triage

Issue: #621
Parent: #615
Scope: `docs/superpowers/pr/*.md`

## 결정

이 directory의 기존 파일은 GitHub PR body 또는 PR handoff 초안이다. repo 규칙상 public
GitHub issue/PR title과 body는 English로 작성해야 하므로 기존 PR body 문서를 한국어로
직접 재작성하지 않는다.

따라서 #621에서는 이 companion 문서가 한국어 설명을 제공하고, 기존 PR handoff artifact는
English 원문을 유지한다. 이는 “No public GitHub issue/PR body template is localized in place”
acceptance criterion을 만족하기 위한 의도적 범위 결정이다.

## 보존 대상

| Pattern | 처리 | 이유 |
|---|---|---|
| `*-pr-body.md` | English 원문 보존 | GitHub PR body 초안이므로 public metadata language rule을 따른다. |
| `*-pr.md` | English 원문 보존 | PR handoff artifact이며 issue/PR comment로 재사용될 수 있다. |
| code block, command, issue/PR number | 원문 보존 | GitHub traceability와 실행 재현성을 유지한다. |

## 검증 기준

- 기존 `docs/superpowers/pr/*.md` file은 이 companion 추가 외에는 변경하지 않는다.
- PR body template 본문이 한국어로 localize되지 않았음을 diff로 확인한다.
- `git diff --check -- docs/superpowers/pr`가 통과해야 한다.
