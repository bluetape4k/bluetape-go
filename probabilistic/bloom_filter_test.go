package probabilistic

import (
	"errors"
	"strconv"
	"testing"
)

func TestBloomFilterHasNoFalseNegativesForInsertedValues(t *testing.T) {
	cfg, err := NewConfig(1_000, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	filter, err := NewStringBloomFilter(cfg)
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	for i := 0; i < 1_000; i++ {
		if !filter.Put("value-" + strconv.Itoa(i)) {
			t.Fatalf("expected first put to change bits for value %d", i)
		}
	}
	for i := 0; i < 1_000; i++ {
		if !filter.MightContain("value-" + strconv.Itoa(i)) {
			t.Fatalf("inserted value %d produced false negative", i)
		}
	}
	if filter.BitCount() == 0 {
		t.Fatal("expected bit count to increase")
	}
	if filter.ApproximateElementCount() == 0 {
		t.Fatal("expected approximate count to increase")
	}
}

func TestBloomFilterPutReturnValueIsBitChangeBased(t *testing.T) {
	filter := newStringFilterForTest(t, 100, 0.01)

	if !filter.Put("alpha") {
		t.Fatal("first put should change bits")
	}
	if filter.Put("alpha") {
		t.Fatal("second put should not change bits")
	}
}

func TestBloomFilterClearResetsBits(t *testing.T) {
	filter := newStringFilterForTest(t, 100, 0.01)
	filter.Put("alpha")

	filter.Clear()

	if !filter.IsEmpty() {
		t.Fatal("expected filter to be empty")
	}
	if filter.MightContain("alpha") {
		t.Fatal("cleared value should not be contained")
	}
}

func TestBloomFilterObservedFalsePositiveRateStaysBounded(t *testing.T) {
	filter := newStringFilterForTest(t, 10_000, 0.01)

	for i := 0; i < 10_000; i++ {
		filter.Put("inserted-" + strconv.Itoa(i))
	}
	falsePositives := 0
	for i := 0; i < 20_000; i++ {
		if filter.MightContain("missing-" + strconv.Itoa(i)) {
			falsePositives++
		}
	}
	observed := float64(falsePositives) / 20_000.0
	if observed >= 0.03 {
		t.Fatalf("observed FPP too high: %.4f false positives=%d", observed, falsePositives)
	}
	if filter.ExpectedFPP() >= 0.03 {
		t.Fatalf("expected FPP too high: %.4f", filter.ExpectedFPP())
	}
}

func TestBloomFilterPutAllMergesCompatibleFilters(t *testing.T) {
	left := newStringFilterForTest(t, 1_000, 0.01)
	right := newStringFilterForTest(t, 1_000, 0.01)
	left.Put("left")
	right.Put("right")

	if err := left.PutAll(right); err != nil {
		t.Fatalf("PutAll failed: %v", err)
	}

	if !left.MightContain("left") || !left.MightContain("right") {
		t.Fatal("merged filter is missing expected values")
	}
}

func TestBloomFilterPutAllRejectsIncompatibleFilters(t *testing.T) {
	left := newStringFilterForTest(t, 1_000, 0.01)
	right := newStringFilterForTest(t, 2_000, 0.01)

	if err := left.PutAll(right); !errors.Is(err, ErrIncompatibleFilter) {
		t.Fatalf("expected ErrIncompatibleFilter, got %v", err)
	}
}

func TestBloomFilterPutAllRejectsNilFilter(t *testing.T) {
	left := newStringFilterForTest(t, 1_000, 0.01)

	if err := left.PutAll(nil); !errors.Is(err, ErrNilFilter) {
		t.Fatalf("expected ErrNilFilter, got %v", err)
	}
}

func TestBloomFilterCustomHasherCompatibilityKey(t *testing.T) {
	cfg, err := NewConfig(1_000, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	leftHasher, err := NewHasher("int-decimal", func(value int) []byte {
		return []byte(strconv.Itoa(value))
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	rightHasher, err := NewHasher("int-decimal", func(value int) []byte {
		return []byte(strconv.Itoa(value))
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	otherHasher, err := NewHasher("int-hex", func(value int) []byte {
		return []byte(strconv.FormatInt(int64(value), 16))
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}

	left, err := NewBloomFilter(cfg, leftHasher)
	if err != nil {
		t.Fatalf("NewBloomFilter failed: %v", err)
	}
	right, err := NewBloomFilter(cfg, rightHasher)
	if err != nil {
		t.Fatalf("NewBloomFilter failed: %v", err)
	}
	other, err := NewBloomFilter(cfg, otherHasher)
	if err != nil {
		t.Fatalf("NewBloomFilter failed: %v", err)
	}

	right.Put(42)
	if err := left.PutAll(right); err != nil {
		t.Fatalf("same key PutAll failed: %v", err)
	}
	if !left.MightContain(42) {
		t.Fatal("expected merged custom-hasher value")
	}
	if err := left.PutAll(other); !errors.Is(err, ErrIncompatibleFilter) {
		t.Fatalf("expected ErrIncompatibleFilter for different hasher key, got %v", err)
	}
}

func TestNewBloomFilterRejectsInvalidHasher(t *testing.T) {
	cfg, err := NewConfig(100, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if _, err := NewHasher("", func(string) []byte { return nil }); !errors.Is(err, ErrEmptyHasherKey) {
		t.Fatalf("expected ErrEmptyHasherKey, got %v", err)
	}
	if _, err := NewHasher[string]("bad", nil); !errors.Is(err, ErrNilHasher) {
		t.Fatalf("expected ErrNilHasher, got %v", err)
	}
	hasher := Hasher[string]{key: "bad"}
	if _, err := NewBloomFilter(cfg, hasher); !errors.Is(err, ErrNilHasher) {
		t.Fatalf("expected ErrNilHasher from NewBloomFilter, got %v", err)
	}
}

func TestHasherBytesExposesValidatedByteBoundary(t *testing.T) {
	hasher, err := NewHasher("custom:v1", func(value string) []byte {
		return []byte("prefix:" + value)
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}

	bytes, err := hasher.Bytes("alpha")
	if err != nil {
		t.Fatalf("Bytes failed: %v", err)
	}
	if string(bytes) != "prefix:alpha" {
		t.Fatalf("unexpected bytes: %q", string(bytes))
	}

	var zero Hasher[string]
	if _, err := zero.Bytes("alpha"); !errors.Is(err, ErrEmptyHasherKey) {
		t.Fatalf("expected ErrEmptyHasherKey, got %v", err)
	}
}

func newStringFilterForTest(t *testing.T, expectedInsertions uint64, fpp float64) BloomFilter[string] {
	t.Helper()
	cfg, err := NewConfig(expectedInsertions, fpp)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	filter, err := NewStringBloomFilter(cfg)
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	return filter
}
