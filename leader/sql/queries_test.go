package sqlleader

import (
	"strings"
	"testing"
)

func TestSchemaSQLHasExpectedShape(t *testing.T) {
	for _, required := range []string{
		"public.bluetape_leader_leases",
		"leader_key text primary key",
		"owner_token text not null",
		"lease_until timestamptz not null",
	} {
		if !strings.Contains(strings.ToLower(SchemaSQL), required) {
			t.Fatalf("SchemaSQL missing %q", required)
		}
	}
}
