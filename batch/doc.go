// Package batch provides a small context-aware chunk processing core.
//
// A Step reads one input item at a time, processes it, and writes processed
// items in chunks. A Job runs one or more steps sequentially and stops at the
// first failed or cancelled step.
//
// NewStep preserves the legacy Writer + CheckpointStore path. That path can use
// durable checkpoint storage, but it is not atomic with business writes because
// Writer.Write and CheckpointStore.Save are separate operations. NewAtomicStep
// is the additive opt-in path for an AtomicCheckpointWriter that commits output
// and reader progress together.
//
// In an atomic step, RetryPolicy and SkipPolicy apply to processor failures only.
// They never apply to AtomicCheckpointWriter.Commit, its business callback,
// checkpoint CAS, or commit-unknown and atomicity-unknown unknown-outcome errors.
package batch
