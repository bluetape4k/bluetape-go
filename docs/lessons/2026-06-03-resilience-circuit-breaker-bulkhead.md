# Resilience Circuit Breaker and Bulkhead Workflow

Issue #19는 resilience state machine의 Go shape를 확인했다. runtime은
first-party로 유지하고, `context.Context` boundary를 명시하며, timing은 sleep이
아니라 injected time source로 deterministic하게 만든다.

graph-aware review를 요청하기 전에는 새 파일을 stage하거나 다른 방식으로
등록해야 한다. code-review-graph가 새 Go file을 parse할 수 있어도 untracked file은
initial changed-file set에 빠질 수 있다.

circuit breaker와 bulkhead test는 channel과 fake clock으로 concurrency를 제어해야
한다. half-open transition이나 permit release를 arbitrary sleep interval에 의존해
검증하지 않는다.

concurrency-sensitive primitive는 작은 race-safe smoke test만으로 부족하다. 많은
goroutine을 launch하고, admitted work를 channel로 block하며, atomic counter로 최대
observed concurrency를 기록하고, final state/permit counter가 0으로 돌아오는지
assert하는 stress test를 추가한다.
