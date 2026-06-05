package workreport

import "time"

// Report captures one work outcome and optional child outcomes.
type Report struct {
	Name      string
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	Err       error
	Reason    string
	Children  []Report
}

// Completed reports successful work.
func Completed(name string) Report {
	return newReport(name, StatusCompleted, nil, "", nil)
}

// Failed reports work that failed with err.
func Failed(name string, err error) Report {
	return newReport(name, StatusFailed, err, "", nil)
}

// Partial reports aggregated work with one or more non-completed children.
func Partial(name string, children ...Report) Report {
	return newReport(name, StatusPartial, nil, "", children)
}

// Aborted reports work stopped for a caller-defined reason.
func Aborted(name, reason string) Report {
	return newReport(name, StatusAborted, nil, reason, nil)
}

// Cancelled reports work stopped by caller cancellation or deadline.
func Cancelled(name string, err error) Report {
	return newReport(name, StatusCancelled, err, "", nil)
}

// Aggregate combines child reports according to policy.
func Aggregate(name string, policy FailurePolicy, children ...Report) (Report, error) {
	if !policy.valid() {
		return Report{}, FailurePolicyError{Policy: policy}
	}
	if len(children) == 0 {
		return Completed(name), nil
	}

	switch policy {
	case StopOnFailure:
		return aggregateStopOnFailure(name, children), nil
	case ContinueOnFailure:
		return aggregateContinueOnFailure(name, children), nil
	default:
		return Report{}, FailurePolicyError{Policy: policy}
	}
}

// IsSuccess reports whether the report is completed.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailed reports whether the report status is failed.
func (r Report) IsFailed() bool {
	return r.Status == StatusFailed
}

// IsPartial reports whether the report status is partial.
func (r Report) IsPartial() bool {
	return r.Status == StatusPartial
}

// IsAborted reports whether the report status is aborted.
func (r Report) IsAborted() bool {
	return r.Status == StatusAborted
}

// IsCancelled reports whether the report status is cancelled.
func (r Report) IsCancelled() bool {
	return r.Status == StatusCancelled
}

// IsFailure reports whether the report represents a non-success known outcome.
func (r Report) IsFailure() bool {
	switch r.Status {
	case StatusFailed, StatusPartial, StatusAborted, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether the report has a known terminal status.
func (r Report) IsTerminal() bool {
	return r.Status.known()
}

func aggregateStopOnFailure(name string, children []Report) Report {
	included := make([]Report, 0, len(children))
	for _, child := range children {
		included = append(included, child)
		if child.Status != StatusCompleted {
			return newReport(name, child.Status, child.Err, child.Reason, included)
		}
	}
	return newReport(name, StatusCompleted, nil, "", included)
}

func aggregateContinueOnFailure(name string, children []Report) Report {
	copied := copyReports(children)
	for _, child := range copied {
		if child.Status != StatusCompleted {
			return newReport(name, StatusPartial, nil, "", copied)
		}
	}
	return newReport(name, StatusCompleted, nil, "", copied)
}

func newReport(name string, status Status, err error, reason string, children []Report) Report {
	now := time.Now()
	return Report{
		Name:      name,
		Status:    status,
		StartedAt: now,
		EndedAt:   now,
		Err:       err,
		Reason:    reason,
		Children:  copyReports(children),
	}
}

func copyReports(reports []Report) []Report {
	if len(reports) == 0 {
		return nil
	}
	copied := make([]Report, len(reports))
	copy(copied, reports)
	return copied
}
