package rules

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestRuleSetRegistrationLookupRemovalAndOrdering(t *testing.T) {
	third := mustRule(t, "z-third", 10, true, nil)
	first := mustRule(t, "a-first", 1, true, nil)
	second := mustRule(t, "b-second", 1, true, nil)

	set, err := NewRuleSet(third, first, second)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	if err := set.Add(first); !errors.Is(err, ErrDuplicateRule) {
		t.Fatalf("duplicate err = %v, want ErrDuplicateRule", err)
	}

	ordered := set.Rules()
	names := []string{ordered[0].Name(), ordered[1].Name(), ordered[2].Name()}
	if fmt.Sprint(names) != "[a-first b-second z-third]" {
		t.Fatalf("ordered names = %v", names)
	}
	if rule, ok := set.Get("a-first"); !ok || rule.Name() != "a-first" {
		t.Fatalf("lookup = %v/%v, want a-first/true", rule, ok)
	}
	if !set.Remove("a-first") || set.Len() != 2 {
		t.Fatalf("remove failed, len=%d", set.Len())
	}
}

func TestNewRuleValidationAndDefaults(t *testing.T) {
	if _, err := NewRule(" ", func(context.Context, *Facts) (bool, error) { return true, nil }, nil); !errors.Is(err, ErrBlankKey) {
		t.Fatalf("blank name err = %v, want ErrBlankKey", err)
	}
	if _, err := NewRule("missing-eval", nil, nil); !errors.Is(err, ErrNilEvaluate) {
		t.Fatalf("nil evaluate err = %v, want ErrNilEvaluate", err)
	}

	rule := mustRule(t, "noop", 0, true, nil)
	if err := rule.Execute(context.Background(), NewFacts()); err != nil {
		t.Fatalf("nil execute should be no-op: %v", err)
	}
}

func TestRuleSetNilReceiverAndRuleErrorZeroValue(t *testing.T) {
	var set *RuleSet
	if err := set.Add(mustRule(t, "noop", 0, true, nil)); !errors.Is(err, ErrNilRuleSet) {
		t.Fatalf("nil ruleset err = %v, want ErrNilRuleSet", err)
	}

	if got := (RuleError{Phase: PhaseEvaluate}).Error(); got != "rules evaluate failed" {
		t.Fatalf("zero rule error = %q", got)
	}
}

func TestRuleSetAndEngineUseCachedMetadataAfterRegistration(t *testing.T) {
	rule := &mutableMetadataRule{name: "cached", priority: 7}
	set, err := NewRuleSet(rule)
	if err != nil {
		t.Fatalf("new ruleset: %v", err)
	}
	rule.freeze()

	if got := len(set.Rules()); got != 1 {
		t.Fatalf("rules len = %d, want 1", got)
	}
	result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), NewFacts())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Details[0].RuleName != "cached" || result.Details[0].Priority != 7 {
		t.Fatalf("detail = %+v", result.Details[0])
	}
}

func mustRule(t testing.TB, name string, priority int, triggered bool, execute ExecuteFunc) Rule {
	t.Helper()
	rule, err := NewRule(
		name,
		func(context.Context, *Facts) (bool, error) { return triggered, nil },
		execute,
		WithPriority(priority),
		WithDescription(name+" description"),
	)
	if err != nil {
		t.Fatalf("new rule %s: %v", name, err)
	}
	return rule
}

type mutableMetadataRule struct {
	name     string
	priority int
	frozen   atomic.Bool
}

func (r *mutableMetadataRule) freeze() {
	r.frozen.Store(true)
}

func (r *mutableMetadataRule) Name() string {
	if r.frozen.Load() {
		panic("Name called after registration")
	}
	return r.name
}

func (r *mutableMetadataRule) Description() string {
	return ""
}

func (r *mutableMetadataRule) Priority() int {
	if r.frozen.Load() {
		panic("Priority called after registration")
	}
	return r.priority
}

func (r *mutableMetadataRule) Evaluate(context.Context, *Facts) (bool, error) {
	return true, nil
}

func (r *mutableMetadataRule) Execute(context.Context, *Facts) error {
	return nil
}
