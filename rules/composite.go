package rules

import "context"

// CompositeOption는 func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CompositeOption func(*compositeRule)

// WithCompositeDescription는 WithCompositeDescription 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - description: WithCompositeDescription가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func WithCompositeDescription(description string) CompositeOption {
	return func(rule *compositeRule) {
		rule.description = description
	}
}

// WithCompositePriority는 WithCompositePriority 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - priority: WithCompositePriority 동작에 필요한 priority 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithCompositePriority(priority int) CompositeOption {
	return func(rule *compositeRule) {
		rule.priority = priority
	}
}

// NewActivationGroup는 NewActivationGroup 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: NewActivationGroup가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - children: NewActivationGroup가 읽거나 복사하는 children 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - options: NewActivationGroup 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewActivationGroup(name string, children []Rule, options ...CompositeOption) (Rule, error) {
	group, err := newCompositeRule(name, compositeActivation, children, "", options...)
	if err != nil {
		return nil, err
	}
	return group, nil
}

// NewConditionalGroup는 NewConditionalGroup 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: NewConditionalGroup가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - guardName: NewConditionalGroup가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - children: NewConditionalGroup가 읽거나 복사하는 children 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - options: NewConditionalGroup 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewConditionalGroup(name, guardName string, children []Rule, options ...CompositeOption) (Rule, error) {
	guardName = normalizeKey(guardName)
	if guardName == "" {
		return nil, ErrBlankKey
	}
	group, err := newCompositeRule(name, compositeConditional, children, guardName, options...)
	if err != nil {
		return nil, err
	}
	if _, ok := group.children.Get(guardName); !ok {
		return nil, ErrGuardRuleMissing
	}
	return group, nil
}

// NewUnitGroup는 NewUnitGroup 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: NewUnitGroup가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - children: NewUnitGroup가 읽거나 복사하는 children 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - options: NewUnitGroup 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUnitGroup(name string, children []Rule, options ...CompositeOption) (Rule, error) {
	group, err := newCompositeRule(name, compositeUnit, children, "", options...)
	if err != nil {
		return nil, err
	}
	return group, nil
}

type compositeKind int

const (
	compositeActivation compositeKind = iota
	compositeConditional
	compositeUnit
)

type compositeRule struct {
	name        string
	description string
	priority    int
	kind        compositeKind
	guardName   string
	children    *RuleSet
}

func newCompositeRule(name string, kind compositeKind, children []Rule, guardName string, options ...CompositeOption) (*compositeRule, error) {
	name = normalizeKey(name)
	if name == "" {
		return nil, ErrBlankKey
	}
	if len(children) == 0 {
		return nil, ErrEmptyRuleGroup
	}
	set, err := NewRuleSet(children...)
	if err != nil {
		return nil, err
	}
	rule := &compositeRule{name: name, kind: kind, guardName: guardName, children: set}
	for _, option := range options {
		if option != nil {
			option(rule)
		}
	}
	return rule, nil
}

func (r *compositeRule) Name() string {
	return r.name
}

func (r *compositeRule) Description() string {
	return r.description
}

func (r *compositeRule) Priority() int {
	return r.priority
}

func (r *compositeRule) Evaluate(ctx context.Context, facts *Facts) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	switch r.kind {
	case compositeActivation:
		_, ok, err := firstMatchingRule(ctx, r.children.entries(), facts)
		return ok, err
	case compositeConditional:
		return evaluateNamedRule(ctx, r.children, r.guardName, facts)
	case compositeUnit:
		return allRulesMatch(ctx, r.children.entries(), facts)
	default:
		return false, nil
	}
}

func (r *compositeRule) Execute(ctx context.Context, facts *Facts) error {
	if ctx == nil {
		ctx = context.Background()
	}
	switch r.kind {
	case compositeActivation:
		return executeFirstMatchingRule(ctx, r.children.entries(), facts)
	case compositeConditional:
		return executeConditionalGroup(ctx, r.children, r.guardName, facts)
	case compositeUnit:
		return executeUnitGroup(ctx, r.children.entries(), facts)
	default:
		return nil
	}
}

func executeFirstMatchingRule(ctx context.Context, entries []ruleEntry, facts *Facts) error {
	entry, ok, err := firstMatchingRule(ctx, entries, facts)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCompositeNotTriggered
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return entry.rule.Execute(ctx, facts)
}

func executeConditionalGroup(ctx context.Context, children *RuleSet, guardName string, facts *Facts) error {
	triggered, err := evaluateNamedRule(ctx, children, guardName, facts)
	if err != nil {
		return err
	}
	if !triggered {
		return ErrCompositeNotTriggered
	}
	executed := false
	for _, entry := range children.entries() {
		if entry.name == guardName {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		triggered, err := entry.rule.Evaluate(ctx, facts)
		if err != nil {
			return err
		}
		if !triggered {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := entry.rule.Execute(ctx, facts); err != nil {
			return err
		}
		executed = true
	}
	if !executed {
		return ErrCompositeNotTriggered
	}
	return nil
}

func executeUnitGroup(ctx context.Context, entries []ruleEntry, facts *Facts) error {
	matched, err := allRulesMatch(ctx, entries, facts)
	if err != nil {
		return err
	}
	if !matched {
		return ErrCompositeNotTriggered
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := entry.rule.Execute(ctx, facts); err != nil {
			return err
		}
	}
	return nil
}

func firstMatchingRule(ctx context.Context, entries []ruleEntry, facts *Facts) (ruleEntry, bool, error) {
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return ruleEntry{}, false, err
		}
		triggered, err := entry.rule.Evaluate(ctx, facts)
		if err != nil {
			return ruleEntry{}, false, err
		}
		if triggered {
			return entry, true, nil
		}
	}
	return ruleEntry{}, false, nil
}

func evaluateNamedRule(ctx context.Context, rules *RuleSet, name string, facts *Facts) (bool, error) {
	rule, ok := rules.Get(name)
	if !ok {
		return false, ErrGuardRuleMissing
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return rule.Evaluate(ctx, facts)
}

func allRulesMatch(ctx context.Context, entries []ruleEntry, facts *Facts) (bool, error) {
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		triggered, err := entry.rule.Evaluate(ctx, facts)
		if err != nil {
			return false, err
		}
		if !triggered {
			return false, nil
		}
	}
	return true, nil
}
