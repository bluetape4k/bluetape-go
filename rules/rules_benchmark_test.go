package rules

import (
	"context"
	"strconv"
	"testing"
)

func BenchmarkRulesCompositeActivationFirstMatch(b *testing.B) {
	ctx := context.Background()
	facts := NewFacts()
	_ = facts.Set("score", 75)
	_ = facts.Set("segment", "standard")

	group, err := NewActivationGroup("discounts", []Rule{
		benchmarkRule(b, "high-value", 1,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("score")
				score, _ := value.(int)
				return score >= 90, nil
			},
			nil,
		),
		benchmarkRule(b, "standard", 2,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("segment")
				segment, _ := value.(string)
				return segment == "standard", nil
			},
			func(_ context.Context, facts *Facts) error {
				return facts.Set("discount", 10)
			},
		),
		benchmarkRule(b, "fallback", 3,
			func(context.Context, *Facts) (bool, error) { return true, nil },
			func(_ context.Context, facts *Facts) error {
				return facts.Set("discount", 1)
			},
		),
	})
	if err != nil {
		b.Fatalf("new activation group: %v", err)
	}
	validateBenchmarkRule(ctx, b, facts, group)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := group.Execute(ctx, facts); err != nil {
			b.Fatalf("execute activation group: %v", err)
		}
	}
}

func BenchmarkRulesCompositeUnitAllMatch(b *testing.B) {
	ctx := context.Background()
	facts := NewFacts()
	_ = facts.Set("score", 91)
	_ = facts.Set("verified", true)

	group, err := NewUnitGroup("eligibility", []Rule{
		benchmarkRule(b, "minimum-score", 1,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("score")
				score, _ := value.(int)
				return score >= 90, nil
			},
			func(_ context.Context, facts *Facts) error {
				return facts.Set("score-qualified", true)
			},
		),
		benchmarkRule(b, "verified", 2,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("verified")
				verified, _ := value.(bool)
				return verified, nil
			},
			func(_ context.Context, facts *Facts) error {
				return facts.Set("verified-qualified", true)
			},
		),
	})
	if err != nil {
		b.Fatalf("new unit group: %v", err)
	}
	validateBenchmarkRule(ctx, b, facts, group)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := group.Execute(ctx, facts); err != nil {
			b.Fatalf("execute unit group: %v", err)
		}
	}
}

func BenchmarkRulesCompositeConditionalDependents(b *testing.B) {
	ctx := context.Background()
	facts := NewFacts()
	_ = facts.Set("enabled", true)
	_ = facts.Set("score", 72)

	group, err := NewConditionalGroup("conditional-discounts", "enabled", []Rule{
		benchmarkRule(b, "enabled", 0,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("enabled")
				enabled, _ := value.(bool)
				return enabled, nil
			},
			nil,
		),
		benchmarkRule(b, "score-discount", 1,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("score")
				score, _ := value.(int)
				return score >= 70, nil
			},
			func(_ context.Context, facts *Facts) error {
				return facts.Set("score-discount", 5)
			},
		),
		benchmarkRule(b, "premium-discount", 2,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("segment")
				segment, _ := value.(string)
				return segment == "premium", nil
			},
			func(_ context.Context, facts *Facts) error {
				return facts.Set("premium-discount", 15)
			},
		),
	})
	if err != nil {
		b.Fatalf("new conditional group: %v", err)
	}
	validateBenchmarkRule(ctx, b, facts, group)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := group.Execute(ctx, facts); err != nil {
			b.Fatalf("execute conditional group: %v", err)
		}
	}
}

