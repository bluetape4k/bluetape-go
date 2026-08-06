// Package exprreader bluetape-go의 exprreader 기능을 제공한다.
package exprreader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/bluetape4k/bluetape-go/rules"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"go.yaml.in/yaml/v3"
)

const defaultMaxNodes uint = 128

var (
	// ErrInvalidRuleDocument reports malformed YAML/JSON or schema validation.
	ErrInvalidRuleDocument = errors.New("exprreader rule document is invalid")
	// ErrInvalidRuleExpression reports expression parse, compile, or type-check failure.
	ErrInvalidRuleExpression = errors.New("exprreader rule expression is invalid")
	// ErrInvalidRuleAction reports unsupported or invalid declarative actions.
	ErrInvalidRuleAction = errors.New("exprreader rule action is invalid")
)

// ReaderError 패키지에서 공개하는 구조체다.
type ReaderError struct {
	RuleName string
	Field    string
	Err      error
}

// Error 오류 메시지를 반환한다.
func (e ReaderError) Error() string {
	var b strings.Builder
	b.WriteString("exprreader")
	if e.RuleName != "" {
		b.WriteString(" rule ")
		b.WriteString(e.RuleName)
	}
	if e.Field != "" {
		b.WriteString(" field ")
		b.WriteString(e.Field)
	}
	b.WriteString(" failed")
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (e ReaderError) Unwrap() error {
	return e.Err
}

// Document 패키지에서 공개하는 구조체다.
type Document struct {
	// Rules contains compiled rules in deterministic RuleSet order.
	Rules *rules.RuleSet
	// EngineConfig contains the optional sequential engine configuration.
	EngineConfig rules.EngineConfig
}

// Option func 공개 타입이다.
type Option func(*config)

// WithMaxNodes MaxNodes 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - maxNodes: WithMaxNodes에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithMaxNodes(maxNodes uint) Option {
	return func(c *config) {
		c.maxNodes = maxNodes
	}
}

type config struct {
	maxNodes uint
}

// Load key에 해당하는 값을 조회한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - r: Load에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Load(ctx context.Context, r io.Reader, options ...Option) (*Document, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ReaderError{Field: "reader", Err: ErrInvalidRuleDocument}
	}

	cfg := config{maxNodes: defaultMaxNodes}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.maxNodes == 0 {
		return nil, ReaderError{Field: "maxNodes", Err: ErrInvalidRuleDocument}
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, ReaderError{Field: "reader", Err: fmt.Errorf("%w: %w", ErrInvalidRuleDocument, err)}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	spec, err := decodeDocument(data)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	engineConfig, err := parseEngineConfig(spec.Engine)
	if err != nil {
		return nil, err
	}
	if err := validateDocument(spec); err != nil {
		return nil, err
	}

	compiled := make([]rules.Rule, 0, len(spec.Rules))
	for i, ruleSpec := range spec.Rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rule, err := compileRule(ruleSpec, cfg)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, rule)
		if i%8 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
	}

	set, err := rules.NewRuleSet(compiled...)
	if err != nil {
		return nil, ReaderError{Field: "rules", Err: fmt.Errorf("%w: %w", ErrInvalidRuleDocument, err)}
	}
	return &Document{Rules: set, EngineConfig: engineConfig}, nil
}

func decodeDocument(data []byte) (documentSpec, error) {
	var spec documentSpec
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return documentSpec{}, ReaderError{Field: "document", Err: fmt.Errorf("%w: %w", ErrInvalidRuleDocument, err)}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple rule documents are not supported")
		}
		return documentSpec{}, ReaderError{Field: "document", Err: fmt.Errorf("%w: %w", ErrInvalidRuleDocument, err)}
	}
	return spec, nil
}

type documentSpec struct {
	Version int            `yaml:"version"`
	Rules   []ruleSpec     `yaml:"rules"`
	Engine  map[string]any `yaml:"engine"`
}

type ruleSpec struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Priority    int                     `yaml:"priority"`
	When        string                  `yaml:"when"`
	Then        []map[string]actionSpec `yaml:"then"`
}

type actionSpec struct {
	Key   string `yaml:"key"`
	Value any    `yaml:"value"`
}

