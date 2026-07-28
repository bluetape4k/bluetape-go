package rules

import "context"

// InferenceConfig 패키지에서 공개하는 구조체다.
type InferenceConfig struct {
	// MaxCycles bounds inference and must be positive.
	MaxCycles int
	// EngineConfig configures each cycle's sequential engine run.
	EngineConfig EngineConfig
}

// InferenceEngine 패키지에서 공개하는 구조체다.
type InferenceEngine struct {
	rules  *RuleSet
	config InferenceConfig
}

// NewInferenceEngine InferenceEngine 인스턴스를 생성한다.
//
// 매개변수:
//   - rules: NewInferenceEngine에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - config: NewInferenceEngine에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewInferenceEngine(rules *RuleSet, config InferenceConfig) (*InferenceEngine, error) {
	if config.MaxCycles <= 0 {
		return nil, ErrInvalidMaxCycles
	}
	if config.EngineConfig.StopOnFirstNotTriggered {
		return nil, ErrInvalidInferenceConfig
	}
	if rules == nil {
		rules = &RuleSet{byName: make(map[string]ruleEntry)}
	}
	return &InferenceEngine{rules: rules, config: config}, nil
}

// InferenceResult 패키지에서 공개하는 구조체다.
type InferenceResult struct {
	Cycles       int
	CycleResults []Result
	Applied      int
	Failed       int
	Converged    bool
	Stopped      bool
	StopReason   DetailStatus
}

// Run 작업을 실행하고 완료 또는 오류를 반환한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - facts: Run에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (e *InferenceEngine) Run(ctx context.Context, facts *Facts) (InferenceResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if facts == nil {
		return InferenceResult{}, ErrNilFacts
	}

	var result InferenceResult
	engine := NewEngine(e.rules, e.config.EngineConfig)
	for cycle := 1; cycle <= e.config.MaxCycles; cycle++ {
		if err := ctx.Err(); err != nil {
			result.Stopped = true
			result.StopReason = StatusCancelled
			return result, wrapContextError("before inference cycle", "inference", err)
		}

		cycleResult, err := engine.Run(ctx, facts)
		result.Cycles = cycle
		result.CycleResults = append(result.CycleResults, cycleResult)
		result.Applied += cycleResult.Applied
		result.Failed += cycleResult.Failed
		if cycleResult.Stopped {
			result.Stopped = true
			result.StopReason = cycleResult.StopReason
		}
		if err != nil {
			if isContextError(err) {
				result.Stopped = true
				result.StopReason = StatusCancelled
			}
			return result, err
		}
		if cycleResult.Applied == 0 {
			result.Converged = true
			return result, nil
		}
	}

	result.Stopped = true
	result.StopReason = StatusNonConverged
	return result, InferenceError{Cycles: result.Cycles, Err: ErrInferenceNonConverged}
}
