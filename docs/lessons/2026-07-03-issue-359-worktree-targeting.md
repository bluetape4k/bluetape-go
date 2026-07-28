# Issue #359 worktree targeting

일자: 2026-07-03

TDD RED 단계에서 첫 test patch가 issue worktree가 아니라 root checkout에 적용되었다.
Root checkout은 즉시 정리했고, 같은 test를 `.worktrees/issue-359-core-helpers`에 적용해
missing helper symbol로 RED 단계가 올바르게 실패하는 것을 확인했다.

교훈: git worktree를 만든 뒤에는 edit-producing step마다 absolute path 또는 `workdir`로
scoped command를 사용한다. Patch tool에 명시적인 workdir parameter가 없을 때는 test
result를 신뢰하기 전에 root checkout과 worktree 모두에서 `git status --short --branch`를
확인한다.

예방: root checkout에서 branch를 만든 경우, 특히 TDD RED claim 전에 Step 0 repair
evidence에 root/worktree status를 포함한다.
