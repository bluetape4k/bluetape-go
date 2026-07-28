package rules

import "context"

// CompositeOption func 공개 타입이다.
type CompositeOption func(*compositeRule)

// WithCompositeDescription CompositeDescription 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - description: WithCompositeDescription가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func WithCompositeDescription(description string) CompositeOption {
	return func(rule *compositeRule) {
		rule.description = description
	}
}

// WithCompositePriority CompositePriority 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - priority: WithCompositePriority에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithCompositePriority(priority int) CompositeOption {
	return func(rule *compositeRule) {
		rule.priority = priority
	}
}

// NewActivationGroup ActivationGroup 인스턴스를 생성한다.
//
// 매개변수:
//   - name: NewActivationGroup가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - children: NewActivationGroup가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewActivationGroup(name string, children []Rule, options ...CompositeOption) (Rule, error) {
	group, err := newCompositeRule(name, compositeActivation, children, "", options...)
	if err != nil {
		return nil, err
	}
	return group, nil
}

// NewConditionalGroup ConditionalGroup 인스턴스를 생성한다.
//
// 매개변수:
//   - name: NewConditionalGroup가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - guardName: NewConditionalGroup가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - children: NewConditionalGroup가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// NewUnitGroup UnitGroup 인스턴스를 생성한다.
//
// 매개변수:
//   - name: NewUnitGroup가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - children: NewUnitGroup가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
