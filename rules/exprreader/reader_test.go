package exprreader_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/rules"
	"github.com/bluetape4k/bluetape-go/rules/exprreader"
)

func TestLoadCompilesYAMLRulesIntoRuleSet(t *testing.T) {
	const document = `
version: 1
rules:
  - name: high-value-discount
    description: Apply a discount for high-value orders.
    priority: 10
    when: amount >= 100 && tier in ["gold", "platinum"]
    then:
      - set:
          key: discount
          value: 10
  - name: fraud-review
    priority: 20
    when: risk_score >= 80
    then:
      - set:
          key: review_required
          value: true
engine:
  stopOnFirstFailed: true
`

	loaded, err := exprreader.Load(context.Background(), strings.NewReader(document))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.EngineConfig.StopOnFirstFailed != true {
		t.Fatalf("StopOnFirstFailed = false, want true")
	}

	facts, err := rules.NewFactsFrom(map[string]any{
		"amount":     125,
		"tier":       "gold",
		"risk_score": 40,
	})
	if err != nil {
		t.Fatalf("NewFactsFrom() error = %v", err)
	}

	result, err := rules.NewEngine(loaded.Rules, loaded.EngineConfig).Run(context.Background(), facts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Applied != 1 || result.NotTriggered != 1 {
		t.Fatalf("result applied/notTriggered = %d/%d, want 1/1", result.Applied, result.NotTriggered)
	}
	if got, ok := facts.Get("discount"); !ok || got != 10 {
		t.Fatalf("discount = %v, %v; want 10, true", got, ok)
	}
	if facts.Has("review_required") {
		t.Fatalf("review_required was set for a non-triggered rule")
	}
}

func TestLoadCompilesJSONAndPreservesRuleSetOrdering(t *testing.T) {
	const document = `{
  "version": 1,
  "rules": [
    {"name": "second", "priority": 20, "when": "true", "then": [{"set": {"key": "last", "value": "second"}}]},
    {"name": "first", "priority": 10, "when": "true", "then": [{"set": {"key": "first", "value": true}}]}
  ]
}`

	loaded, err := exprreader.Load(context.Background(), strings.NewReader(document))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	result, err := rules.NewEngine(loaded.Rules, loaded.EngineConfig).Run(context.Background(), rules.NewFacts())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Details) != 2 {
		t.Fatalf("detail count = %d, want 2", len(result.Details))
	}
	if result.Details[0].RuleName != "first" || result.Details[1].RuleName != "second" {
		t.Fatalf("detail order = %q, %q; want first, second", result.Details[0].RuleName, result.Details[1].RuleName)
	}
}

func TestLoadRejectsInvalidDocumentsWithTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want error
	}{
		{
			name: "missing name",
			doc: `version: 1
rules:
  - when: "true"
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
		{
			name: "duplicate name",
			doc: `version: 1
rules:
  - name: same
    when: "true"
  - name: same
    when: "true"
`,
			want: rules.ErrDuplicateRule,
		},
		{
			name: "invalid action",
			doc: `version: 1
rules:
  - name: unsupported-action
    when: "true"
    then:
      - delete:
          key: value
`,
			want: exprreader.ErrInvalidRuleAction,
		},
		{
			name: "invalid engine config",
			doc: `version: 1
rules:
  - name: ok
    when: "true"
engine:
  unknown: true
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
		{
			name: "unknown top-level field",
			doc: `version: 1
rules:
  - name: ok
    when: "true"
inference:
  enabled: true
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
		{
			name: "unknown rule field",
			doc: `version: 1
rules:
  - name: ok
    prioirty: 1
    when: "true"
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
		{
			name: "unknown set payload field",
			doc: `version: 1
rules:
  - name: ok
    when: "true"
    then:
      - set:
          key: applied
          value: true
          append: true
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
		{
			name: "engine integer overflow",
			doc: `version: 1
rules:
  - name: ok
    when: "true"
engine:
  priorityThreshold: 9223372036854775808
`,
			want: exprreader.ErrInvalidRuleDocument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exprreader.Load(context.Background(), strings.NewReader(tt.doc))
			if !errors.Is(err, tt.want) {
				t.Fatalf("Load() error = %v, want errors.Is(..., %v)", err, tt.want)
			}

			var readerErr exprreader.ReaderError
			if !errors.As(err, &readerErr) {
				t.Fatalf("Load() error = %T, want ReaderError", err)
			}
		})
	}
}

func TestLoadRequiresBooleanExpressions(t *testing.T) {
	const document = `
version: 1
rules:
  - name: not-bool
    when: amount + 1
`

	_, err := exprreader.Load(context.Background(), strings.NewReader(document))
	if !errors.Is(err, exprreader.ErrInvalidRuleExpression) {
		t.Fatalf("Load() error = %v, want ErrInvalidRuleExpression", err)
	}
}

func TestLoadRejectsFunctionCallsInPredicates(t *testing.T) {
	tests := []struct {
		name string
		when string
	}{
		{name: "function call", when: "allowed() == true"},
		{name: "builtin call", when: "len(items) > 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := `
version: 1
rules:
  - name: no-callbacks
    when: ` + strconv.Quote(tt.when) + `
`

			_, err := exprreader.Load(context.Background(), strings.NewReader(document))
			if !errors.Is(err, exprreader.ErrInvalidRuleExpression) {
				t.Fatalf("Load() error = %v, want ErrInvalidRuleExpression", err)
			}
		})
	}
}

func TestLoadRejectsExpressionsAboveMaxNodes(t *testing.T) {
	const document = `
version: 1
rules:
  - name: too-large
    when: amount > 10 && tier == "gold"
`

	_, err := exprreader.Load(context.Background(), strings.NewReader(document), exprreader.WithMaxNodes(2))
	if !errors.Is(err, exprreader.ErrInvalidRuleExpression) {
		t.Fatalf("Load() error = %v, want ErrInvalidRuleExpression", err)
	}
}

func TestGeneratedRuleReturnsRuntimePredicateAndActionErrors(t *testing.T) {
	const document = `
version: 1
rules:
  - name: runtime
    when: missing.value == true
    then:
      - set:
          key: applied
          value: true
`

	loaded, err := exprreader.Load(context.Background(), strings.NewReader(document))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	rule, ok := loaded.Rules.Get("runtime")
	if !ok {
		t.Fatalf("runtime rule not found")
	}

	engine := rules.NewEngine(loaded.Rules, rules.EngineConfig{StopOnFirstFailed: true})
	result, err := engine.Run(context.Background(), rules.NewFacts())
	if !errors.Is(err, rules.ErrRuleEvaluation) {
		t.Fatalf("Run() error = %v, want ErrRuleEvaluation", err)
	}
	if result.Failed != 1 || len(result.Details) != 1 {
		t.Fatalf("result failed/details = %d/%d, want 1/1", result.Failed, len(result.Details))
	}
	if !errors.Is(result.Details[0].Err, rules.ErrRuleEvaluation) {
		t.Fatalf("detail error = %v, want ErrRuleEvaluation", result.Details[0].Err)
	}
	if err := rule.Execute(context.Background(), nil); !errors.Is(err, rules.ErrNilFacts) {
		t.Fatalf("Execute(nil) error = %v, want ErrNilFacts", err)
	}
}

func TestLoadAndEvaluateObserveContextCancellation(t *testing.T) {
	const document = `
version: 1
rules:
  - name: cancellable
    when: "true"
`

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := exprreader.Load(ctx, strings.NewReader(document)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load(cancelled) error = %v, want context.Canceled", err)
	}

	loaded, err := exprreader.Load(context.Background(), strings.NewReader(document))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	rule, ok := loaded.Rules.Get("cancellable")
	if !ok {
		t.Fatalf("cancellable rule not found")
	}
	if _, err := rule.Evaluate(ctx, rules.NewFacts()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate(cancelled) error = %v, want context.Canceled", err)
	}
}