func BenchmarkRulesInferenceConverges(b *testing.B) {
	ctx := context.Background()
	increment := benchmarkRule(b, "increment-count", 1,
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
	markReady := benchmarkRule(b, "mark-ready", 2,
		func(_ context.Context, facts *Facts) (bool, error) {
			value, _ := facts.Get("count")
			count, _ := value.(int)
			return count >= 3 && !facts.Has("ready"), nil
		},
		func(_ context.Context, facts *Facts) error {
			return facts.Set("ready", true)
		},
	)
	set, err := NewRuleSet(increment, markReady)
	if err != nil {
		b.Fatalf("new ruleset: %v", err)
	}
	engine, err := NewInferenceEngine(set, InferenceConfig{MaxCycles: 8})
	if err != nil {
		b.Fatalf("new inference engine: %v", err)
	}
	facts := NewFacts()
	_ = facts.Set("count", 0)
	result, err := engine.Run(ctx, facts)
	if err != nil {
		b.Fatalf("validate inference: %v", err)
	}
	if !result.Converged || result.Applied != 4 {
		b.Fatalf("validate inference result = %+v, want converged with 4 applied rules", result)
	}

	for _, tt := range []struct {
		name        string
		initial     int
		wantApplied int
	}{
		{name: "Count0", initial: 0, wantApplied: 4},
		{name: "Count1", initial: 1, wantApplied: 3},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				facts := NewFacts()
				_ = facts.Set("count", tt.initial)
				result, err := engine.Run(ctx, facts)
				if err != nil {
					b.Fatalf("run inference: %v", err)
				}
				if !result.Converged || result.Applied != tt.wantApplied {
					b.Fatalf("result = %+v, want converged with %d applied rules", result, tt.wantApplied)
				}
			}
		})
	}
}

func BenchmarkRulesEngineSequentialRun(b *testing.B) {
	ctx := context.Background()
	rules := make([]Rule, 0, 16)
	for i := 0; i < 16; i++ {
		key := "rule-" + strconv.Itoa(i)
		rules = append(rules, benchmarkRule(b, key, i,
			func(_ context.Context, facts *Facts) (bool, error) {
				value, _ := facts.Get("enabled")
				enabled, _ := value.(bool)
				return enabled, nil
			},
			func(_ context.Context, facts *Facts) error {
				value, _ := facts.Get("applied")
				applied, _ := value.(int)
				return facts.Set("applied", applied+1)
			},
		))
	}
	set, err := NewRuleSet(rules...)
	if err != nil {
		b.Fatalf("new ruleset: %v", err)
	}
	engine := NewEngine(set, EngineConfig{})
	facts := NewFacts()
	_ = facts.Set("enabled", true)
	result, err := engine.Run(ctx, facts)
	if err != nil {
		b.Fatalf("validate engine run: %v", err)
	}
	if result.Applied != len(rules) {
		b.Fatalf("validate engine result applied = %d, want %d", result.Applied, len(rules))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		facts := NewFacts()
		_ = facts.Set("enabled", true)
		result, err := engine.Run(ctx, facts)
		if err != nil {
			b.Fatalf("run engine: %v", err)
		}
		if result.Applied != len(rules) {
			b.Fatalf("applied = %d, want %d", result.Applied, len(rules))
		}
	}
}

func benchmarkRule(tb testing.TB, name string, priority int, evaluate EvaluateFunc, execute ExecuteFunc) Rule {
	tb.Helper()
	rule, err := NewRule(name, evaluate, execute, WithPriority(priority))
	if err != nil {
		tb.Fatalf("new benchmark rule %q: %v", name, err)
	}
	return rule
}

func validateBenchmarkRule(ctx context.Context, tb testing.TB, facts *Facts, rule Rule) {
	tb.Helper()
	triggered, err := rule.Evaluate(ctx, facts)
	if err != nil {
		tb.Fatalf("validate %s evaluate: %v", rule.Name(), err)
	}
	if !triggered {
		tb.Fatalf("validate %s evaluate = false, want true", rule.Name())
	}
	if err := rule.Execute(ctx, facts); err != nil {
		tb.Fatalf("validate %s execute: %v", rule.Name(), err)
	}
	if facts.Len() == 0 {
		tb.Fatal("validate " + rule.Name() + " left facts empty")
	}
}
