package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/workreport"
)

// Work func 공개 타입이며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Work func(context.Context) workreport.Report

// Predicate func 공개 타입이며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Predicate func(context.Context) (bool, error)

// Runner interface 공개 타입이며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Runner interface {
	Run(context.Context) workreport.Report
}

type sequentialRunner struct {
	name   string
	policy workreport.FailurePolicy
	works  []Work
}

type conditionalRunner struct {
	name        string
	predicate   Predicate
	trueWork    Work
	falseBranch []Work
}

type parallelRunner struct {
	name   string
	policy workreport.FailurePolicy
	works  []Work
}

// Sequential Sequential 공개 API의 동작을 수행하며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
//
// 매개변수:
//   - name: Sequential가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - policy: Sequential 동작에 필요한 policy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - works: Sequential 동작에 필요한 works 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Sequential(name string, policy workreport.FailurePolicy, works ...Work) Runner {
	return sequentialRunner{
		name:   name,
		policy: policy,
		works:  copyWorks(works),
	}
}

// Conditional Conditional 공개 API의 동작을 수행하며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
//
// 매개변수:
//   - name: Conditional가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - predicate: Conditional 동작에 필요한 predicate 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - trueWork: Conditional 동작에 필요한 trueWork 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - falseWork: Conditional 동작에 필요한 falseWork 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Conditional(name string, predicate Predicate, trueWork Work, falseWork ...Work) Runner {
	return conditionalRunner{
		name:        name,
		predicate:   predicate,
		trueWork:    trueWork,
		falseBranch: copyWorks(falseWork),
	}
}

// Parallel Parallel 공개 API의 동작을 수행하며 workflow 실행 순서, 병렬성, 실패 전파 계약을 보존한다.
//
// 매개변수:
//   - name: Parallel가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - policy: Parallel 동작에 필요한 policy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - works: Parallel 동작에 필요한 works 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Parallel(name string, policy workreport.FailurePolicy, works ...Work) Runner {
	return parallelRunner{
		name:   name,
		policy: policy,
		works:  copyWorks(works),
	}
}

func (r sequentialRunner) Run(ctx context.Context) workreport.Report {
	ctx = normalizeContext(ctx)
	if err := validatePolicy(r.policy); err != nil {
		return workreport.Failed(r.name, err)
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled(r.name, err)
	}

	children := make([]workreport.Report, 0, len(r.works))
	for index, work := range r.works {
		child := runWork(ctx, childName(r.name, index), work)
		children = append(children, child)
		if shouldStopSequential(r.policy, child) {
			return aggregate(r.name, workreport.StopOnFailure, children)
		}
	}
	return aggregate(r.name, r.policy, children)
}

func (r conditionalRunner) Run(ctx context.Context) workreport.Report {
	ctx = normalizeContext(ctx)
	if r.predicate == nil {
		return workreport.Failed(r.name, ErrNilPredicate)
	}
	if len(r.falseBranch) > 1 {
		return workreport.Failed(r.name, ErrTooManyFalseBranches)
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled(r.name, err)
	}

	selected, err := r.predicate(ctx)
	if err != nil {
		if isContextError(err) {
			return workreport.Cancelled(r.name, err)
		}
		return workreport.Failed(r.name, err)
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled(r.name, err)
	}

	if selected {
		child := runWork(ctx, branchName(r.name, "true"), r.trueWork)
		return aggregate(r.name, workreport.StopOnFailure, []workreport.Report{child})
	}
	if len(r.falseBranch) == 0 {
		return workreport.Completed(r.name)
	}

	child := runWork(ctx, branchName(r.name, "false"), r.falseBranch[0])
	return aggregate(r.name, workreport.StopOnFailure, []workreport.Report{child})
}

func (r parallelRunner) Run(ctx context.Context) workreport.Report {
	ctx = normalizeContext(ctx)
	if err := validatePolicy(r.policy); err != nil {
		return workreport.Failed(r.name, err)
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled(r.name, err)
	}
	if len(r.works) == 0 {
		return aggregate(r.name, r.policy, nil)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]workreport.Report, len(r.works))
	var wg sync.WaitGroup
	var stopOnce sync.Once
	var stopCause workreport.Report

	for index, work := range r.works {
		wg.Add(1)
		go func(index int, work Work) {
			defer wg.Done()

			report := runWork(runCtx, childName(r.name, index), work)
			results[index] = report

			if shouldCancelParallel(r.policy, report) {
				stopOnce.Do(func() {
					stopCause = report
					cancel()
				})
			}
		}(index, work)
	}

	wg.Wait()
	if stopCause.IsTerminal() {
		return parentFrom(r.name, stopCause, results)
	}
	return aggregate(r.name, r.policy, results)
}

func runWork(ctx context.Context, name string, work Work) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled(name, err)
	}
	if work == nil {
		return workreport.Failed(name, ErrNilWork)
	}

	report := work(ctx)
	if report.Name == "" {
		report.Name = name
	}
	if !report.IsTerminal() {
		return workreport.Failed(report.Name, ErrUnknownReportStatus)
	}
	return report
}

func shouldStopSequential(policy workreport.FailurePolicy, report workreport.Report) bool {
	if report.IsAborted() || report.IsCancelled() {
		return true
	}
	return policy == workreport.StopOnFailure && !report.IsSuccess()
}

func shouldCancelParallel(policy workreport.FailurePolicy, report workreport.Report) bool {
	if report.IsAborted() || report.IsCancelled() {
		return true
	}
	return policy == workreport.StopOnFailure && !report.IsSuccess()
}

func aggregate(name string, policy workreport.FailurePolicy, children []workreport.Report) workreport.Report {
	report, err := workreport.Aggregate(name, policy, children...)
	if err != nil {
		return workreport.Failed(name, err)
	}
	return report
}

func validatePolicy(policy workreport.FailurePolicy) error {
	_, err := workreport.Aggregate("", policy)
	return err
}

func parentFrom(name string, cause workreport.Report, children []workreport.Report) workreport.Report {
	now := time.Now()
	return workreport.Report{
		Name:      name,
		Status:    cause.Status,
		StartedAt: now,
		EndedAt:   now,
		Err:       cause.Err,
		Reason:    cause.Reason,
		Children:  copyReports(children),
	}
}

func copyReports(reports []workreport.Report) []workreport.Report {
	if len(reports) == 0 {
		return nil
	}
	copied := make([]workreport.Report, len(reports))
	copy(copied, reports)
	return copied
}

func copyWorks(works []Work) []Work {
	if len(works) == 0 {
		return nil
	}
	copied := make([]Work, len(works))
	copy(copied, works)
	return copied
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func childName(name string, index int) string {
	if name == "" {
		return fmt.Sprintf("work[%d]", index)
	}
	return fmt.Sprintf("%s[%d]", name, index)
}

func branchName(name, branch string) string {
	if name == "" {
		return branch
	}
	return name + "." + branch
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
