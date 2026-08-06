package rules

import (
	"context"
	"errors"
	"fmt"
)

// EngineConfig 패키지에서 공개하는 구조체다.
type EngineConfig struct {
	// StopOnFirstApplied stops after the first rule whose Execute succeeds.
	StopOnFirstApplied bool
	// StopOnFirstFailed stops after the first evaluation or execution failure.
	StopOnFirstFailed bool
	// StopOnFirstNotTriggered stops after the first rule that evaluates false.
	StopOnFirstNotTriggered bool
	// PriorityThreshold limits execution to rules whose priority is <= this value
	// when UsePriorityThreshold is true.
	PriorityThreshold int
	// UsePriorityThreshold enables PriorityThreshold.
	UsePriorityThreshold bool
}

// Engine 패키지에서 공개하는 구조체다.
type Engine struct {
	rules  *RuleSet
	config EngineConfig
}

// NewEngine Engine 인스턴스를 생성한다.
//
// 매개변수:
//   - rules: NewEngine에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - config: NewEngine에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func NewEngine(rules *RuleSet, config EngineConfig) *Engine {
	if rules == nil {
		rules = &RuleSet{byName: make(map[string]ruleEntry)}
	}
	return &Engine{rules: rules, config: config}
}

// DetailStatus string 공개 타입이다.
type DetailStatus string

const (
	// StatusApplied means the rule evaluated true and executed successfully.
	StatusApplied DetailStatus = "applied"
	// StatusCancelled means the engine stopped because the context was cancelled.
	StatusCancelled DetailStatus = "cancelled"
	// StatusNotTriggered means the rule evaluated false.
	StatusNotTriggered DetailStatus = "not_triggered"
	// StatusEvaluationFailed means Evaluate returned an error.
	StatusEvaluationFailed DetailStatus = "evaluation_failed"
	// StatusExecutionFailed means Execute returned an error.
	StatusExecutionFailed DetailStatus = "execution_failed"
	// StatusNonConverged means bounded inference exceeded its cycle limit.
	StatusNonConverged DetailStatus = "non_converged"
	// StatusSkipped means the rule did not run because of engine configuration.
	StatusSkipped DetailStatus = "skipped"
)

// Detail 패키지에서 공개하는 구조체다.
type Detail struct {
	RuleName  string
	Priority  int
	Status    DetailStatus
	Triggered bool
	Applied   bool
	Err       error
}

// Result 패키지에서 공개하는 구조체다.
type Result struct {
	Details      []Detail
	Applied      int
	Failed       int
	NotTriggered int
	Skipped      int
	Stopped      bool
	StopReason   DetailStatus
}

// Run 작업을 실행하고 완료 또는 오류를 반환한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - facts: Run에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (e *Engine) Run(ctx context.Context, facts *Facts) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if facts == nil {
		return Result{}, ErrNilFacts
	}

	var result Result
	for _, entry := range e.rules.entries() {
		rule := entry.rule
		if e.config.UsePriorityThreshold && entry.priority > e.config.PriorityThreshold {
			result.Skipped++
			result.Details = append(result.Details, Detail{
				RuleName: entry.name,
				Priority: entry.priority,
				Status:   StatusSkipped,
			})
			continue
		}

		if err := ctx.Err(); err != nil {
			result.Stopped = true
			result.StopReason = StatusCancelled
			return result, wrapContextError("before evaluation", entry.name, err)
		}

		triggered, err := rule.Evaluate(ctx, facts)
		if err != nil {
			wrapped := RuleError{RuleName: entry.name, Phase: PhaseEvaluate, Err: err}
			result.Failed++
			result.Details = append(result.Details, Detail{
				RuleName: entry.name,
				Priority: entry.priority,
				Status:   StatusEvaluationFailed,
				Err:      wrapped,
			})
			if isContextError(err) {
				result.Stopped = true
				result.StopReason = StatusCancelled
				return result, wrapped
			}
			if e.config.StopOnFirstFailed {
				result.Stopped = true
				result.StopReason = StatusEvaluationFailed
				return result, wrapped
			}
			continue
		}
		if !triggered {
			result.NotTriggered++
			result.Details = append(result.Details, Detail{
				RuleName: entry.name,
				Priority: entry.priority,
				Status:   StatusNotTriggered,
			})
			if e.config.StopOnFirstNotTriggered {
				result.Stopped = true
				result.StopReason = StatusNotTriggered
				return result, nil
			}
			continue
		}

		if err := ctx.Err(); err != nil {
			result.Stopped = true
			result.StopReason = StatusCancelled
			return result, wrapContextError("before execution", entry.name, err)
		}

		if err := rule.Execute(ctx, facts); err != nil {
			wrapped := RuleError{RuleName: entry.name, Phase: PhaseExecute, Err: err}
			result.Failed++
			result.Details = append(result.Details, Detail{
				RuleName:  entry.name,
				Priority:  entry.priority,
				Status:    StatusExecutionFailed,
				Triggered: true,
				Err:       wrapped,
			})
			if isContextError(err) {
				result.Stopped = true
				result.StopReason = StatusCancelled
				return result, wrapped
			}
			if e.config.StopOnFirstFailed {
				result.Stopped = true
				result.StopReason = StatusExecutionFailed
				return result, wrapped
			}
			continue
		}

		result.Applied++
		result.Details = append(result.Details, Detail{
			RuleName:  entry.name,
			Priority:  entry.priority,
			Status:    StatusApplied,
			Triggered: true,
			Applied:   true,
		})
		if e.config.StopOnFirstApplied {
			result.Stopped = true
			result.StopReason = StatusApplied
			return result, nil
		}
	}
	if err := resultError(result); err != nil {
		return result, err
	}
	return result, nil
}

func wrapContextError(phase, ruleName string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("rules context canceled %s for %s: %w", phase, ruleName, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("rules context deadline exceeded %s for %s: %w", phase, ruleName, err)
	}
	return fmt.Errorf("rules context failed %s for %s: %w", phase, ruleName, err)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func resultError(result Result) error {
	if result.Failed == 0 {
		return nil
	}
	errs := make([]error, 0, result.Failed)
	for _, detail := range result.Details {
		if detail.Err != nil {
			errs = append(errs, detail.Err)
		}
	}
	return errors.Join(errs...)
}
