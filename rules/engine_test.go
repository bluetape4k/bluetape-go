package rules

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestEngineRunsRulesDeterministicallyAndReportsDetails(t *testing.T) {
	var order []string
	lowB := ruleWithHooks(t, "b-low", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:b")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:b")
			return nil
		},
	)
	lowA := ruleWithHooks(t, "a-low", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:a")
			return false, nil
		},
		nil,
	)
	high := ruleWithHooks(t, "high", 5,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:high")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:high")
			return nil
		},
	)
	set, err := NewRuleSet(high, lowB, lowA)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}

	result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), NewFacts())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if fmt.Sprint(order) != "[eval:a eval:b exec:b eval:high exec:high]" {
		t.Fatalf("order = %v", order)
	}
	if result.Applied != 2 || result.NotTriggered != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v", result)
	}
	if result.Details[0].Status != StatusNotTriggered || result.Details[1].Status != StatusApplied {
		t.Fatalf("details = %+v", result.Details)
	}
}

func TestEngineConfigStopPoliciesAndPriorityThreshold(t *testing.T) {
	set, err := NewRuleSet(
		mustRule(t, "apply", 1, true, nil),
		mustRule(t, "skip", 2, false, nil),
		mustRule(t, "too-high", 9, true, nil),
	)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}

	result, err := NewEngine(set, EngineConfig{StopOnFirstApplied: true}).Run(context.Background(), NewFacts())
	if err != nil {
		t.Fatalf("run stop applied: %v", err)
	}
	if !result.Stopped || result.StopReason != StatusApplied || len(result.Details) != 1 {
		t.Fatalf("stop applied result = %+v", result)
	}

	result, err = NewEngine(set, EngineConfig{
		PriorityThreshold:    2,
		UsePriorityThreshold: true,
	}).Run(context.Background(), NewFacts())
	if err != nil {
		t.Fatalf("run threshold: %v", err)
	}
	if result.Skipped != 1 || result.Details[len(result.Details)-1].Status != StatusSkipped {
		t.Fatalf("threshold result = %+v", result)
	}

	result, err = NewEngine(set, EngineConfig{StopOnFirstNotTriggered: true}).Run(context.Background(), NewFacts())
	if err != nil {
		t.Fatalf("run stop not triggered: %v", err)
	}
	if !result.Stopped || result.StopReason != StatusNotTriggered {
		t.Fatalf("stop not triggered result = %+v", result)
	}
}

func TestEngineErrorPolicyAndTypedErrors(t *testing.T) {
	evalErr := errors.New("bad predicate")
	execErr := errors.New("bad action")
	evalRule := ruleWithHooks(t, "eval-error", 1,
		func(context.Context, *Facts) (bool, error) { return false, evalErr },
		nil,
	)
	execRule := ruleWithHooks(t, "exec-error", 2,
		func(context.Context, *Facts) (bool, error) { return true, nil },
		func(context.Context, *Facts) error { return execErr },
	)
	set, err := NewRuleSet(evalRule, execRule)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}

	result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), NewFacts())
	if !errors.Is(err, ErrRuleEvaluation) || !errors.Is(err, ErrRuleExecution) {
		t.Fatalf("continue-on-error err = %v, want evaluation and execution sentinels", err)
	}
	if result.Failed != 2 || len(result.Details) != 2 {
		t.Fatalf("continue result = %+v", result)
	}
	if !errors.Is(result.Details[0].Err, ErrRuleEvaluation) || !errors.Is(result.Details[0].Err, evalErr) {
		t.Fatalf("eval detail err = %v", result.Details[0].Err)
	}
	if !errors.Is(result.Details[1].Err, ErrRuleExecution) || !errors.Is(result.Details[1].Err, execErr) {
		t.Fatalf("exec detail err = %v", result.Details[1].Err)
	}

	result, err = NewEngine(set, EngineConfig{StopOnFirstFailed: true}).Run(context.Background(), NewFacts())
	if !errors.Is(err, ErrRuleEvaluation) || !errors.Is(err, evalErr) {
		t.Fatalf("stop error = %v", err)
	}
	if !result.Stopped || result.Failed != 1 {
		t.Fatalf("stop result = %+v", result)
	}
}

