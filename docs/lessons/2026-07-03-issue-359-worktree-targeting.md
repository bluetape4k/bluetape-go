# Issue #359 Worktree Targeting

Date: 2026-07-03

During the TDD RED step, the first test patch was applied to the root checkout
instead of the issue worktree. The root checkout was cleaned immediately and the
same tests were applied to `.worktrees/issue-359-core-helpers`, where the RED
step failed correctly with missing helper symbols.

Lesson: after creating a git worktree, use absolute paths or commands scoped
with `workdir` for every edit-producing step. Before trusting a test result,
check both the root checkout and the worktree with `git status --short --branch`
when a patch tool has no explicit workdir parameter.

Prevention: include root/worktree status in the Step 0 repair evidence when a
branch is created from the root checkout, especially before TDD RED claims.
