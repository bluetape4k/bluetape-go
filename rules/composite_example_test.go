package rules_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/rules"
)

func ExampleNewActivationGroup() {
	facts := rules.NewFacts()
	_ = facts.Set("amount", 120)

	small, _ := rules.NewRule(
		"small-discount",
		func(_ context.Context, facts *rules.Facts) (bool, error) {
			value, _ := facts.Get("amount")
			amount, _ := value.(int)
			return amount >= 50, nil
		},
		func(_ context.Context, facts *rules.Facts) error {
			return facts.Set("discount", 5)
		},
		rules.WithPriority(10),
	)
	large, _ := rules.NewRule(
		"large-discount",
		func(_ context.Context, facts *rules.Facts) (bool, error) {
			value, _ := facts.Get("amount")
			amount, _ := value.(int)
			return amount >= 100, nil
		},
		func(_ context.Context, facts *rules.Facts) error {
			return facts.Set("discount", 20)
		},
		rules.WithPriority(1),
	)
	group, _ := rules.NewActivationGroup("discounts", []rules.Rule{small, large})

	set, _ := rules.NewRuleSet(group)
	result, _ := rules.NewEngine(set, rules.EngineConfig{}).Run(context.Background(), facts)

	discount, _ := facts.Get("discount")
	fmt.Println(result.Applied, discount)

	// Output:
	// 1 20
}

func ExampleInferenceEngine_Run() {
	facts := rules.NewFacts()
	_ = facts.Set("count", 0)

	increment, _ := rules.NewRule(
		"increment",
		func(_ context.Context, facts *rules.Facts) (bool, error) {
			value, _ := facts.Get("count")
			count, _ := value.(int)
			return count < 3, nil
		},
		func(_ context.Context, facts *rules.Facts) error {
			value, _ := facts.Get("count")
			count, _ := value.(int)
			return facts.Set("count", count+1)
		},
	)
	set, _ := rules.NewRuleSet(increment)
	engine, _ := rules.NewInferenceEngine(set, rules.InferenceConfig{MaxCycles: 5})

	result, err := engine.Run(context.Background(), facts)
	if err != nil {
		var inferenceErr rules.InferenceError
		if errors.As(err, &inferenceErr) {
			fmt.Println("non-converged", inferenceErr.Cycles)
		}
		return
	}

	count, _ := facts.Get("count")
	fmt.Println(result.Cycles, result.Applied, count)

	// Output:
	// 4 3 3
}
