# Cache Benchmark Suite Lessons (2026-06-04)

Issue: #107
Milestone: 0.3.0

## L1: benchmark PR에는 command만 아니라 측정 해석 문서가 필요하다

### 문제

#107은 benchmark command, environment note, sample result를 요구했다. PR body만으로도
단기 review는 통과할 수 있지만 branch가 merge된 뒤 GNO에서 검색 가능한 durable
knowledge가 남지 않는다.

### 교훈

benchmark work는 sample output, environment note, "local snapshot, not production
ranking" 같은 explicit limit를 `docs/research` 아래에 기록해야 한다.

### Evidence

- `docs/research/2026-06-04-issue-107-cache-benchmark-suite.md`
- `go test -run '^$' -bench '^BenchmarkMemory' -benchtime=100ms -benchmem ./cache`
- `go test -run '^$' -bench '^BenchmarkNearCache' -benchtime=100ms -benchmem ./cache/redisnear`

## L2: patch 전에 worktree boundary를 확인한다

### 문제

첫 patch application은 session default directory를 사용해 #107 파일을 feature
worktree가 아니라 main `develop` worktree에 작성했다.

### 교훈

multi-worktree bluetape-go work에서는 `apply_patch` edit 전마다 absolute path를
사용하거나 target worktree에서 `git status`를 확인한다. patch가 잘못된 worktree에
들어가면 feature worktree로 옮기고 `develop`을 clean하게 복구한 뒤 계속한다.

### Evidence

- test 진행 전 main `develop` worktree를 clean 상태로 복구했다.
- 모든 #107 변경은 `.worktrees/bench-issue-107-cache-suite`에 존재한다.
