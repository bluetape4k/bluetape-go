# Concurrency Helpers

Issue #10은 `concurrency`를 context-aware goroutine helper용 작은 public package로
정했다. shared cancellation과 limit에는 `golang.org/x/sync/errgroup`을 재사용하고,
task panic은 error로 변환한다. test-only orchestration helper는 production API에
넣지 않는다. bluetape4k-junit5에서 영감을 받은 tester surface는 issue #69가
소유한다.
