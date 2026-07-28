package workreport

import "time"

// Report work report 상태, failure policy, child report에서 사용하는 구조체다.
type Report struct {
	Name      string
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	Err       error
	Reason    string
	Children  []Report
}

// Completed work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
func Completed(name string) Report {
	return newReport(name, StatusCompleted, nil, "", nil)
}

// Failed work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - err: 검사하거나 감쌀 오류 값이다.
func Failed(name string, err error) Report {
	return newReport(name, StatusFailed, err, "", nil)
}

// Partial work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - children: Partial에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func Partial(name string, children ...Report) Report {
	return newReport(name, StatusPartial, nil, "", children)
}

// Aborted work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - reason: 중단 또는 실패 이유다.
func Aborted(name, reason string) Report {
	return newReport(name, StatusAborted, nil, reason, nil)
}

// Cancelled work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - err: 검사하거나 감쌀 오류 값이다.
func Cancelled(name string, err error) Report {
	return newReport(name, StatusCancelled, err, "", nil)
}

// Aggregate work report 상태, failure policy, child report 동작을 수행한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - policy: Aggregate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - children: Aggregate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// IsSuccess work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailed work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsFailed() bool {
	return r.Status == StatusFailed
}

// IsPartial work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsPartial() bool {
	return r.Status == StatusPartial
}

// IsAborted work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsAborted() bool {
	return r.Status == StatusAborted
}

// IsCancelled work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsCancelled() bool {
	return r.Status == StatusCancelled
}

// IsFailure work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
func (r Report) IsFailure() bool {
	switch r.Status {
	case StatusFailed, StatusPartial, StatusAborted, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal work report 상태, failure policy, child report 상태가 조건을 만족하는지 반환한다.
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
