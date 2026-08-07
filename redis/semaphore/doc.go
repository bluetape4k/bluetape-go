// Package redissem provides a bounded Redis semaphore with expiring permit
// leases and owner-safe idempotent release.
//
// A semaphore lease has no fencing token. After a lease expires, the old work
// may overlap with a new permit holder, so callers must bound critical
// sections, use an external resource version when required, and perform
// cleanup with a separate context when the work context is canceled.
package redissem
