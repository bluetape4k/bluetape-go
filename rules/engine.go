package rules

import (
	"context"
	"errors"
	"fmt"
)

// EngineConfig struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// Engine struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Engine struct {
	rules  *RuleSet
	config EngineConfig
}

// NewEngine NewEngine 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - rules: NewEngine 동작에 필요한 rules 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - config: NewEngine 동작에 필요한 config 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func NewEngine(rules *RuleSet, config EngineConfig) *Engine {
	if rules == nil {
		rules = &RuleSet{byName: make(map[string]ruleEntry)}
	}
	return &Engine{rules: rules, config: config}
}

// DetailStatus string 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// Detail struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Detail struct {
	RuleName  string
	Priority  int
	Status    DetailStatus
	Triggered bool
	Applied   bool
	Err       error
}

// Result struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Result struct {
	Details      []Detail
	Applied      int
	Failed       int
	NotTriggered int
	Skipped      int
	Stopped      bool
	StopReason   DetailStatus
}

// Run Run 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - facts: Run 동작에 필요한 facts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
