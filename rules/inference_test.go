package rules

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestInferenceEngineConvergesWhenNoRulesMatch(t *testing.T) {
	increment, err := NewRule(
		"increment",
		func(_ context.Context, facts *Facts) (bool, error) {
			value, _ := facts.Get("count")
			count, _ := value.(int)
			return count < 3, nil
		},
		func(_ context.Context, facts *Facts) error {
			value, _ := facts.Get("count")
			count, _ := value.(int)
			return facts.Set("count", count+1)
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	set, err := NewRuleSet(increment)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 5})
	if err != nil {
		t.Fatalf("new inference engine: %v", err)
	}
	facts := NewFacts()
	_ = facts.Set("count", 0)

	result, err := engine.Run(context.Background(), facts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Converged || result.Cycles != 4 || result.Applied != 3 {
		t.Fatalf("result = %+v", result)
	}
	value, _ := facts.Get("count")
	if value != 3 {
		t.Fatalf("count = %v, want 3", value)
	}
}

func TestInferenceEngineReturnsTypedNonConvergenceError(t *testing.T) {
	always, err := NewRule(
		"always",
		func(context.Context, *Facts) (bool, error) { return true, nil },
		func(_ context.Context, facts *Facts) error {
			value, _ := facts.Get("attempts")
			attempts, _ := value.(int)
			return facts.Set("attempts", attempts+1)
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	set, err := NewRuleSet(always)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 2})
	if err != nil {
		t.Fatalf("new inference engine: %v", err)
	}

	facts := NewFacts()
	_ = facts.Set("attempts", 0)
	result, err := engine.Run(context.Background(), facts)
	if !errors.Is(err, ErrInferenceNonConverged) {
		t.Fatalf("err = %v, want ErrInferenceNonConverged", err)
	}
	var inferenceErr InferenceError
	if !errors.As(err, &inferenceErr) || inferenceErr.Cycles != 2 {
		t.Fatalf("inference err = %#v", err)
	}
	if result.Converged || result.Cycles != 2 || result.Applied != 2 {
		t.Fatalf("result = %+v", result)
	}
	if !result.Stopped || result.StopReason != StatusNonConverged {
		t.Fatalf("stop state = %v/%q, want stopped/non_converged", result.Stopped, result.StopReason)
	}
	value, _ := facts.Get("attempts")
	if value != 2 {
		t.Fatalf("attempts = %v, want retained partial mutations", value)
	}
}

func TestInferenceEngineRejectsStopOnFirstNotTriggered(t *testing.T) {
	skipped, err := NewRule(
		"a-skipped",
		func(context.Context, *Facts) (bool, error) { return false, nil },
		nil,
	)
	if err != nil {
		t.Fatalf("new skipped rule: %v", err)
	}
	applicable, err := NewRule(
		"b-applicable",
		func(context.Context, *Facts) (bool, error) { return true, nil },
		func(_ context.Context, facts *Facts) error { return facts.Set("applied", true) },
	)
	if err != nil {
		t.Fatalf("new applicable rule: %v", err)
	}
	set, err := NewRuleSet(skipped, applicable)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}

	_, err = NewInferenceEngine(set, InferenceConfig{
		MaxCycles: 1,
		EngineConfig: EngineConfig{
			StopOnFirstNotTriggered: true,
		},
	})
	if !errors.Is(err, ErrInvalidInferenceConfig) {
		t.Fatalf("err = %v, want ErrInvalidInferenceConfig", err)
	}
}

func TestInferenceEngineValidationFailureAndCancellation(t *testing.T) {
	if _, err := NewInferenceEngine(nil, InferenceConfig{}); !errors.Is(err, ErrInvalidMaxCycles) {
		t.Fatalf("invalid max cycles err = %v, want ErrInvalidMaxCycles", err)
	}

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
	engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 3})
	if err != nil {
		t.Fatalf("new inference engine: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := engine.Run(ctx, NewFacts())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if !result.Stopped || result.StopReason != StatusCancelled {
		t.Fatalf("result = %+v", result)
	}
}

func TestInferenceEngineCancellationWithAsyncJobTester(t *testing.T) {
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
	engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 2})
	if err != nil {
		t.Fatalf("new inference engine: %v", err)
	}

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

func TestInferenceEngineCycleStress(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 25,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		increment, err := NewRule(
			"increment",
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("count")
				count, _ := value.(int)
				return count < 2, nil
			},
			func(_ context.Context, facts *Facts) error {
				value, _ := facts.Get("count")
				count, _ := value.(int)
				return facts.Set("count", count+1)
			},
		)
		if err != nil {
			return err
		}
		set, err := NewRuleSet(increment)
		if err != nil {
			return err
		}
		engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 4})
		if err != nil {
			return err
		}
		facts := NewFacts()
		_ = facts.Set("count", 0)
		result, err := engine.Run(context.Background(), facts)
		if err != nil {
			return err
		}
		if !result.Converged || result.Applied != 2 {
			return fmt.Errorf("result = %+v", result)
		}
		return nil
	})
}
