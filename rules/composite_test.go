package rules

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestActivationGroupExecutesOnlyFirstMatchingRuleDeterministically(t *testing.T) {
	var order []string
	first := ruleWithHooks(t, "a-first", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:first")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:first")
			return nil
		},
	)
	second := ruleWithHooks(t, "b-second", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:second")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:second")
			return nil
		},
	)
	high := ruleWithHooks(t, "high", 10,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:high")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:high")
			return nil
		},
	)

	group, err := NewActivationGroup("activation", []Rule{high, second, first})
	if err != nil {
		t.Fatalf("new activation group: %v", err)
	}
	triggered, err := group.Evaluate(context.Background(), NewFacts())
	if err != nil || !triggered {
		t.Fatalf("evaluate = %v/%v, want true/nil", triggered, err)
	}
	if err := group.Execute(context.Background(), NewFacts()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fmt.Sprint(order) != "[eval:first eval:first exec:first]" {
		t.Fatalf("order = %v", order)
	}
}

func TestConditionalGroupRejectsMissingAndAmbiguousGuard(t *testing.T) {
	guard := mustRule(t, "guard", 0, true, nil)
	dependent := mustRule(t, "dependent", 1, true, nil)

	if _, err := NewConditionalGroup("conditional", "missing", []Rule{guard, dependent}); !errors.Is(err, ErrGuardRuleMissing) {
		t.Fatalf("missing guard err = %v, want ErrGuardRuleMissing", err)
	}
	if _, err := NewConditionalGroup("conditional", "guard", []Rule{guard, guard}); !errors.Is(err, ErrDuplicateRule) {
		t.Fatalf("duplicate guard err = %v, want ErrDuplicateRule", err)
	}
	if _, err := NewConditionalGroup("conditional", " ", []Rule{guard, dependent}); !errors.Is(err, ErrBlankKey) {
		t.Fatalf("blank guard err = %v, want ErrBlankKey", err)
	}
}

func TestConditionalGroupExecutesDependentsOnlyWhenGuardMatches(t *testing.T) {
	var order []string
	guard := ruleWithHooks(t, "guard", 0,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:guard")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:guard")
			return nil
		},
	)
	dependentA := ruleWithHooks(t, "a-dependent", 2,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:a")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:a")
			return nil
		},
	)
	dependentB := ruleWithHooks(t, "b-dependent", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:b")
			return false, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:b")
			return nil
		},
	)

	group, err := NewConditionalGroup("conditional", "guard", []Rule{dependentA, guard, dependentB})
	if err != nil {
		t.Fatalf("new conditional group: %v", err)
	}
	triggered, err := group.Evaluate(context.Background(), NewFacts())
	if err != nil || !triggered {
		t.Fatalf("evaluate = %v/%v, want true/nil", triggered, err)
	}
	if err := group.Execute(context.Background(), NewFacts()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fmt.Sprint(order) != "[eval:guard eval:guard eval:b eval:a exec:a]" {
		t.Fatalf("order = %v", order)
	}

	order = nil
	closedGuard := mustRule(t, "guard", 0, false, nil)
	closed, err := NewConditionalGroup("conditional", "guard", []Rule{closedGuard, dependentA})
	if err != nil {
		t.Fatalf("new closed conditional group: %v", err)
	}
	triggered, err = closed.Evaluate(context.Background(), NewFacts())
	if err != nil || triggered {
		t.Fatalf("closed evaluate = %v/%v, want false/nil", triggered, err)
	}
	if err := closed.Execute(context.Background(), NewFacts()); !errors.Is(err, ErrCompositeNotTriggered) {
		t.Fatalf("closed execute err = %v, want ErrCompositeNotTriggered", err)
	}
	if len(order) != 0 {
		t.Fatalf("dependent should not run when guard is false: %v", order)
	}
}

