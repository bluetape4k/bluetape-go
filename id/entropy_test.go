package id

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestLockedBufferedEntropyReadReturnsFullBuffer(t *testing.T) {
	source := strings.NewReader("abcdefghijklmnopqrstuvwxyz")
	entropy := &lockedBufferedEntropy{
		reader: newBufferedEntropyReader(source, 4),
	}

	first := make([]byte, 6)
	n, err := entropy.Read(first)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(first) {
		t.Fatalf("expected full read length %d, got %d", len(first), n)
	}
	if string(first) != "abcdef" {
		t.Fatalf("expected first bytes, got %q", first)
	}

	second := make([]byte, 4)
	n, err = entropy.Read(second)
	if err != nil {
		t.Fatalf("second Read failed: %v", err)
	}
	if n != len(second) || string(second) != "ghij" {
		t.Fatalf("expected second full read, got n=%d value=%q", n, second)
	}
}

func TestLockedBufferedEntropyReadReportsShortSource(t *testing.T) {
	entropy := &lockedBufferedEntropy{
		reader: newBufferedEntropyReader(bytes.NewReader([]byte{1, 2}), 4),
	}

	n, err := entropy.Read(make([]byte, 3))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected short source error, got n=%d err=%v", n, err)
	}
}

func TestLockedBufferedEntropyConcurrentStress(t *testing.T) {
	const (
		workers          = 32
		totalReads       = 16_384
		chunkSize        = 32
		sourceBufferSize = 7
	)

	source := make([]byte, totalReads*chunkSize)
	for i := range totalReads {
		offset := i * chunkSize
		binary.BigEndian.PutUint64(source[offset:], uint64(i))
		for j := 8; j < chunkSize; j++ {
			source[offset+j] = byte(i % 251)
		}
	}

	entropy := &lockedBufferedEntropy{
		reader: newBufferedEntropyReader(bytes.NewReader(source), sourceBufferSize),
	}
	seen := make([]bool, totalReads)
	var mu sync.Mutex

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       workers,
		RoundsPerTask: totalReads,
		Timeout:       10 * time.Second,
	})
	report, err := tester.Run(context.Background(), func(context.Context) error {
		buf := make([]byte, chunkSize)
		n, err := entropy.Read(buf)
		if err != nil {
			return fmt.Errorf("read entropy: %w", err)
		}
		if n != chunkSize {
			return fmt.Errorf("expected full read length %d, got %d", chunkSize, n)
		}

		id := int(binary.BigEndian.Uint64(buf))
		if id < 0 || id >= totalReads {
			return fmt.Errorf("read id out of range: %d", id)
		}
		for j := 8; j < chunkSize; j++ {
			if buf[j] != byte(id%251) {
				return fmt.Errorf("corrupt chunk id=%d at byte %d: got %d", id, j, buf[j])
			}
		}

		mu.Lock()
		defer mu.Unlock()
		if seen[id] {
			return fmt.Errorf("duplicate chunk id: %d", id)
		}
		seen[id] = true
		return nil
	})
	if err != nil {
		t.Fatalf("entropy stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != totalReads {
		t.Fatalf("expected %d completions, got %+v", totalReads, report)
	}

	for id, ok := range seen {
		if !ok {
			t.Fatalf("missing chunk id: %d", id)
		}
	}
}
