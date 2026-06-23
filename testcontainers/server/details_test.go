package server_test

import (
	"errors"
	"testing"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

func TestConnectionDetailsCloneAndMergeAreImmutable(t *testing.T) {
	base := tcserver.ConnectionDetails{"redis.address": "localhost:6379"}

	clone := base.Clone()
	clone["redis.address"] = "changed"
	if got := base["redis.address"]; got != "localhost:6379" {
		t.Fatalf("base detail mutated through clone: %q", got)
	}

	merged := base.Merge(tcserver.ConnectionDetails{"nats.url": "nats://localhost:4222"})
	merged["redis.address"] = "changed-again"
	if got := base["redis.address"]; got != "localhost:6379" {
		t.Fatalf("base detail mutated through merge: %q", got)
	}
	if got := merged["nats.url"]; got != "nats://localhost:4222" {
		t.Fatalf("merged detail = %q", got)
	}
}

func TestConnectionDetailsRequire(t *testing.T) {
	details := tcserver.ConnectionDetails{"mysql.dsn": "user:pass@tcp(localhost:3306)/db"}

	got, err := details.Require("mysql.dsn")
	if err != nil {
		t.Fatalf("Require existing key: %v", err)
	}
	if got != "user:pass@tcp(localhost:3306)/db" {
		t.Fatalf("Require = %q", got)
	}

	if _, err := details.Require("missing"); !errors.Is(err, tcserver.ErrMissingDetail) {
		t.Fatalf("Require missing error = %v, want ErrMissingDetail", err)
	}
}
