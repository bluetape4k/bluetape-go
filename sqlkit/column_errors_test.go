package sqlkit

import (
	"errors"
	"strings"
	"testing"
)

func TestBoundedCopiedColumnSourceRejectsBeforeCopy(t *testing.T) {
	sources := map[string]any{
		"bytes":  make([]byte, 1<<20),
		"string": strings.Repeat("x", 1<<20),
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			result := testing.Benchmark(func(b *testing.B) {
				for range b.N {
					raw, _, err := boundedCopiedColumnSource(source, 8, "test source")
					if raw != nil || !errors.Is(err, ErrColumnValueTooLarge) {
						b.Fatalf("bounded copy = %d bytes, %v", len(raw), err)
					}
				}
			})

			if allocated := result.AllocedBytesPerOp(); allocated >= 64<<10 {
				t.Fatalf("oversized source allocated %d bytes/op; want early rejection", allocated)
			}
		})
	}
}
