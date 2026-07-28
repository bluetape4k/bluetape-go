package probabilistic

import (
	"math/bits"
	"sync"
)

// BloomFilter interface 공개 타입이며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// NewBloomFilter NewBloomFilter 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - cfg: NewBloomFilter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - hasher: hash index를 계산하는 deterministic hasher다. compatibility와 seed 의미는 hasher 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// NewStringBloomFilter NewStringBloomFilter 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - cfg: NewStringBloomFilter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewStringBloomFilter(cfg Config) (BloomFilter[string], error) {
	return NewBloomFilter(cfg, stringHasher())
}

// NewBytesBloomFilter NewBytesBloomFilter 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - cfg: NewBytesBloomFilter에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewBytesBloomFilter(cfg Config) (BloomFilter[[]byte], error) {
	return NewBloomFilter(cfg, bytesHasher())
}

// ExpectedInsertions ExpectedInsertions 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) ExpectedInsertions() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.expectedInsertions
}

// FalsePositiveProbability FalsePositiveProbability 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) FalsePositiveProbability() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.falsePositiveProbability
}

// BitSize BitSize 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) BitSize() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.bitSize
}

// HashFunctionCount HashFunctionCount 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) HashFunctionCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.hashFunctionCount
}

// BitCount BitCount 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) BitCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bitCount
}

// IsEmpty IsEmpty 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) IsEmpty() bool {
	return f.BitCount() == 0
}

// MightContain MightContain 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - value: Bloom/Redis filter에 추가하거나 검사할 값이다. nil/empty/hash input 의미는 hasher 계약을 따른다.
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

// Put Put 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - value: Bloom/Redis filter에 추가하거나 검사할 값이다. nil/empty/hash input 의미는 hasher 계약을 따른다.
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

// PutAll PutAll 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - other: PutAll에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// ApproximateElementCount ApproximateElementCount 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) ApproximateElementCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return approximateElementCount(f.bitCount, f.config.bitSize, f.config.hashFunctionCount)
}

// ExpectedFPP ExpectedFPP 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (f *bloomFilter[T]) ExpectedFPP() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return expectedFPP(f.bitCount, f.config.bitSize, f.config.hashFunctionCount)
}

// Clear Clear 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
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
