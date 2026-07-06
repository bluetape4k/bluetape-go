package rules

import "context"

// InferenceConfig configures bounded sequential inference.
type InferenceConfig struct {
	// MaxCycles bounds inference and must be positive.
	MaxCycles int
	// EngineConfig configures each cycle's sequential engine run.
	EngineConfig EngineConfig
}

// InferenceEngine runs rules repeatedly until convergence or a cycle limit.
type InferenceEngine struct {
	rules  *RuleSet
	config InferenceConfig
}

// NewInferenceEngine creates a bounded inference engine.
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

// InferenceResult reports a bounded inference run.
type InferenceResult struct {
	Cycles       int
	CycleResults []Result
	Applied      int
	Failed       int
	Converged    bool
	Stopped      bool
	StopReason   DetailStatus
}

// Run executes inference until no rules apply, context cancellation occurs, a
// rule fails, or MaxCycles is exceeded.
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
