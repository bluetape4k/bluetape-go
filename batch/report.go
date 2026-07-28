package batch

import "time"

// Status는 string 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// Report는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// IsSuccess는 IsSuccess 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailure는 IsFailure 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
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
