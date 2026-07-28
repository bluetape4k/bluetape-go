package rules

import "context"

// InferenceConfig는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type InferenceConfig struct {
	// MaxCycles bounds inference and must be positive.
	MaxCycles int
	// EngineConfig configures each cycle's sequential engine run.
	EngineConfig EngineConfig
}

// InferenceEngine는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type InferenceEngine struct {
	rules  *RuleSet
	config InferenceConfig
}

// NewInferenceEngine는 NewInferenceEngine 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - rules: NewInferenceEngine 동작에 필요한 rules 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - config: NewInferenceEngine 동작에 필요한 config 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// InferenceResult는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type InferenceResult struct {
	Cycles       int
	CycleResults []Result
	Applied      int
	Failed       int
	Converged    bool
	Stopped      bool
	StopReason   DetailStatus
}

// Run는 Run 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - facts: Run 동작에 필요한 facts 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
