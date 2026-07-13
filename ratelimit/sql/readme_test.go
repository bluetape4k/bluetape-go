package sqlratelimit

import (
	"os"
	"strings"
	"testing"
)

func TestReadmeContract(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"README.md", "README.ko.md"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := readContractFile(t, name)
			assertContractMarkers(t, body, []string{
				"[English](README.md) | [한국어](README.ko.md)",
				"SchemaSQL",
				"New",
				"Allow",
				"Cleanup",
				"ErrConfigurationMismatch",
				"ErrCommitUnknown",
				"caller-owned",
				"moderate-QPS",
				"not a Redis replacement",
				"public.bluetape_ratelimit_buckets",
				"least-privilege",
				"primary-only",
				"namespace rotation",
				"no automatic replay",
				"count zero",
				"up to `limit` rows",
				"not an idempotent batch replay",
				"quota state is not shared",
				"multiple full bursts",
				"independent namespace",
				"independent cohort",
				"quiesce",
				"full-refill window",
				"approved extra-burst budget",
			})
		})
	}

	for _, name := range []string{"../README.md", "../README.ko.md"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := readContractFile(t, name)
			assertContractMarkers(t, body, []string{
				"ratelimit/redis",
				"ratelimit/sql",
				"OperationError",
				"ErrCommitUnknown",
				"quota state is not shared",
				"multiple full bursts",
				"independent namespace",
				"independent cohort",
				"quiesce",
				"full-refill window",
				"approved extra-burst budget",
			})
		})
	}
}

func readContractFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

func assertContractMarkers(t *testing.T, body string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Errorf("missing contract marker %q", marker)
		}
	}
}
