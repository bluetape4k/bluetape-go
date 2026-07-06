package rules

import "context"

// Rule evaluates facts and optionally applies changes.
type Rule interface {
	// Name returns a stable unique rule name.
	Name() string
	// Description returns human-readable rule documentation.
	Description() string
	// Priority returns the ordering priority. Lower values run first.
	Priority() int
	// Evaluate reports whether Execute should run.
	Evaluate(context.Context, *Facts) (bool, error)
	// Execute applies the rule.
	Execute(context.Context, *Facts) error
}

// EvaluateFunc evaluates a rule predicate.
type EvaluateFunc func(context.Context, *Facts) (bool, error)

// ExecuteFunc executes a triggered rule.
type ExecuteFunc func(context.Context, *Facts) error

// RuleOption configures a functional rule.
type RuleOption func(*functionalRule)

// WithDescription sets a rule description.
func WithDescription(description string) RuleOption {
	return func(rule *functionalRule) {
		rule.description = description
	}
}

// WithPriority sets a rule priority. Lower priority values run first.
func WithPriority(priority int) RuleOption {
	return func(rule *functionalRule) {
		rule.priority = priority
	}
}

// NewRule creates a Rule from ordinary Go functions.
func NewRule(name string, evaluate EvaluateFunc, execute ExecuteFunc, options ...RuleOption) (Rule, error) {
	name = normalizeKey(name)
	if name == "" {
		return nil, ErrBlankKey
	}
	if evaluate == nil {
		return nil, ErrNilEvaluate
	}
	rule := &functionalRule{
		name:     name,
		evaluate: evaluate,
		execute:  execute,
	}
	for _, option := range options {
		if option != nil {
			option(rule)
		}
	}
	return rule, nil
}

type functionalRule struct {
	name        string
	description string
	priority    int
	evaluate    EvaluateFunc
	execute     ExecuteFunc
}

func (r *functionalRule) Name() string {
	return r.name
}

func (r *functionalRule) Description() string {
	return r.description
}

func (r *functionalRule) Priority() int {
	return r.priority
}

func (r *functionalRule) Evaluate(ctx context.Context, facts *Facts) (bool, error) {
	return r.evaluate(ctx, facts)
}

func (r *functionalRule) Execute(ctx context.Context, facts *Facts) error {
	if r.execute == nil {
		return nil
	}
	return r.execute(ctx, facts)
}
