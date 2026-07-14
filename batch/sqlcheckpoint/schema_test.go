package sqlcheckpoint

import (
	"strings"
	"testing"
)

func TestSchemaSQLFixedContract(t *testing.T) {
	const expected = `create table if not exists public.bluetape_batch_checkpoints (
    namespace bytea not null constraint bluetape_batch_checkpoints_namespace_size_check
        check (pg_catalog.octet_length(namespace) between 1 and 128),
    checkpoint_key bytea not null constraint bluetape_batch_checkpoints_key_size_check
        check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024),
    revision bigint not null constraint bluetape_batch_checkpoints_revision_check
        check (revision > 0),
    payload bytea not null constraint bluetape_batch_checkpoints_payload_size_check
        check (pg_catalog.octet_length(payload) <= 16777216),
    updated_at timestamptz not null,
    constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)
)`

	if got, want := strings.Join(strings.Fields(SchemaSQL), " "), strings.Join(strings.Fields(expected), " "); got != want {
		t.Fatalf("SchemaSQL contract mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestSchemaSQLIdentityTypesOrderAndConstraints(t *testing.T) {
	normalized := strings.ToLower(strings.Join(strings.Fields(SchemaSQL), " "))
	requiredInOrder := []string{
		"create table if not exists public.bluetape_batch_checkpoints",
		"namespace bytea not null constraint bluetape_batch_checkpoints_namespace_size_check check (pg_catalog.octet_length(namespace) between 1 and 128)",
		"checkpoint_key bytea not null constraint bluetape_batch_checkpoints_key_size_check check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024)",
		"revision bigint not null constraint bluetape_batch_checkpoints_revision_check check (revision > 0)",
		"payload bytea not null constraint bluetape_batch_checkpoints_payload_size_check check (pg_catalog.octet_length(payload) <= 16777216)",
		"updated_at timestamptz not null",
		"constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)",
	}

	position := 0
	for _, required := range requiredInOrder {
		relative := strings.Index(normalized[position:], required)
		if relative < 0 {
			t.Fatalf("SchemaSQL missing or misordered %q", required)
		}
		position += relative + len(required)
	}

	for _, forbidden := range []string{
		"create index",
		"expires_at",
		"created_at",
		"deleted_at",
		"ttl",
		"cleanup",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("SchemaSQL contains extra lifecycle/index token %q", forbidden)
		}
	}
}