func TestUnitGroupRequiresAllChildrenBeforeExecution(t *testing.T) {
	var order []string
	first := ruleWithHooks(t, "first", 1,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:first")
			return true, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:first")
			return nil
		},
	)
	second := ruleWithHooks(t, "second", 2,
		func(context.Context, *Facts) (bool, error) {
			order = append(order, "eval:second")
			return false, nil
		},
		func(context.Context, *Facts) error {
			order = append(order, "exec:second")
			return nil
		},
	)

	group, err := NewUnitGroup("unit", []Rule{second, first})
	if err != nil {
		t.Fatalf("new unit group: %v", err)
	}
	triggered, err := group.Evaluate(context.Background(), NewFacts())
	if err != nil || triggered {
		t.Fatalf("evaluate = %v/%v, want false/nil", triggered, err)
	}
	if err := group.Execute(context.Background(), NewFacts()); !errors.Is(err, ErrCompositeNotTriggered) {
		t.Fatalf("execute err = %v, want ErrCompositeNotTriggered", err)
	}
	if fmt.Sprint(order) != "[eval:first eval:second eval:first eval:second]" {
		t.Fatalf("order = %v", order)
	}

	order = nil
	all, err := NewUnitGroup("unit-all", []Rule{
		mustRule(t, "second", 2, true, func(context.Context, *Facts) error {
			order = append(order, "exec:second")
			return nil
		}),
		mustRule(t, "first", 1, true, func(context.Context, *Facts) error {
			order = append(order, "exec:first")
			return nil
		}),
	})
	if err != nil {
		t.Fatalf("new all unit group: %v", err)
	}
	triggered, err = all.Evaluate(context.Background(), NewFacts())
	if err != nil || !triggered {
		t.Fatalf("all evaluate = %v/%v, want true/nil", triggered, err)
	}
	if err := all.Execute(context.Background(), NewFacts()); err != nil {
		t.Fatalf("all execute: %v", err)
	}
	if fmt.Sprint(order) != "[exec:first exec:second]" {
		t.Fatalf("all order = %v", order)
	}
}

func TestCompositeGroupValidationAndContextCancellation(t *testing.T) {
	if _, err := NewActivationGroup(" ", []Rule{mustRule(t, "rule", 0, true, nil)}); !errors.Is(err, ErrBlankKey) {
		t.Fatalf("blank group err = %v, want ErrBlankKey", err)
	}
	if _, err := NewActivationGroup("empty", nil); !errors.Is(err, ErrEmptyRuleGroup) {
		t.Fatalf("empty group err = %v, want ErrEmptyRuleGroup", err)
	}

	group, err := NewActivationGroup("activation", []Rule{mustRule(t, "rule", 0, true, nil)})
	if err != nil {
		t.Fatalf("new activation group: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := group.Evaluate(ctx, NewFacts()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel evaluate err = %v, want context.Canceled", err)
	}
	if err := group.Execute(ctx, NewFacts()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel execute err = %v, want context.Canceled", err)
	}
}

func TestCompositeGroupsFailClosedWhenEngineEvaluationDrifts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		group func(Rule) (Rule, error)
	}{
		{
			name: "activation",
			group: func(rule Rule) (Rule, error) {
				return NewActivationGroup("activation", []Rule{rule})
			},
		},
		{
			name: "conditional",
			group: func(rule Rule) (Rule, error) {
				dependent := mustRule(t, "dependent", 1, true, nil)
				return NewConditionalGroup("conditional", rule.Name(), []Rule{rule, dependent})
			},
		},
		{
			name: "unit",
			group: func(rule Rule) (Rule, error) {
				always := mustRule(t, "always", 0, true, nil)
				return NewUnitGroup("unit", []Rule{always, rule})
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			drifting := &driftingRule{name: "drift", priority: 1}
			group, err := tt.group(drifting)
			if err != nil {
				t.Fatalf("new group: %v", err)
			}
			set, err := NewRuleSet(group)
			if err != nil {
				t.Fatalf("new ruleset: %v", err)
			}
			result, err := NewEngine(set, EngineConfig{}).Run(context.Background(), NewFacts())
			if !errors.Is(err, ErrRuleExecution) || !errors.Is(err, ErrCompositeNotTriggered) {
				t.Fatalf("err = %v, want rule execution and composite drift sentinels", err)
			}
			if result.Applied != 0 || result.Failed != 1 || result.Details[0].Status != StatusExecutionFailed {
				t.Fatalf("result = %+v", result)
			}
			if drifting.executed {
				t.Fatal("drifted child should not execute")
			}
		})
	}
}

type driftingRule struct {
	name     string
	priority int
	calls    int
	executed bool
}

func (r *driftingRule) Name() string {
	return r.name
}

func (r *driftingRule) Description() string {
	return ""
}

func (r *driftingRule) Priority() int {
	return r.priority
}

func (r *driftingRule) Evaluate(context.Context, *Facts) (bool, error) {
	r.calls++
	return r.calls == 1, nil
}

func (r *driftingRule) Execute(context.Context, *Facts) error {
	r.executed = true
	return nil
}
