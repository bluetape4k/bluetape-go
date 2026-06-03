package leader_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestGroupOptionsNormalize(t *testing.T) {
	opts, err := leader.GroupOptions{
		Options: leader.Options{
			Group:    "workers",
			MemberID: "worker-1",
		},
		MaxLeaders: 2,
	}.Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if opts.Lease <= 0 {
		t.Fatal("Lease should use default")
	}
	if opts.RenewInterval <= 0 {
		t.Fatal("RenewInterval should use default")
	}
	if opts.KeyPrefix != "bluetape:leader-group" {
		t.Fatalf("KeyPrefix should use group default, got %q", opts.KeyPrefix)
	}
	if opts.MaxLeaders != 2 {
		t.Fatalf("MaxLeaders should be preserved, got %d", opts.MaxLeaders)
	}
}

func TestGroupOptionsNormalizeRejectsInvalidMaxLeaders(t *testing.T) {
	_, err := leader.GroupOptions{
		Options: leader.Options{
			Group:    "workers",
			MemberID: "worker-1",
			Lease:    time.Second,
		},
		MaxLeaders: 0,
	}.Normalize()
	if err == nil {
		t.Fatal("Normalize should reject zero MaxLeaders")
	}
	if !strings.Contains(err.Error(), "maxLeaders") {
		t.Fatalf("error should mention maxLeaders, got %v", err)
	}
}
