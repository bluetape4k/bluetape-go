package rules

import "context"

// Rule 패키지에서 공개하는 인터페이스다.
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
type EvaluateFunc func(context.Context, *Facts) (bool, error)

// ExecuteFunc func 공개 타입이다.
type ExecuteFunc func(context.Context, *Facts) error

// RuleOption func 공개 타입이다.
type RuleOption func(*functionalRule)

// WithDescription Description 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - description: WithDescription가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func WithDescription(description string) RuleOption {
	return func(rule *functionalRule) {
		rule.description = description
	}
}

// WithPriority Priority 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - priority: WithPriority에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithPriority(priority int) RuleOption {
	return func(rule *functionalRule) {
		rule.priority = priority
	}
}

// NewRule Rule 인스턴스를 생성한다.
//
// 매개변수:
//   - name: NewRule가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - evaluate: NewRule에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - execute: NewRule에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
