package rules_test

import (
	"context"
	"fmt"

	"github.com/bluetape4k/bluetape-go/rules"
)

func ExampleEngine_Run() {
	facts := rules.NewFacts()
	_ = facts.Set("amount", 120)

	discount, _ := rules.NewRule(
		"discount",
		func(_ context.Context, facts *rules.Facts) (bool, error) {
			value, ok := facts.Get("amount")
			if !ok {
				return false, nil
			}
			amount, ok := value.(int)
			return ok && amount >= 100, nil
		},
		func(_ context.Context, facts *rules.Facts) error {
			return facts.Set("discount", 10)
		},
		rules.WithPriority(10),
		rules.WithDescription("apply a threshold discount"),
	)

	set, _ := rules.NewRuleSet(discount)
	result, err := rules.NewEngine(set, rules.EngineConfig{
		StopOnFirstFailed: true,
	}).Run(context.Background(), facts)
	if err != nil {
		return
	}

	value, _ := facts.Get("discount")
	fmt.Println(result.Applied, value)

	// Output:
	// 1 10
}
