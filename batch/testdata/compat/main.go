package compat

import "github.com/bluetape4k/bluetape-go/batch"

var _ = batch.StepOptions[int, int]{
	"legacy-unkeyed", 1, nil, nil, nil,
	batch.RetryPolicy{}, batch.SkipPolicy{}, nil, "",
}
