package batch

import (
	"context"
	"fmt"
)

// Runner is executable batch work.
type Runner interface {
	Name() string
	Run(context.Context) Report
}

// Job runs batch steps sequentially.
type Job struct {
	name  string
	steps []Runner
}

// NewJob creates a sequential batch job.
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

// Name returns the job name.
func (j *Job) Name() string {
	if j == nil {
		return ""
	}
	return j.name
}

// Run executes the job until all steps complete or one step fails/cancels.
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
