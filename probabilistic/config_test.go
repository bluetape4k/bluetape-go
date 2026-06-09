package probabilistic

import (
	"errors"
	"math"
	"testing"
)

func TestNewConfigValidatesInputs(t *testing.T) {
	tests := []struct {
		name               string
		expectedInsertions uint64
		fpp                float64
	}{
		{name: "zero expected insertions", expectedInsertions: 0, fpp: 0.01},
		{name: "zero fpp", expectedInsertions: 100, fpp: 0},
		{name: "one fpp", expectedInsertions: 100, fpp: 1},
		{name: "negative fpp", expectedInsertions: 100, fpp: -0.1},
		{name: "nan fpp", expectedInsertions: 100, fpp: math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfig(tt.expectedInsertions, tt.fpp)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestNewConfigRejectsUnsupportedBitSize(t *testing.T) {
	_, err := NewConfig(math.MaxUint64, math.SmallestNonzeroFloat64)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestNewConfigCalculatesBitSizeAndHashFunctionCount(t *testing.T) {
	cfg, err := NewConfig(10_000, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	if cfg.ExpectedInsertions() != 10_000 {
		t.Fatalf("expected insertions mismatch: %d", cfg.ExpectedInsertions())
	}
	if cfg.FalsePositiveProbability() != 0.01 {
		t.Fatalf("fpp mismatch: %f", cfg.FalsePositiveProbability())
	}
	if cfg.BitSize() == 0 {
		t.Fatal("expected positive bit size")
	}
	if cfg.HashFunctionCount() == 0 || cfg.HashFunctionCount() > 20 {
		t.Fatalf("unexpected hash function count: %d", cfg.HashFunctionCount())
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	filter, err := NewStringBloomFilter(Config{})
	if err != nil {
		t.Fatalf("zero Config should normalize to default: %v", err)
	}
	if filter.ExpectedInsertions() != defaultExpectedInsertions {
		t.Fatalf("expected default insertions, got %d", filter.ExpectedInsertions())
	}
}
