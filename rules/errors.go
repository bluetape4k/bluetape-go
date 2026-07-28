package rules

import (
	"errors"
	"strconv"
)

var (
	// ErrBlankKey reports a blank Facts key or rule name.
	ErrBlankKey = errors.New("rules key must not be blank")
	// ErrNilFacts reports a nil Facts pointer passed to an API that needs facts.
	ErrNilFacts = errors.New("rules facts must not be nil")
	// ErrNilRuleSet reports a nil RuleSet receiver.
	ErrNilRuleSet = errors.New("rules rule set must not be nil")
	// ErrNilRule reports a nil Rule passed to a RuleSet.
	ErrNilRule = errors.New("rules rule must not be nil")
	// ErrDuplicateRule reports a duplicate rule name registration.
	ErrDuplicateRule = errors.New("rules rule name already exists")
	// ErrNilEvaluate reports a functional rule without an evaluate function.
	ErrNilEvaluate = errors.New("rules evaluate function must not be nil")
	// ErrRuleEvaluation reports a rule evaluation failure.
	ErrRuleEvaluation = errors.New("rules evaluation failed")
	// ErrRuleExecution reports a rule execution failure.
	ErrRuleExecution = errors.New("rules execution failed")
	// ErrEmptyRuleGroup reports a composite group without child rules.
	ErrEmptyRuleGroup = errors.New("rules group must contain at least one rule")
	// ErrCompositeNotTriggered reports a composite Execute with no executable child.
	ErrCompositeNotTriggered = errors.New("rules composite did not trigger")
	// ErrGuardRuleMissing reports a conditional group without its guard rule.
	ErrGuardRuleMissing = errors.New("rules guard rule not found")
	// ErrInvalidMaxCycles reports an invalid inference max-cycle limit.
	ErrInvalidMaxCycles = errors.New("rules inference max cycles must be positive")
	// ErrInvalidInferenceConfig reports an inference option that breaks convergence checks.
	ErrInvalidInferenceConfig = errors.New("rules inference config is invalid")
	// ErrInferenceNonConverged reports inference that exceeded its cycle limit.
	ErrInferenceNonConverged = errors.New("rules inference did not converge")
)

// RulePhase string 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RulePhase string

const (
	// PhaseEvaluate identifies rule predicate evaluation.
	PhaseEvaluate RulePhase = "evaluate"
	// PhaseExecute identifies rule action execution.
	PhaseExecute RulePhase = "execute"
)

// RuleError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RuleError struct {
	RuleName string
	Phase    RulePhase
	Err      error
}

// Error Error 공개 API의 동작을 수행한다.
func (e RuleError) Error() string {
	if e.Err == nil {
		if e.RuleName == "" {
			return "rules " + string(e.Phase) + " failed"
		}
		return "rules " + string(e.Phase) + " failed for " + e.RuleName
	}
	if e.RuleName == "" {
		return "rules " + string(e.Phase) + " failed: " + e.Err.Error()
	}
	return "rules " + string(e.Phase) + " failed for " + e.RuleName + ": " + e.Err.Error()
}

// Unwrap Unwrap 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (e RuleError) Unwrap() error {
	return e.Err
}

// Is Is 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e RuleError) Is(target error) bool {
	return e.Phase == PhaseEvaluate && target == ErrRuleEvaluation ||
		e.Phase == PhaseExecute && target == ErrRuleExecution
}

// InferenceError struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type InferenceError struct {
	Cycles int
	Err    error
}

// Error Error 공개 API의 동작을 수행한다.
func (e InferenceError) Error() string {
	if e.Err == nil {
		return "rules inference failed"
	}
	return "rules inference failed after " + strconv.Itoa(e.Cycles) + " cycles: " + e.Err.Error()
}

// Unwrap Unwrap 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (e InferenceError) Unwrap() error {
	return e.Err
}
