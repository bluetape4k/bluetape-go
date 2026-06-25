package server_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"

	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
)

type fakeContainer struct {
	host       string
	mappedPort network.Port
	endpoint   string
	terminated bool
}

func (f *fakeContainer) Host(context.Context) (string, error) {
	return f.host, nil
}

func (f *fakeContainer) MappedPort(context.Context, string) (network.Port, error) {
	return f.mappedPort, nil
}

func (f *fakeContainer) PortEndpoint(context.Context, string, string) (string, error) {
	return f.endpoint, nil
}

func (f *fakeContainer) Terminate(context.Context, ...testcontainers.TerminateOption) error {
	f.terminated = true
	return nil
}

func TestStartedDelegatesContainerOperations(t *testing.T) {
	ctx := context.Background()
	container := &fakeContainer{
		host:       "127.0.0.1",
		mappedPort: network.MustParsePort("16379/tcp"),
		endpoint:   "redis://127.0.0.1:16379",
	}

	srv, err := tcserver.New(
		"redis",
		container,
		tcserver.WithConnectionDetails(func(context.Context, *tcserver.Started) (tcserver.ConnectionDetails, error) {
			return tcserver.ConnectionDetails{"redis.address": "127.0.0.1:16379"}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := srv.Name(); got != "redis" {
		t.Fatalf("Name = %q", got)
	}
	if got, err := srv.Host(ctx); err != nil || got != "127.0.0.1" {
		t.Fatalf("Host = %q, %v", got, err)
	}
	if got, err := srv.MappedPort(ctx, "6379/tcp"); err != nil || got != "16379" {
		t.Fatalf("MappedPort = %q, %v", got, err)
	}
	if got, err := srv.Endpoint(ctx, "6379/tcp", "redis"); err != nil || got != "redis://127.0.0.1:16379" {
		t.Fatalf("Endpoint = %q, %v", got, err)
	}

	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("ConnectionDetails: %v", err)
	}
	details["redis.address"] = "changed"

	again, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("ConnectionDetails again: %v", err)
	}
	if got := again["redis.address"]; got != "127.0.0.1:16379" {
		t.Fatalf("details leaked mutable state: %q", got)
	}

	if err := srv.Terminate(ctx); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if !container.terminated {
		t.Fatal("container was not terminated")
	}
}

func TestNewValidatesInputs(t *testing.T) {
	if _, err := tcserver.New("", &fakeContainer{}); !errors.Is(err, tcserver.ErrInvalidServer) {
		t.Fatalf("blank name error = %v, want ErrInvalidServer", err)
	}
	if _, err := tcserver.New("redis", nil); !errors.Is(err, tcserver.ErrInvalidServer) {
		t.Fatalf("nil container error = %v, want ErrInvalidServer", err)
	}
}
