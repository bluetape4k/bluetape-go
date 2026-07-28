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

// CandidateInfo strategy-based leader candidate의 metadata를 보관한다.
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

// TotalCount 기록된 action outcome 수를 반환한다.
func (c CandidateInfo) TotalCount() int64 {
	return c.SuccessCount + c.FailureCount
}

// SuccessRate candidate 성공 비율을 0.0부터 1.0 사이 값으로 반환한다.
func (c CandidateInfo) SuccessRate() float64 {
	total := c.TotalCount()
	if total == 0 {
		return 0
	}
	return float64(c.SuccessCount) / float64(total)
}

// IdleDuration은 마지막 completion 또는 registration 이후 경과 시간을 반환한다.
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

// CandidateResult candidate에 대해 기록된 action outcome이다.
type CandidateResult int

const (
	// CandidateSucceeded guarded action 성공을 기록한다.
	CandidateSucceeded CandidateResult = iota + 1
	// CandidateFailed guarded action 실패를 기록한다.
	CandidateFailed
)

// ElectionStrategy 공유 candidate list에서 candidate 하나를 선택한다.
type ElectionStrategy interface {
	Elect(candidates []CandidateInfo) (CandidateInfo, bool)
}

// CandidateScorer candidate의 priority score를 계산한다.
type CandidateScorer interface {
	Score(candidate CandidateInfo, all []CandidateInfo) float64
}

// StrategicElector strategy-based leadership을 조정한다.
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

// FifoStrategy 가장 먼저 등록된 candidate를 선출한다.
type FifoStrategy struct{}

// Elect RegisteredAt을 먼저 보고 그다음 NodeID로 candidate를 선택한다.
func (FifoStrategy) Elect(candidates []CandidateInfo) (CandidateInfo, bool) {
	sorted := sortCandidates(candidates)
	if len(sorted) == 0 {
		return CandidateInfo{}, false
	}
	return sorted[0], true
}

// RandomStrategy seed-stable random candidate를 선출한다.
type RandomStrategy struct {
	Seed uint64
}

// Elect seed-stable random choice를 적용하기 전에 candidate를 NodeID로 정렬한다.
func (s RandomStrategy) Elect(candidates []CandidateInfo) (CandidateInfo, bool) {
	sorted := sortCandidatesByNodeID(candidates)
	if len(sorted) == 0 {
		return CandidateInfo{}, false
	}

	rng := rand.New(rand.NewPCG(s.Seed, hashCandidateIDs(sorted)))
	return sorted[rng.IntN(len(sorted))], true
}

// ScoredStrategy score가 가장 높은 candidate를 선출한다.
type ScoredStrategy struct {
	Scorer CandidateScorer
}

// Elect candidate score를 계산하고 동점이면 FIFO ordering으로 결정한다.
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

// IdleTimeScorer 상대 idle time으로 candidate score를 계산한다.
type IdleTimeScorer struct {
	Now func() time.Time
}

// Score candidate list 안에서 idle time을 0-100 범위로 정규화해 반환한다.
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

// SuccessRateScorer success rate로 candidate score를 계산한다.
type SuccessRateScorer struct{}

// Score success rate를 0-100 범위로 정규화해 반환한다.
func (SuccessRateScorer) Score(candidate CandidateInfo, _ []CandidateInfo) float64 {
	return candidate.SuccessRate() * 100
}

// WeightScorer CandidateInfo.Weight로 candidate score를 계산한다.
type WeightScorer struct{}

// Score candidate weight를 그대로 반환한다.
func (WeightScorer) Score(candidate CandidateInfo, _ []CandidateInfo) float64 {
	return candidate.Weight
}

// WeightedScore 하나의 scorer와 양수 weight를 결합한다.
type WeightedScore struct {
	Scorer CandidateScorer
	Weight float64
}

// WeightedScorer 여러 scorer를 weighted sum으로 결합한다.
type WeightedScorer struct {
	scorers []WeightedScore
}

// NewWeightedScorer weighted composite scorer를 생성한다.
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

// Score 모든 scorer 결과의 weighted sum을 반환한다.
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
