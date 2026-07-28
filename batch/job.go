package batch

import (
	"context"
	"fmt"
)

// Runner interface 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Runner interface {
	Name() string
	Run(context.Context) Report
}

// Job struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Job struct {
	name  string
	steps []Runner
}

// NewJob NewJob 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - name: report나 상태를 식별할 이름이다.
//   - steps: job에 포함할 step 목록이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func NewJob(name string, steps ...Runner) (*Job, error) {
	if name == "" {
		return nil, fmt.Errorf("job name must not be empty")
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("at least one step is required")
	}
	for index, step := range steps {
		if step == nil {
			return nil, fmt.Errorf("step %d must not be nil", index)
		}
	}
	return &Job{name: name, steps: append([]Runner(nil), steps...)}, nil
}

// Name Name 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (j *Job) Name() string {
	if j == nil {
		return ""
	}
	return j.name
}

// Run Run 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
func (j *Job) Run(ctx context.Context) Report {
	ctx = normalizeContext(ctx)
	if j == nil {
		report := newReport("")
		report.finish(StatusFailed, fmt.Errorf("job must not be nil"))
		return report
	}

	report := newReport(j.name)
	children := make([]Report, 0, len(j.steps))
	for _, step := range j.steps {
		child := step.Run(ctx)
		children = append(children, child)
		report.ReadCount += child.ReadCount
		report.WriteCount += child.WriteCount
		report.FilterCount += child.FilterCount
		report.SkipCount += child.SkipCount
		report.RetryCount += child.RetryCount
		if !child.IsSuccess() {
			report.Children = copyReports(children)
			status := child.Status
			if len(children) > 1 && status == StatusFailed {
				status = StatusPartial
			}
			report.finish(status, child.Err)
			return report
		}
	}

	report.Children = copyReports(children)
	report.finish(StatusCompleted, nil)
	return report
}
