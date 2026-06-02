# Concurrency Helpers

Issue #10 established `concurrency` as a small public package for context-aware
goroutine helpers. Reuse `golang.org/x/sync/errgroup` for shared cancellation
and limits, convert task panics into errors, and keep test-only orchestration
helpers out of production APIs; issue #69 owns the tester surface inspired by
bluetape4k-junit5.
