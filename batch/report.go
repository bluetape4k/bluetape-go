package batch

import "time"

// Status is the terminal outcome for a batch step or job.
type Status string

const (
	// StatusCompleted means all batch work finished successfully.
	StatusCompleted Status = "completed"
	// StatusFailed means batch work stopped after a non-cancellation error.
	StatusFailed Status = "failed"
	// StatusCancelled means caller cancellation or deadline stopped batch work.
	StatusCancelled Status = "cancelled"
	// StatusPartial means a job completed at least one step before a later step failed.
	StatusPartial Status = "partial"
)

// Report captures a batch step or job outcome.
type Report struct {
	Name        string
	Status      Status
	StartedAt   time.Time
	EndedAt     time.Time
	ReadCount   int
	WriteCount  int
	FilterCount int
	Err         error
	Children    []Report
}

// IsSuccess reports whether the batch work completed without failure.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailure reports whether the batch work did not complete cleanly.
func (r Report) IsFailure() bool {
	return r.Status == StatusFailed || r.Status == StatusCancelled || r.Status == StatusPartial
}

func newReport(name string) Report {
	now := time.Now()
	return Report{
		Name:      name,
		Status:    StatusCompleted,
		StartedAt: now,
		EndedAt:   now,
	}
}

func (r *Report) finish(status Status, err error) {
	r.Status = status
	r.Err = err
	r.EndedAt = time.Now()
}

func copyReports(reports []Report) []Report {
	if len(reports) == 0 {
		return nil
	}
	copied := make([]Report, len(reports))
	copy(copied, reports)
	return copied
}
