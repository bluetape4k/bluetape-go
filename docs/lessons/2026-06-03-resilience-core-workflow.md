# Resilience Core Workflow

Issue #18 is Type A work because it adds a new public package and policy API.
Create `docs/superpowers/research`, `docs/superpowers/specs`, and
`docs/superpowers/plans` artifacts before implementation. Initialize both
CodeGraph and code-review-graph in the worktree before review; code-review-graph
may miss untracked new files, so include explicit changed files and perform
direct source review for new packages.

Keep `resilience` first-party. External libraries such as failsafe-go,
cenkalti/backoff, gobreaker, semaphore, and rate are reference inputs only, not
runtime wrappers. For retry/timeout, verify context classification carefully:
bare parent `context.DeadlineExceeded` should not be retried by default, while a
policy-owned `TimeoutError` can be retried when retry is the outer policy.
