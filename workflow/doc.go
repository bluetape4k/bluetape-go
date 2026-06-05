// Package workflow provides lightweight context-aware workflow runners.
//
// Work is an ordinary Go function that returns a workreport.Report. Runners
// compose those functions sequentially, conditionally, or in parallel without a
// mutable shared context map or durable workflow engine.
package workflow
