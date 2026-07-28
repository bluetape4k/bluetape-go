package batch

import "time"

// Status batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 문자열 타입이다.
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

// Report batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type Report struct {
	Name        string
	Status      Status
	StartedAt   time.Time
	EndedAt     time.Time
	ReadCount   int
	WriteCount  int
	FilterCount int
	SkipCount   int
	RetryCount  int
	Err         error
	Children    []Report
}

// IsSuccess batch 단계, checkpoint, writer 안전성, 재시작 상태가 조건을 만족하는지 반환한다.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailure batch 단계, checkpoint, writer 안전성, 재시작 상태가 조건을 만족하는지 반환한다.
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
