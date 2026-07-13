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
				"caller-owned deadline",
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

func TestReleaseRunbookContract(t *testing.T) {
	t.Parallel()

	body := readContractFile(t, "../../docs/release/v0.19.0-provider-conformance-runbook.md")
	start := strings.Index(body, "### SQL Rate Limiter Deployment Gates")
	if start < 0 {
		t.Fatal("missing SQL rate limiter deployment section")
	}
	body = body[start:]
	ordered := []string{
		"verify public schema ownership",
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"effective CREATE",
		"SET LOCAL lock_timeout",
		"SET LOCAL statement_timeout",
		"SchemaSQL",
		"Catalog preflight",
		"Runtime grants",
	}
	last := -1
	for _, marker := range ordered {
		index := strings.Index(body, marker)
		if index < 0 {
			t.Errorf("missing ordered runbook marker %q", marker)
			continue
		}
		if index <= last {
			t.Errorf("runbook marker %q appears out of order", marker)
		}
		last = index
	}

	assertContractMarkers(t, body, []string{
		"pg_is_in_recovery() = false",
		"transaction_read_only = off",
		"server identity",
		"HA timeline",
		"cadence shorter than `IdleTTL`",
		"limit in `1..1000`",
		"maximum batches",
		"elapsed budget",
		"Pause cleanup",
		"count is zero",
		"up to `limit` rows",
		"stable baseline",
		"consecutive breach",
		"minimum canary observation window",
		"independent canary namespace",
		"independent canary cohort",
		"old-writer fencing",
		"durability/RPO",
		"no statement replay",
		"retain the SQL relation, expiry index, and grants",
		"zero SQL-provider binary deployment",
		"zero SQL-provider traffic",
		"separate destructive migration",
	})
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