func TestEngineFailureAfterFactMutationReturnsError(t *testing.T) {
	execErr := errors.New("partial mutation")
	mutatingRule := ruleWithHooks(t, "mutating-error", 1,
		func(context.Context, *Facts) (bool, error) {
			return true, nil
		},
		func(_ context.Context, facts *Facts) error {
			if err := facts.Set("decision", "partial"); err != nil {
				return err
			}
			return execErr
		},
	)
	set, err := NewRuleSet(mutatingRule)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	facts := NewFacts()

	result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), facts)
	if !errors.Is(err, ErrRuleExecution) || !errors.Is(err, execErr) {
		t.Fatalf("err = %v, want execution sentinel and original error", err)
	}
	if result.Failed != 1 || result.Details[0].Status != StatusExecutionFailed {
		t.Fatalf("result = %+v", result)
	}
	value, ok := facts.Get("decision")
	if !ok || value != "partial" {
		t.Fatalf("decision fact = %v, %v", value, ok)
	}
}

func TestEngineCancellationBeforeEvaluationAndExecution(t *testing.T) {
	neverRun := false
	set, err := NewRuleSet(ruleWithHooks(t, "cancelled", 1,
		func(context.Context, *Facts) (bool, error) {
			neverRun = true
			return true, nil
		},
		nil,
	))
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewEngine(set, EngineConfig{}).Run(ctx, NewFacts())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-eval err = %v, want context.Canceled", err)
	}
	if neverRun {
		t.Fatal("evaluation should not run after cancellation")
	}

	execCtx, execCancel := context.WithCancel(context.Background())
	set, err = NewRuleSet(ruleWithHooks(t, "cancel-before-exec", 1,
		func(context.Context, *Facts) (bool, error) {
			execCancel()
			return true, nil
		},
		func(context.Context, *Facts) error {
			t.Fatal("execute should not run after cancellation")
			return nil
		},
	))
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	result, err := NewEngine(set, EngineConfig{}).Run(execCtx, NewFacts())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-exec err = %v, want context.Canceled", err)
	}
	if !result.Stopped || result.StopReason != StatusCancelled {
		t.Fatalf("cancel result = %+v", result)
	}
}

func TestEngineCancellationReturnedFromRuleShortCircuits(t *testing.T) {
	ranAfterCancel := false
	set, err := NewRuleSet(
		ruleWithHooks(t, "deadline", 1,
			func(context.Context, *Facts) (bool, error) {
				return false, context.DeadlineExceeded
			},
			nil,
		),
		ruleWithHooks(t, "after", 2,
			func(context.Context, *Facts) (bool, error) {
				ranAfterCancel = true
				return true, nil
			},
			nil,
		),
	)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}

	result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), NewFacts())
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrRuleEvaluation) {
		t.Fatalf("err = %v, want deadline and evaluation sentinel", err)
	}
	if ranAfterCancel {
		t.Fatal("engine should stop after rule returns context deadline")
	}
	if !result.Stopped || result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.StopReason != StatusCancelled {
		t.Fatalf("stop reason = %q, want %q", result.StopReason, StatusCancelled)
	}
}

func TestEngineCancellationWithAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 25,
		Timeout:       2 * time.Second,
	})
	set, err := NewRuleSet(ruleWithHooks(t, "cancel", 1,
		func(ctx context.Context, _ *Facts) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		},
		nil,
	))
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	engine := NewEngine(set, EngineConfig{StopOnFirstFailed: true})

	tester.RunT(t, func(ctx context.Context) error {
		runCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := engine.Run(runCtx, NewFacts())
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("err = %w, want context.Canceled", err)
		}
		return nil
	})
}

func ruleWithHooks(t testing.TB, name string, priority int, evaluate EvaluateFunc, execute ExecuteFunc) Rule {
	t.Helper()
	rule, err := NewRule(name, evaluate, execute, WithPriority(priority))
	if err != nil {
		t.Fatalf("new rule %s: %v", name, err)
	}
	return rule
}
