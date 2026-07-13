package sqlratelimit

import (
	"strings"
	"testing"
)

func TestSchemaSQLHasFixedContract(t *testing.T) {
	normalized := strings.ToLower(SchemaSQL)
	for _, required := range []string{
		"public.bluetape_ratelimit_buckets",
		"namespace bytea not null",
		"bucket_key bytea not null",
		"tokens_micros numeric(30, 6) not null",
		"primary key (namespace, bucket_key)",
		"bluetape_ratelimit_buckets_expires_at_idx",
		"(expires_at)",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("SchemaSQL missing %q", required)
		}
	}
	if MaxCleanupBatch != 1000 {
		t.Fatalf("MaxCleanupBatch = %d", MaxCleanupBatch)
	}
}
