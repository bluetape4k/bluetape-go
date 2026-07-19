package leadertest

import (
	"testing"
	"time"
)

func TestNormalizeTimingDefaultsAndPartialOverride(t *testing.T) {
	defaults, err := normalizeConfig(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Timing.Lease != 300*time.Millisecond || defaults.Timing.RenewInterval != 50*time.Millisecond ||
		defaults.Timing.CaseTimeout != 5*time.Second || defaults.Timing.WaitTimeout != 2*time.Second ||
		defaults.Timing.ResignTimeout != 250*time.Millisecond {
		t.Fatalf("default timing = %+v", defaults.Timing)
	}

	got, err := normalizeConfig(Config{Timing: Timing{Lease: 3 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Timing.Lease != 3*time.Second || got.Timing.RenewInterval != 50*time.Millisecond ||
		got.Timing.CaseTimeout != 5*time.Second || got.Timing.WaitTimeout != 2*time.Second ||
		got.Timing.ResignTimeout != 250*time.Millisecond {
		t.Fatalf("normalized timing = %+v", got.Timing)
	}
}

func TestNormalizeTimingRejectsContainmentViolations(t *testing.T) {
	invalid := []Timing{
		{Lease: time.Second, RenewInterval: time.Second},
		{CaseTimeout: -time.Second},
		{CaseTimeout: 3 * time.Second, WaitTimeout: 2 * time.Second, ResignTimeout: time.Second},
		{CaseTimeout: time.Duration(1<<63 - 1), WaitTimeout: time.Duration(1<<63 - 2), ResignTimeout: time.Second},
	}
	for _, timing := range invalid {
		if _, err := normalizeConfig(Config{Timing: timing}); err == nil {
			t.Fatalf("accepted invalid timing %+v", timing)
		}
	}
}
