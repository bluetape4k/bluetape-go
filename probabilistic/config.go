package probabilistic

import (
	"fmt"
	"math"
)

const (
	defaultExpectedInsertions            = uint64(1_000_000)
	defaultFalsePositiveProbability      = 0.03
	maxSupportedHashFunctionCount        = uint64(math.MaxInt32)
	maxSupportedMachineWordCount         = uint64(math.MaxInt)
	bitsPerWord                     uint = 64
)

var (
	ln2        = math.Log(2)
	ln2Squared = ln2 * ln2
)

// Config 는 Bloom filter 생성 설정입니다.
type Config struct {
	expectedInsertions       uint64
	falsePositiveProbability float64
	bitSize                  uint64
	hashFunctionCount        uint64
}

// DefaultConfig 는 기본 Bloom filter 설정을 반환합니다.
func DefaultConfig() Config {
	cfg, err := NewConfig(defaultExpectedInsertions, defaultFalsePositiveProbability)
	if err != nil {
		panic(err)
	}
	return cfg
}

// NewConfig 는 예상 삽입 수와 목표 false-positive probability로 설정을 만듭니다.
func NewConfig(expectedInsertions uint64, falsePositiveProbability float64) (Config, error) {
	if expectedInsertions == 0 {
		return Config{}, ConfigError{Field: "expectedInsertions", Err: fmt.Errorf("must be greater than zero")}
	}
	if falsePositiveProbability <= 0 || falsePositiveProbability >= 1 || math.IsNaN(falsePositiveProbability) {
		return Config{}, ConfigError{Field: "falsePositiveProbability", Err: fmt.Errorf("must be between 0 and 1")}
	}

	bitSize, err := optimalBitSize(expectedInsertions, falsePositiveProbability)
	if err != nil {
		return Config{}, err
	}
	hashFunctionCount, err := optimalHashFunctionCount(expectedInsertions, bitSize)
	if err != nil {
		return Config{}, err
	}

	return Config{
		expectedInsertions:       expectedInsertions,
		falsePositiveProbability: falsePositiveProbability,
		bitSize:                  bitSize,
		hashFunctionCount:        hashFunctionCount,
	}, nil
}

// ExpectedInsertions 는 설정이 가정한 예상 삽입 수를 반환합니다.
func (c Config) ExpectedInsertions() uint64 {
	return c.normalized().expectedInsertions
}

// FalsePositiveProbability 는 설정의 목표 false-positive probability를 반환합니다.
func (c Config) FalsePositiveProbability() float64 {
	return c.normalized().falsePositiveProbability
}

// BitSize 는 계산된 bitset 크기를 반환합니다.
func (c Config) BitSize() uint64 {
	return c.normalized().bitSize
}

// HashFunctionCount 는 값 하나당 계산할 hash offset 수를 반환합니다.
func (c Config) HashFunctionCount() uint64 {
	return c.normalized().hashFunctionCount
}

func (c Config) normalized() Config {
	if c.expectedInsertions == 0 && c.falsePositiveProbability == 0 && c.bitSize == 0 && c.hashFunctionCount == 0 {
		return DefaultConfig()
	}
	return c
}

func normalizeConfig(c Config) (Config, error) {
	c = c.normalized()
	if c.expectedInsertions == 0 || c.bitSize == 0 || c.hashFunctionCount == 0 {
		return Config{}, ConfigError{Err: fmt.Errorf("must be created with NewConfig or DefaultConfig")}
	}
	if c.falsePositiveProbability <= 0 || c.falsePositiveProbability >= 1 || math.IsNaN(c.falsePositiveProbability) {
		return Config{}, ConfigError{Field: "falsePositiveProbability", Err: fmt.Errorf("must be between 0 and 1")}
	}
	if err := validateWordCount(c.bitSize); err != nil {
		return Config{}, err
	}
	return c, nil
}

func optimalBitSize(expectedInsertions uint64, fpp float64) (uint64, error) {
	calculated := math.Ceil(-float64(expectedInsertions) * math.Log(fpp) / ln2Squared)
	if math.IsInf(calculated, 0) || math.IsNaN(calculated) || calculated <= 0 {
		return 0, ConfigError{Field: "bitSize", Err: fmt.Errorf("calculation overflow")}
	}
	if calculated > float64(maxSupportedMachineWordCount)*float64(bitsPerWord) {
		return 0, ConfigError{Field: "bitSize", Err: fmt.Errorf("exceeds supported maximum")}
	}
	bitSize := uint64(calculated)
	if err := validateWordCount(bitSize); err != nil {
		return 0, err
	}
	return bitSize, nil
}

func optimalHashFunctionCount(expectedInsertions uint64, bitSize uint64) (uint64, error) {
	calculated := math.Round((float64(bitSize) / float64(expectedInsertions)) * ln2)
	if math.IsInf(calculated, 0) || math.IsNaN(calculated) || calculated <= 0 {
		return 0, ConfigError{Field: "hashFunctionCount", Err: fmt.Errorf("calculation overflow")}
	}
	if calculated > float64(maxSupportedHashFunctionCount) {
		return 0, ConfigError{Field: "hashFunctionCount", Err: fmt.Errorf("exceeds supported maximum")}
	}
	return maxUint64(1, uint64(calculated)), nil
}

func validateWordCount(bitSize uint64) error {
	if bitSize == 0 {
		return ConfigError{Field: "bitSize", Err: fmt.Errorf("must be greater than zero")}
	}
	words := wordCount(bitSize)
	if words == 0 || words > maxSupportedMachineWordCount {
		return ConfigError{Field: "bitSize", Err: fmt.Errorf("exceeds supported word count")}
	}
	return nil
}

func wordCount(bitSize uint64) uint64 {
	return (bitSize + uint64(bitsPerWord) - 1) / uint64(bitsPerWord)
}

func expectedFPP(bitCount uint64, bitSize uint64, hashFunctionCount uint64) float64 {
	if bitCount == 0 || bitSize == 0 || hashFunctionCount == 0 {
		return 0
	}
	fillRatio := float64(bitCount) / float64(bitSize)
	return math.Pow(fillRatio, float64(hashFunctionCount))
}

func approximateElementCount(bitCount uint64, bitSize uint64, hashFunctionCount uint64) uint64 {
	if bitCount == 0 || bitSize == 0 || hashFunctionCount == 0 {
		return 0
	}
	if bitCount >= bitSize {
		return math.MaxUint64
	}
	fraction := 1 - float64(bitCount)/float64(bitSize)
	estimate := math.Ceil(-(float64(bitSize) / float64(hashFunctionCount)) * math.Log(fraction))
	if math.IsInf(estimate, 0) || math.IsNaN(estimate) || estimate >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(estimate)
}

func maxUint64(left uint64, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}
