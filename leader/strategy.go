package leader

import (
	"context"
	"errors"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/bluetape4k/bluetape-go/core"
)

// CandidateInfo holds metadata for a strategy-based leader candidate.
type CandidateInfo struct {
	NodeID          string
	RegisteredAt    time.Time
	LastStartedAt   time.Time
	LastCompletedAt time.Time
	SuccessCount    int64
	FailureCount    int64
	Weight          float64
	Metadata        map[string]string
}

// TotalCount returns the number of recorded action outcomes.
func (c CandidateInfo) TotalCount() int64 {
	return c.SuccessCount + c.FailureCount
}

// SuccessRate returns the candidate success ratio from 0.0 to 1.0.
func (c CandidateInfo) SuccessRate() float64 {
	total := c.TotalCount()
	if total == 0 {
		return 0
	}
	return float64(c.SuccessCount) / float64(total)
}

// IdleDuration returns time elapsed since the last completion, or registration.
func (c CandidateInfo) IdleDuration(now time.Time) time.Duration {
	base := c.RegisteredAt
	if !c.LastCompletedAt.IsZero() {
		base = c.LastCompletedAt
	}
	if base.IsZero() || now.Before(base) {
		return 0
	}
	return now.Sub(base)
}

// CandidateResult is a recorded action outcome for a candidate.
type CandidateResult int

const (
	// CandidateSucceeded records a successful guarded action.
	CandidateSucceeded CandidateResult = iota + 1
	// CandidateFailed records a failed guarded action.
	CandidateFailed
)

// ElectionStrategy selects one candidate from a shared candidate list.
type ElectionStrategy interface {
	Elect(candidates []CandidateInfo) (CandidateInfo, bool)
}

// CandidateScorer computes a priority score for a candidate.
type CandidateScorer interface {
	Score(candidate CandidateInfo, all []CandidateInfo) float64
}

// StrategicElector coordinates strategy-based leadership.
type StrategicElector[T any] interface {
	RegisterCandidate(ctx context.Context, group string, info CandidateInfo, ttl time.Duration) error
	UnregisterCandidate(ctx context.Context, group string, nodeID string) error
	ListCandidates(ctx context.Context, group string) ([]CandidateInfo, error)
	UpdateResult(ctx context.Context, group string, nodeID string, result CandidateResult) error
	RunIfLeader(
		ctx context.Context,
		group string,
		strategy ElectionStrategy,
		action func(context.Context) (T, error),
	) (T, bool, error)
}

// FifoStrategy elects the earliest registered candidate.
type FifoStrategy struct{}

// Elect selects by RegisteredAt, then NodeID.
func (FifoStrategy) Elect(candidates []CandidateInfo) (CandidateInfo, bool) {
	sorted := sortCandidates(candidates)
	if len(sorted) == 0 {
		return CandidateInfo{}, false
	}
	return sorted[0], true
}

// RandomStrategy elects a seed-stable random candidate.
type RandomStrategy struct {
	Seed uint64
}

// Elect sorts candidates by NodeID before applying a seed-stable random choice.
func (s RandomStrategy) Elect(candidates []CandidateInfo) (CandidateInfo, bool) {
	sorted := sortCandidatesByNodeID(candidates)
	if len(sorted) == 0 {
		return CandidateInfo{}, false
	}

	rng := rand.New(rand.NewPCG(s.Seed, hashCandidateIDs(sorted)))
	return sorted[rng.IntN(len(sorted))], true
}

// ScoredStrategy elects the highest scoring candidate.
type ScoredStrategy struct {
	Scorer CandidateScorer
}

// Elect scores candidates and uses FIFO ordering to break ties.
func (s ScoredStrategy) Elect(candidates []CandidateInfo) (CandidateInfo, bool) {
	if s.Scorer == nil {
		return CandidateInfo{}, false
	}

	sorted := sortCandidates(candidates)
	if len(sorted) == 0 {
		return CandidateInfo{}, false
	}

	winner := sorted[0]
	winnerScore := s.Scorer.Score(winner, sorted)
	for _, candidate := range sorted[1:] {
		score := s.Scorer.Score(candidate, sorted)
		if score > winnerScore {
			winner = candidate
			winnerScore = score
		}
	}
	return winner, true
}

// IdleTimeScorer scores candidates by relative idle time.
type IdleTimeScorer struct {
	Now func() time.Time
}

// Score returns idle time normalized to 0-100 within the candidate list.
func (s IdleTimeScorer) Score(candidate CandidateInfo, all []CandidateInfo) float64 {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	current := now()

	maxIdle := time.Duration(0)
	for _, other := range all {
		idle := other.IdleDuration(current)
		if idle > maxIdle {
			maxIdle = idle
		}
	}
	if maxIdle <= 0 {
		return 0
	}
	return float64(candidate.IdleDuration(current)) / float64(maxIdle) * 100
}

// SuccessRateScorer scores candidates by success rate.
type SuccessRateScorer struct{}

// Score returns success rate normalized to 0-100.
func (SuccessRateScorer) Score(candidate CandidateInfo, _ []CandidateInfo) float64 {
	return candidate.SuccessRate() * 100
}

// WeightScorer scores candidates by CandidateInfo.Weight.
type WeightScorer struct{}

// Score returns the candidate weight unchanged.
func (WeightScorer) Score(candidate CandidateInfo, _ []CandidateInfo) float64 {
	return candidate.Weight
}

// WeightedScore combines one scorer with a positive weight.
type WeightedScore struct {
	Scorer CandidateScorer
	Weight float64
}

// WeightedScorer combines multiple scorers by weighted sum.
type WeightedScorer struct {
	scorers []WeightedScore
}

// NewWeightedScorer creates a weighted composite scorer.
func NewWeightedScorer(scorers ...WeightedScore) (WeightedScorer, error) {
	if len(scorers) == 0 {
		return WeightedScorer{}, errors.New("weighted scorer requires at least one scorer")
	}
	for _, scorer := range scorers {
		if scorer.Scorer == nil {
			return WeightedScorer{}, errors.New("weighted scorer contains nil scorer")
		}
		if err := core.RequirePositive("weight", scorer.Weight); err != nil {
			return WeightedScorer{}, err
		}
	}

	copied := make([]WeightedScore, len(scorers))
	copy(copied, scorers)
	return WeightedScorer{scorers: copied}, nil
}

// Score returns the weighted sum of all scorer results.
func (s WeightedScorer) Score(candidate CandidateInfo, all []CandidateInfo) float64 {
	total := 0.0
	for _, scorer := range s.scorers {
		total += scorer.Scorer.Score(candidate, all) * scorer.Weight
	}
	return total
}

func sortCandidates(candidates []CandidateInfo) []CandidateInfo {
	sorted := append([]CandidateInfo(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if left.RegisteredAt.Equal(right.RegisteredAt) {
			return left.NodeID < right.NodeID
		}
		return left.RegisteredAt.Before(right.RegisteredAt)
	})
	return sorted
}

func sortCandidatesByNodeID(candidates []CandidateInfo) []CandidateInfo {
	sorted := append([]CandidateInfo(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].NodeID < sorted[j].NodeID
	})
	return sorted
}

func hashCandidateIDs(candidates []CandidateInfo) uint64 {
	hash := fnv.New64a()
	for _, candidate := range candidates {
		_, _ = hash.Write([]byte(candidate.NodeID))
		_, _ = hash.Write([]byte{0})
	}
	return hash.Sum64()
}
