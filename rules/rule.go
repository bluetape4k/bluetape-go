package rules

import "context"

// Rule interface 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// EvaluateFunc func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type EvaluateFunc func(context.Context, *Facts) (bool, error)

// ExecuteFunc func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ExecuteFunc func(context.Context, *Facts) error

// RuleOption func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RuleOption func(*functionalRule)

// WithDescription WithDescription 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - description: WithDescription가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func WithDescription(description string) RuleOption {
	return func(rule *functionalRule) {
		rule.description = description
	}
}

// WithPriority WithPriority 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - priority: WithPriority 동작에 필요한 priority 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithPriority(priority int) RuleOption {
	return func(rule *functionalRule) {
		rule.priority = priority
	}
}

// NewRule NewRule 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: NewRule가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - evaluate: NewRule 동작에 필요한 evaluate 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - execute: NewRule 동작에 필요한 execute 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: NewRule 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
