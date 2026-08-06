// Package batch context-aware chunk 처리 코어를 제공한다.
//
// Step 입력 항목을 하나씩 읽고, 처리한 뒤, 처리 결과를 chunk 단위로 기록한다.
// Job 하나 이상의 step을 순서대로 실행하며 실패하거나 취소된 step에서 중단한다.
//
// NewStep 기존 Writer + CheckpointStore 경로를 보존한다. 이 경로는 durable
// checkpoint 저장소를 사용할 수 있지만, Writer.Write와 CheckpointStore.Save가
// 별도 작업이므로 business write와 atomic하지 않다. NewAtomicStep은
// AtomicCheckpointWriter가 출력과 reader progress를 함께 commit하는 opt-in 경로다.
//
// atomic step에서 RetryPolicy와 SkipPolicy는 processor failures에만 적용된다.
// 이 정책은 AtomicCheckpointWriter.Commit, business callback, checkpoint CAS,
// commit-unknown 및 atomicity-unknown unknown-outcome 오류에는 적용되지 않는다.
package batch
