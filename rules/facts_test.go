package rules

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestFactsOperationsAndOrdering(t *testing.T) {
	facts := NewFacts()
	if err := facts.Set("region", "ap-northeast-2"); err != nil {
		t.Fatalf("set region: %v", err)
	}
	if err := facts.Set(" amount ", 120); err != nil {
		t.Fatalf("set amount: %v", err)
	}

	if err := facts.Set(" ", 1); !errors.Is(err, ErrBlankKey) {
		t.Fatalf("blank key err = %v, want ErrBlankKey", err)
	}
	if got := facts.Keys(); fmt.Sprint(got) != "[amount region]" {
		t.Fatalf("keys = %v, want sorted amount/region", got)
	}
	if value, ok := facts.Get("amount"); !ok || value != 120 {
		t.Fatalf("amount = %v/%v, want 120/true", value, ok)
	}
	if !facts.Delete("region") || facts.Has("region") {
		t.Fatal("delete should remove existing region")
	}
	if facts.Len() != 1 {
		t.Fatalf("len = %d, want 1", facts.Len())
	}
}

func TestFactsSnapshotAndCloneAreShallowContainerCopies(t *testing.T) {
	nested := map[string]int{"count": 1}
	facts, err := NewFactsFrom(map[string]any{
		"nested": nested,
		"name":   "catalog",
	})
	if err != nil {
		t.Fatalf("new facts: %v", err)
	}

	snapshot := facts.Snapshot()
	snapshot["name"] = "changed"
	if value, _ := facts.Get("name"); value != "catalog" {
		t.Fatalf("snapshot should not rewrite container, got %v", value)
	}

	clone := facts.Clone()
	if err := clone.Set("name", "clone"); err != nil {
		t.Fatalf("clone set: %v", err)
	}
	if value, _ := facts.Get("name"); value != "catalog" {
		t.Fatalf("clone container mutation affected source: %v", value)
	}

	nested["count"] = 2
	cloneNested, _ := clone.Get("nested")
	if cloneNested.(map[string]int)["count"] != 2 {
		t.Fatal("stored values should remain caller-owned shallow copies")
	}
}

func TestFactsConcurrentAccessStress(t *testing.T) {
	facts := NewFacts()
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 100,
		Timeout:       2 * time.Second,
	})

	var next atomic.Int64
	tester.RunT(t,
		func(context.Context) error {
			key := fmt.Sprintf("k-%03d", next.Add(1))
			return facts.Set(key, key)
		},
		func(context.Context) error {
			_ = facts.Keys()
			_ = facts.Snapshot()
			return nil
		},
		func(context.Context) error {
			facts.Delete("missing")
			return nil
		},
	)
}
