package server_test

import (
	"errors"
	"os"
	"testing"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

func TestExportEnvSetsMappedValues(t *testing.T) {
	details := tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"}
	if err := tcserver.ExportEnv(t, details, map[string]string{"redis.address": "BLUETAPE_REDIS_ADDR"}); err != nil {
		t.Fatalf("ExportEnv: %v", err)
	}
	if got := os.Getenv("BLUETAPE_REDIS_ADDR"); got != "127.0.0.1:16379" {
		t.Fatalf("BLUETAPE_REDIS_ADDR = %q", got)
	}
}

func TestExportEnvValidatesBeforeMutating(t *testing.T) {
	t.Setenv("BLUETAPE_REDIS_ADDR", "before")

	err := tcserver.ExportEnv(
		t,
		tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"},
		map[string]string{
			"redis.address": "BLUETAPE_REDIS_ADDR",
			"missing":       "BLUETAPE_MISSING",
		},
	)
	if !errors.Is(err, tcserver.ErrMissingDetail) {
		t.Fatalf("ExportEnv error = %v, want ErrMissingDetail", err)
	}
	if got := os.Getenv("BLUETAPE_REDIS_ADDR"); got != "before" {
		t.Fatalf("ExportEnv mutated before validation finished: %q", got)
	}
}

func TestExportEnvRejectsBlankEnvName(t *testing.T) {
	err := tcserver.ExportEnv(t, tcserver.ConnectionDetails{"redis.address": "x"}, map[string]string{"redis.address": ""})
	if !errors.Is(err, tcserver.ErrInvalidEnvName) {
		t.Fatalf("ExportEnv error = %v, want ErrInvalidEnvName", err)
	}
}
