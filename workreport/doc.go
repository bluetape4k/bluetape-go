// Package workreport provides workflow result and failure-policy values.
//
// The package is intentionally independent from workflow runner execution. It
// gives callers stable status values, explicit failure policies, and report-tree
// aggregation helpers that future workflow runners can consume without owning a
// separate result model.
//
// A zero-value Report has an unknown status. It is not successful, failed, or
// terminal; use constructors such as Completed, Failed, Aborted, Cancelled, or
// Aggregate when creating caller-visible reports.
package workreport
