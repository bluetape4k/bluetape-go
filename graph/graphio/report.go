package graphio

import "time"

// Failure graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Failure struct {
	Phase    Phase
	Severity Severity
	Location Location
	Field    string
	RecordID string
	Summary  string
}

// Report graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
type Report struct {
	Format          Format
	VerticesRead    int64
	EdgesRead       int64
	VerticesWritten int64
	EdgesWritten    int64
	SkippedVertices int64
	SkippedEdges    int64
	Failures        []Failure
	OmittedFailures int64
	Elapsed         time.Duration
}

// AddFailure graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (r *Report) AddFailure(failure Failure, maxFailures int) {
	if maxFailures == 0 {
		maxFailures = defaultMaxFailures
	}
	failure.RecordID = redactID(failure.RecordID)
	if maxFailures < 0 || len(r.Failures) < maxFailures {
		r.Failures = append(r.Failures, failure)
		return
	}
	r.OmittedFailures++
}
