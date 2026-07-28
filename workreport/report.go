package workreport

import "time"

// Report struct 공개 타입이며 work report 상태, failure policy, child report 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Report struct {
	Name      string
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	Err       error
	Reason    string
	Children  []Report
}

// Completed Completed 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Completed가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
func Completed(name string) Report {
	return newReport(name, StatusCompleted, nil, "", nil)
}

// Failed Failed 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Failed가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - err: 검사하거나 감쌀 오류 값이다.
func Failed(name string, err error) Report {
	return newReport(name, StatusFailed, err, "", nil)
}

// Partial Partial 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Partial가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - children: Partial 동작에 필요한 children 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Partial(name string, children ...Report) Report {
	return newReport(name, StatusPartial, nil, "", children)
}

// Aborted Aborted 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Aborted가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - reason: Aborted가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
func Aborted(name, reason string) Report {
	return newReport(name, StatusAborted, nil, reason, nil)
}

// Cancelled Cancelled 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Cancelled가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - err: 검사하거나 감쌀 오류 값이다.
func Cancelled(name string, err error) Report {
	return newReport(name, StatusCancelled, err, "", nil)
}

// Aggregate Aggregate 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
//
// 매개변수:
//   - name: Aggregate가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - policy: Aggregate 동작에 필요한 policy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - children: Aggregate 동작에 필요한 children 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// IsSuccess IsSuccess 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailed IsFailed 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsFailed() bool {
	return r.Status == StatusFailed
}

// IsPartial IsPartial 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsPartial() bool {
	return r.Status == StatusPartial
}

// IsAborted IsAborted 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsAborted() bool {
	return r.Status == StatusAborted
}

// IsCancelled IsCancelled 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsCancelled() bool {
	return r.Status == StatusCancelled
}

// IsFailure IsFailure 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
func (r Report) IsFailure() bool {
	switch r.Status {
	case StatusFailed, StatusPartial, StatusAborted, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal IsTerminal 공개 API의 동작을 수행하며 work report 상태, failure policy, child report 계약을 보존한다.
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
