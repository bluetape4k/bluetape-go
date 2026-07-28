# Concurrency Testers

Issue #69는 Kotlin/JVM tester 이름을 source reference로만 유지한다. Go public API는
Go concept로 이름 붙인다. bounded goroutine contention check는
`GoroutineStressTester`, context/deadline/cancellation job check는 `AsyncJobTester`가
맡는다. Java virtual-thread semantics를 직접 반영한 `StructuredTaskScopeTester`는
노출하지 말고, Go에서는 `concurrency.Group` test로 검증한다.
