// Package batch provides a small context-aware chunk processing core.
//
// A Step reads one input item at a time, processes it, and writes processed
// items in chunks. A Job runs one or more steps sequentially and stops at the
// first failed or cancelled step.
package batch
