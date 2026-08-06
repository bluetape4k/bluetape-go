# Resilience Core Workflow

Issue #18은 새 public package와 policy API를 추가하므로 Type A work다.
implementation 전에 `docs/superpowers/research`, `docs/superpowers/specs`,
`docs/superpowers/plans` artifact를 만든다. review 전에는 worktree에서 CodeGraph와
code-review-graph를 모두 초기화한다. code-review-graph가 untracked new file을
놓칠 수 있으므로 explicit changed file 목록과 direct source review를 함께 사용한다.

`resilience`는 first-party로 유지한다. failsafe-go, cenkalti/backoff, gobreaker,
semaphore, rate 같은 external library는 reference input일 뿐 runtime wrapper가
아니다. retry/timeout에서는 context classification을 엄격히 검증한다. bare parent
`context.DeadlineExceeded`는 기본 retry 대상이 아니지만, policy-owned
`TimeoutError`는 retry가 outer policy일 때 retry될 수 있다.
