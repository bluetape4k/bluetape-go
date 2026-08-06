# Token-Bucket Rate Limiter Lessons (2026-06-05)

Related issue: #25
Milestone: 0.3.0
Affected packages: `ratelimit`, `ratelimit/redis`

## L1: apply patch는 feature worktree를 명시적으로 target해야 한다

### 문제

첫 research/spec/plan patch는 feature worktree가 아니라 main repo cwd에서 적용됐다.
implementation 전에 두 worktree의 `git status`를 비교하면서 이 실수를 발견했다.

### 교훈

linked worktree의 Type A work에서는 항상 `apply_patch` path를 확인하고, 첫 write 후
main worktree와 feature worktree 양쪽에서 `git status --short --branch`를 실행한다.
patch가 잘못된 worktree에 들어가면 agent-created file만 제거하고 feature worktree
아래에 다시 적용한 뒤 계속한다.

### Evidence

- main `develop` worktree를 clean state로 복구했다.
- feature branch는 research/spec/plan/review artifact를 유지했다.

## L2: external official-doc evidence는 PR 전에 wiki 보존이 필요하다

### 문제

첫 plan draft는 official Redis/Go docs를 design evidence로 사용했지만 workflow SOP가
요구하는 `bluetape4k-wiki` preservation step을 포함하지 않았다.

### 교훈

bluetape4k feature가 external official docs를 사용하면 implementation 전에 plan에
wiki note preservation과 `gno embed --collection bluetape4k-wiki`를 포함한다. 누락은
postscript가 아니라 review finding으로 취급한다.

### Evidence

- `bluetape4k-wiki` note:
  `research/2026-06-05-token-bucket-rate-limiter-redis-go.md`
- Wiki commit: `2ac234d`
- Validation: `gno update`, `gno embed --collection bluetape4k-wiki`,
  representative `gno search`.

## L3: per-call token request는 burst로 제한해야 한다

### 문제

initial spec은 positive token request를 검증했지만 caller가 bucket capacity보다 많은
token을 요청하면 어떻게 되는지 명시하지 않았다.

### 교훈

token-bucket API에서 `tokens > Burst`는 아무리 기다려도 만족할 수 없으므로 validation
error여야 한다. local/Redis test 모두에서 이 behavior를 고정한다.

### Evidence

- `TestTokenBucketRejectsOverBurstRequest`
- `TestLimiterRejectsInvalidAllowInput`
