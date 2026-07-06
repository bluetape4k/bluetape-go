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

// RulePhase identifies which engine phase produced an error.
type RulePhase string

const (
	// PhaseEvaluate identifies rule predicate evaluation.
	PhaseEvaluate RulePhase = "evaluate"
	// PhaseExecute identifies rule action execution.
	PhaseExecute RulePhase = "execute"
)

// RuleError wraps an evaluation or execution error with rule context.
type RuleError struct {
	RuleName string
	Phase    RulePhase
	Err      error
}

// Error returns a human-readable rule error message.
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

// Unwrap returns the underlying cause.
func (e RuleError) Unwrap() error {
	return e.Err
}

// Is reports whether the error matches the phase sentinel.
func (e RuleError) Is(target error) bool {
	return e.Phase == PhaseEvaluate && target == ErrRuleEvaluation ||
		e.Phase == PhaseExecute && target == ErrRuleExecution
}

// InferenceError reports bounded inference failure details.
type InferenceError struct {
	Cycles int
	Err    error
}

// Error returns a human-readable inference error message.
func (e InferenceError) Error() string {
	if e.Err == nil {
		return "rules inference failed"
	}
	return "rules inference failed after " + strconv.Itoa(e.Cycles) + " cycles: " + e.Err.Error()
}

// Unwrap returns the underlying inference cause.
func (e InferenceError) Unwrap() error {
	return e.Err
}
