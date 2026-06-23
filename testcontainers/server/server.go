package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
)

// ErrInvalidServer reports invalid server adapter configuration.
var ErrInvalidServer = errors.New("invalid testcontainer server")

// Container is the Testcontainers subset needed by Started.
type Container interface {
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (network.Port, error)
	PortEndpoint(context.Context, string, string) (string, error)
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

// Port describes a named container port exposed by a server.
type Port struct {
	Name          string
	ContainerPort string
	Scheme        string
}

// Server exposes common operations for a started Testcontainers fixture.
type Server interface {
	Name() string
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (string, error)
	Endpoint(context.Context, string, string) (string, error)
	ConnectionDetails(context.Context) (ConnectionDetails, error)
	RegisterCleanup(context.Context, testing.TB)
	Terminate(context.Context) error
}

// Option configures a Started server adapter.
type Option func(*Started) error

// Started adapts a started Testcontainers container to the Server contract.
type Started struct {
	name      string
	container Container
	ports     []Port
	details   func(context.Context, *Started) (ConnectionDetails, error)
}

var _ Server = (*Started)(nil)

// New creates a server adapter around an already-started container.
func New(name string, container Container, options ...Option) (*Started, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name must not be blank", ErrInvalidServer)
	}
	if container == nil {
		return nil, fmt.Errorf("%w: container must not be nil", ErrInvalidServer)
	}

	started := &Started{
		name:      name,
		container: container,
		details: func(context.Context, *Started) (ConnectionDetails, error) {
			return ConnectionDetails{}, nil
		},
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: option must not be nil", ErrInvalidServer)
		}
		if err := option(started); err != nil {
			return nil, err
		}
	}
	return started, nil
}

// WithPort records named port metadata for a server.
func WithPort(name, containerPort, scheme string) Option {
	return func(started *Started) error {
		port := Port{
			Name:          strings.TrimSpace(name),
			ContainerPort: strings.TrimSpace(containerPort),
			Scheme:        strings.TrimSpace(scheme),
		}
		if port.Name == "" {
			return fmt.Errorf("%w: port name must not be blank", ErrInvalidServer)
		}
		if port.ContainerPort == "" {
			return fmt.Errorf("%w: container port must not be blank", ErrInvalidServer)
		}
		started.ports = append(started.ports, port)
		return nil
	}
}

// WithConnectionDetails sets the function used to compute connection details.
func WithConnectionDetails(fn func(context.Context, *Started) (ConnectionDetails, error)) Option {
	return func(started *Started) error {
		if fn == nil {
			return fmt.Errorf("%w: connection details function must not be nil", ErrInvalidServer)
		}
		started.details = fn
		return nil
	}
}

// Name returns the server name used in diagnostics.
func (s *Started) Name() string {
	return s.name
}

// Host returns the host where container ports are exposed.
func (s *Started) Host(ctx context.Context) (string, error) {
	host, err := s.container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("%s host: %w", s.name, err)
	}
	return host, nil
}

// MappedPort returns the mapped host port for a container port.
func (s *Started) MappedPort(ctx context.Context, containerPort string) (string, error) {
	port, err := s.container.MappedPort(ctx, containerPort)
	if err != nil {
		return "", fmt.Errorf("%s mapped port %s: %w", s.name, containerPort, err)
	}
	return port.Port(), nil
}

// Endpoint returns a scheme-qualified endpoint for a container port.
func (s *Started) Endpoint(ctx context.Context, containerPort, scheme string) (string, error) {
	endpoint, err := s.container.PortEndpoint(ctx, containerPort, scheme)
	if err != nil {
		return "", fmt.Errorf("%s endpoint %s: %w", s.name, containerPort, err)
	}
	return endpoint, nil
}

// ConnectionDetails returns cloned connection details for this server.
func (s *Started) ConnectionDetails(ctx context.Context) (ConnectionDetails, error) {
	details, err := s.details(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("%s connection details: %w", s.name, err)
	}
	return details.Clone(), nil
}

// RegisterCleanup registers bounded container cleanup on the test.
func (s *Started) RegisterCleanup(ctx context.Context, tb testing.TB) {
	tb.Helper()
	testcleanup.Register(ctx, tb, s.name, s.container)
}

// Terminate terminates the server container with the default bounded timeout.
func (s *Started) Terminate(ctx context.Context) error {
	if err := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, s.container); err != nil {
		return fmt.Errorf("%s terminate: %w", s.name, err)
	}
	return nil
}
