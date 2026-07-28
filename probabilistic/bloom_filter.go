package probabilistic

import (
	"math/bits"
	"sync"
)

// BloomFilter 삭제를 지원하지 않는 goroutine-safe 인메모리 Bloom filter 계약입니다.
// 구현은 패키지 내부로 제한되며 생성자는 패키지 생성 filter만 반환합니다.
type BloomFilter[T any] interface {
	ExpectedInsertions() uint64
	FalsePositiveProbability() float64
	BitSize() uint64
	HashFunctionCount() uint64
	BitCount() uint64
	IsEmpty() bool
	MightContain(value T) bool
	Put(value T) bool
	PutAll(other BloomFilter[T]) error
	ApproximateElementCount() uint64
	ExpectedFPP() float64
	Clear()
	sealedBloomFilter()
}

type filterSnapshot struct {
	config    Config
	hasherKey string
	words     []uint64
}

type bloomFilter[T any] struct {
	mu       sync.RWMutex
	config   Config
	hasher   Hasher[T]
	words    []uint64
	bitCount uint64
}

func (f *bloomFilter[T]) sealedBloomFilter() {} //nolint:unused // 외부 BloomFilter 구현을 막는 sealing hook입니다.

// NewBloomFilter 명시적 Hasher를 사용하는 BloomFilter를 만듭니다.
func NewBloomFilter[T any](cfg Config, hasher Hasher[T]) (BloomFilter[T], error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := hasher.validate(); err != nil {
		return nil, err
	}

	return &bloomFilter[T]{
		config: cfg,
		hasher: hasher,
		words:  make([]uint64, wordCount(cfg.bitSize)),
	}, nil
}

// NewStringBloomFilter string 값을 위한 BloomFilter를 만듭니다.
func NewStringBloomFilter(cfg Config) (BloomFilter[string], error) {
	return NewBloomFilter(cfg, stringHasher())
}

// NewBytesBloomFilter byte slice 값을 위한 BloomFilter를 만듭니다.
func NewBytesBloomFilter(cfg Config) (BloomFilter[[]byte], error) {
	return NewBloomFilter(cfg, bytesHasher())
}

// ExpectedInsertions 필터 생성 시 가정한 예상 삽입 수를 반환합니다.
func (f *bloomFilter[T]) ExpectedInsertions() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.expectedInsertions
}

// FalsePositiveProbability 필터 생성 시 목표로 한 false-positive probability를 반환합니다.
func (f *bloomFilter[T]) FalsePositiveProbability() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.falsePositiveProbability
}

// BitSize 내부 bitset 크기를 반환합니다.
func (f *bloomFilter[T]) BitSize() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.bitSize
}

// HashFunctionCount 값 하나당 계산하는 hash offset 수를 반환합니다.
func (f *bloomFilter[T]) HashFunctionCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.hashFunctionCount
}

// BitCount 현재 켜진 bit 개수를 반환합니다.
func (f *bloomFilter[T]) BitCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bitCount
}

// IsEmpty 필터가 비어 있는지 반환합니다.
func (f *bloomFilter[T]) IsEmpty() bool {
	return f.BitCount() == 0
}

// MightContain 은 값이 들어 있을 가능성을 검사합니다.
func (f *bloomFilter[T]) MightContain(value T) bool {
	offsets, err := f.offsets(value)
	if err != nil {
		return false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, index := range offsets {
		if !f.getBit(index) {
			return false
		}
	}
	return true
}

// Put 은 값을 Bloom filter에 추가하고 하나 이상의 bit가 새로 켜졌는지 반환합니다.
func (f *bloomFilter[T]) Put(value T) bool {
	offsets, err := f.offsets(value)
	if err != nil {
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	changed := false
	for _, index := range offsets {
		if f.setBit(index) {
			f.bitCount++
			changed = true
		}
	}
	return changed
}

// PutAll 은 호환 가능한 다른 Bloom filter의 bitset을 현재 필터로 OR 병합합니다.
func (f *bloomFilter[T]) PutAll(other BloomFilter[T]) error {
	if other == nil {
		return ErrNilFilter
	}

	source, ok := other.(*bloomFilter[T])
	if !ok {
		return ErrIncompatibleFilter
	}
	if f == source {
		return nil
	}

	snapshot := source.snapshot()

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.compatible(snapshot) {
		return ErrIncompatibleFilter
	}
	for i := range f.words {
		f.words[i] |= snapshot.words[i]
	}
	f.bitCount = countBits(f.words)
	return nil
}

// ApproximateElementCount 현재 bit 포화도를 기준으로 삽입 수를 근사합니다.
func (f *bloomFilter[T]) ApproximateElementCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return approximateElementCount(f.bitCount, f.config.bitSize, f.config.hashFunctionCount)
}

// ExpectedFPP 현재 bit 포화도를 기준으로 기대 false-positive probability를 계산합니다.
func (f *bloomFilter[T]) ExpectedFPP() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return expectedFPP(f.bitCount, f.config.bitSize, f.config.hashFunctionCount)
}

// Clear Bloom filter 상태를 초기화합니다.
func (f *bloomFilter[T]) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.words {
		f.words[i] = 0
	}
	f.bitCount = 0
}

func (f *bloomFilter[T]) offsets(value T) ([]uint64, error) {
	bytes, err := f.hasher.bytes(value)
	if err != nil {
		return nil, err
	}
	return indexes(bytes, f.config.hashFunctionCount, f.config.bitSize), nil
}

func (f *bloomFilter[T]) snapshot() filterSnapshot {
	f.mu.RLock()
	defer f.mu.RUnlock()

	words := make([]uint64, len(f.words))
	copy(words, f.words)
	return filterSnapshot{
		config:    f.config,
		hasherKey: f.hasher.Key(),
		words:     words,
	}
}

func (f *bloomFilter[T]) compatible(snapshot filterSnapshot) bool {
	return f.config.expectedInsertions == snapshot.config.expectedInsertions &&
		f.config.falsePositiveProbability == snapshot.config.falsePositiveProbability &&
		f.config.bitSize == snapshot.config.bitSize &&
		f.config.hashFunctionCount == snapshot.config.hashFunctionCount &&
		f.hasher.Key() == snapshot.hasherKey &&
		len(f.words) == len(snapshot.words)
}

func (f *bloomFilter[T]) getBit(index uint64) bool {
	wordIndex := index >> 6
	mask := uint64(1) << (index & 63)
	return f.words[wordIndex]&mask != 0
}

func (f *bloomFilter[T]) setBit(index uint64) bool {
	wordIndex := index >> 6
	mask := uint64(1) << (index & 63)
	before := f.words[wordIndex]
	f.words[wordIndex] = before | mask
	return before&mask == 0
}

func countBits(words []uint64) uint64 {
	var total uint64
	for _, word := range words {
		total += uint64(bits.OnesCount64(word))
	}
	return total
}
