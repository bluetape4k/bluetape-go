package leader_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestCandidateInfoDerivedMetrics(t *testing.T) {
	registered := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)
	completed := registered.Add(10 * time.Second)
	candidate := leader.CandidateInfo{
		NodeID:          "node-1",
		RegisteredAt:    registered,
		LastCompletedAt: completed,
		SuccessCount:    3,
		FailureCount:    1,
	}

	if candidate.TotalCount() != 4 {
		t.Fatalf("total count = %d, want 4", candidate.TotalCount())
	}
	if candidate.SuccessRate() != 0.75 {
		t.Fatalf("success rate = %f, want 0.75", candidate.SuccessRate())
	}
	if idle := candidate.IdleDuration(completed.Add(5 * time.Second)); idle != 5*time.Second {
		t.Fatalf("idle duration = %s, want 5s", idle)
	}
}

func TestFifoStrategyElectsEarliestRegisteredCandidate(t *testing.T) {
	base := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)
	candidates := []leader.CandidateInfo{
		{NodeID: "node-c", RegisteredAt: base.Add(time.Second)},
		{NodeID: "node-b", RegisteredAt: base},
		{NodeID: "node-a", RegisteredAt: base},
	}

	winner, ok := leader.FifoStrategy{}.Elect(candidates)
	if !ok {
		t.Fatal("fifo strategy should elect a winner")
	}
	if winner.NodeID != "node-a" {
		t.Fatalf("winner = %q, want node-a", winner.NodeID)
	}
	if candidates[0].NodeID != "node-c" {
		t.Fatal("strategy should not mutate input order")
	}
}

func TestRandomStrategyIsSeedStableAndInputOrderIndependent(t *testing.T) {
	base := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)
	first := []leader.CandidateInfo{
		{NodeID: "node-c", RegisteredAt: base},
		{NodeID: "node-a", RegisteredAt: base},
		{NodeID: "node-b", RegisteredAt: base},
	}
	second := []leader.CandidateInfo{
		{NodeID: "node-b", RegisteredAt: base},
		{NodeID: "node-c", RegisteredAt: base},
		{NodeID: "node-a", RegisteredAt: base},
	}
	strategy := leader.RandomStrategy{Seed: 42}

	firstWinner, ok := strategy.Elect(first)
	if !ok {
		t.Fatal("random strategy should elect a winner")
	}
	secondWinner, ok := strategy.Elect(second)
	if !ok {
		t.Fatal("random strategy should elect a winner")
	}
	if firstWinner.NodeID != secondWinner.NodeID {
		t.Fatalf("seed-stable winners differ: %q vs %q", firstWinner.NodeID, secondWinner.NodeID)
	}
}

func TestScoredStrategyElectsHighestScoreAndBreaksTiesByFifo(t *testing.T) {
	base := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)
	candidates := []leader.CandidateInfo{
		{NodeID: "node-c", RegisteredAt: base.Add(time.Second), Weight: 10},
		{NodeID: "node-a", RegisteredAt: base, Weight: 10},
		{NodeID: "node-b", RegisteredAt: base, Weight: 5},
	}

	winner, ok := leader.ScoredStrategy{Scorer: leader.WeightScorer{}}.Elect(candidates)
	if !ok {
		t.Fatal("scored strategy should elect a winner")
	}
	if winner.NodeID != "node-a" {
		t.Fatalf("winner = %q, want node-a", winner.NodeID)
	}
}

func TestScorers(t *testing.T) {
	base := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)
	now := base.Add(10 * time.Second)
	candidates := []leader.CandidateInfo{
		{NodeID: "node-a", RegisteredAt: base, SuccessCount: 3, FailureCount: 1, Weight: 7},
		{NodeID: "node-b", RegisteredAt: base.Add(5 * time.Second), Weight: 3},
	}

	idle := leader.IdleTimeScorer{Now: func() time.Time { return now }}
	if score := idle.Score(candidates[0], candidates); math.Abs(score-100) > 0.001 {
		t.Fatalf("idle score = %f, want 100", score)
	}
	if score := idle.Score(candidates[1], candidates); math.Abs(score-50) > 0.001 {
		t.Fatalf("idle score = %f, want 50", score)
	}
	if score := (leader.SuccessRateScorer{}).Score(candidates[0], candidates); math.Abs(score-75) > 0.001 {
		t.Fatalf("success score = %f, want 75", score)
	}
	if score := (leader.WeightScorer{}).Score(candidates[0], candidates); score != 7 {
		t.Fatalf("weight score = %f, want 7", score)
	}
}

func TestWeightedScorerCombinesScores(t *testing.T) {
	scorer, err := leader.NewWeightedScorer(
		leader.WeightedScore{Scorer: leader.SuccessRateScorer{}, Weight: 0.5},
		leader.WeightedScore{Scorer: leader.WeightScorer{}, Weight: 2},
	)
	if err != nil {
		t.Fatalf("new weighted scorer: %v", err)
	}

	candidate := leader.CandidateInfo{SuccessCount: 1, FailureCount: 1, Weight: 10}
	score := scorer.Score(candidate, []leader.CandidateInfo{candidate})
	if math.Abs(score-45) > 0.001 {
		t.Fatalf("weighted score = %f, want 45", score)
	}
}

func TestWeightedScorerRejectsInvalidInput(t *testing.T) {
	if _, err := leader.NewWeightedScorer(); err == nil {
		t.Fatal("empty weighted scorer should fail")
	}

	_, err := leader.NewWeightedScorer(leader.WeightedScore{
		Scorer: leader.WeightScorer{},
		Weight: 0,
	})
	if err == nil {
		t.Fatal("zero weight should fail")
	}
	if !strings.Contains(err.Error(), "weight") {
		t.Fatalf("error should mention weight, got %v", err)
	}
}