func validateDocument(spec documentSpec) error {
	if spec.Version != 1 {
		return ReaderError{Field: "version", Err: ErrInvalidRuleDocument}
	}
	if len(spec.Rules) == 0 {
		return ReaderError{Field: "rules", Err: ErrInvalidRuleDocument}
	}
	for _, rule := range spec.Rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return ReaderError{Field: "name", Err: ErrInvalidRuleDocument}
		}
		if strings.TrimSpace(rule.When) == "" {
			return ReaderError{RuleName: name, Field: "when", Err: ErrInvalidRuleDocument}
		}
		for _, action := range rule.Then {
			if err := validateAction(name, action); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAction(ruleName string, action map[string]actionSpec) error {
	if len(action) != 1 {
		return ReaderError{RuleName: ruleName, Field: "then", Err: ErrInvalidRuleAction}
	}
	set, ok := action["set"]
	if !ok {
		return ReaderError{RuleName: ruleName, Field: "then", Err: ErrInvalidRuleAction}
	}
	if strings.TrimSpace(set.Key) == "" {
		return ReaderError{RuleName: ruleName, Field: "then.set.key", Err: ErrInvalidRuleAction}
	}
	return nil
}

func parseEngineConfig(engine map[string]any) (rules.EngineConfig, error) {
	var config rules.EngineConfig
	for key, value := range engine {
		switch key {
		case "stopOnFirstApplied":
			parsed, err := boolValue(key, value)
			if err != nil {
				return rules.EngineConfig{}, err
			}
			config.StopOnFirstApplied = parsed
		case "stopOnFirstFailed":
			parsed, err := boolValue(key, value)
			if err != nil {
				return rules.EngineConfig{}, err
			}
			config.StopOnFirstFailed = parsed
		case "stopOnFirstNotTriggered":
			parsed, err := boolValue(key, value)
			if err != nil {
				return rules.EngineConfig{}, err
			}
			config.StopOnFirstNotTriggered = parsed
		case "priorityThreshold":
			parsed, err := intValue(key, value)
			if err != nil {
				return rules.EngineConfig{}, err
			}
			config.PriorityThreshold = parsed
		case "usePriorityThreshold":
			parsed, err := boolValue(key, value)
			if err != nil {
				return rules.EngineConfig{}, err
			}
			config.UsePriorityThreshold = parsed
		default:
			return rules.EngineConfig{}, ReaderError{Field: "engine." + key, Err: ErrInvalidRuleDocument}
		}
	}
	return config, nil
}

func boolValue(field string, value any) (bool, error) {
	parsed, ok := value.(bool)
	if !ok {
		return false, ReaderError{Field: "engine." + field, Err: ErrInvalidRuleDocument}
	}
	return parsed, nil
}

func intValue(field string, value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		if typed > int64(math.MaxInt) || typed < int64(math.MinInt) {
			return 0, ReaderError{Field: "engine." + field, Err: ErrInvalidRuleDocument}
		}
		return int(typed), nil
	case uint64:
		if typed > uint64(math.MaxInt) {
			return 0, ReaderError{Field: "engine." + field, Err: ErrInvalidRuleDocument}
		}
		return int(typed), nil
	default:
		return 0, ReaderError{Field: "engine." + field, Err: ErrInvalidRuleDocument}
	}
}

func compileRule(spec ruleSpec, cfg config) (rules.Rule, error) {
	name := strings.TrimSpace(spec.Name)
	callRejecter := &rejectCallsVisitor{}
	program, err := expr.Compile(
		spec.When,
		expr.AsBool(),
		expr.AllowUndefinedVariables(),
		expr.WarnOnAny(),
		expr.DisableAllBuiltins(),
		expr.Patch(callRejecter),
		expr.MaxNodes(cfg.maxNodes),
	)
	if err != nil {
		return nil, ReaderError{RuleName: name, Field: "when", Err: fmt.Errorf("%w: %w", ErrInvalidRuleExpression, err)}
	}
	if callRejecter.found {
		return nil, ReaderError{RuleName: name, Field: "when", Err: ErrInvalidRuleExpression}
	}

	actions := make([]actionSpec, 0, len(spec.Then))
	for _, action := range spec.Then {
		actions = append(actions, action["set"])
	}

	rule, err := rules.NewRule(
		name,
		func(ctx context.Context, facts *rules.Facts) (bool, error) {
			if ctx == nil {
				ctx = context.Background()
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if facts == nil {
				return false, rules.ErrNilFacts
			}
			output, err := expr.Run(program, facts.Snapshot())
			if err != nil {
				return false, err
			}
			return output.(bool), nil
		},
		func(ctx context.Context, facts *rules.Facts) error {
			if ctx == nil {
				ctx = context.Background()
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if facts == nil {
				return rules.ErrNilFacts
			}
			for _, action := range actions {
				if err := facts.Set(action.Key, action.Value); err != nil {
					return err
				}
			}
			return nil
		},
		rules.WithDescription(spec.Description),
		rules.WithPriority(spec.Priority),
	)
	if err != nil {
		return nil, ReaderError{RuleName: name, Field: "rule", Err: fmt.Errorf("%w: %w", ErrInvalidRuleDocument, err)}
	}
	return rule, nil
}

type rejectCallsVisitor struct {
	found bool
}

func (v *rejectCallsVisitor) Visit(node *ast.Node) {
	switch (*node).(type) {
	case *ast.CallNode, *ast.BuiltinNode:
		v.found = true
	}
}
