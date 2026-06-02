# Concurrency Testers

Issue #69 keeps Kotlin/JVM tester names as source references only. Go public
APIs should name Go concepts: `GoroutineStressTester` for bounded goroutine
contention checks and `AsyncJobTester` for context/deadline/cancellation job
checks. Do not expose a direct `StructuredTaskScopeTester`; Java virtual-thread
semantics belong to direct `concurrency.Group` tests in Go.
