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

// ErrInvalidServer Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
var ErrInvalidServer = errors.New("invalid testcontainer server")

// 이 주석은 Testcontainers fixture startup, endpoint, environment, cleanup 조건을 설명한다.
type Container interface {
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (network.Port, error)
	PortEndpoint(context.Context, string, string) (string, error)
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

// Port Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type Port struct {
	Name          string
	ContainerPort string
	Scheme        string
}

// Server Testcontainers fixture에서 제공하는 기능과 사용 경계를 설명한다.
type Server interface {
	Name() string
	Host(context.Context) (string, error)
	MappedPort(context.Context, string) (string, error)
	Endpoint(context.Context, string, string) (string, error)
	ConnectionDetails(context.Context) (ConnectionDetails, error)
	RegisterCleanup(context.Context, testing.TB)
	Terminate(context.Context) error
}

// Option Testcontainers fixture에서 설정값과 기본값 적용 방식을 설명한다.
type Option func(*Started) error

// Started Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type Started struct {
	name      string
	container Container
	ports     []Port
	details   func(context.Context, *Started) (ConnectionDetails, error)
}

var _ Server = (*Started)(nil)

// New Testcontainers fixture에서 생성과 초기화 계약을 설명한다.
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

// WithPort Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
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

// WithConnectionDetails Testcontainers fixture에서 설정값과 기본값 적용 방식을 설명한다.
func WithConnectionDetails(fn func(context.Context, *Started) (ConnectionDetails, error)) Option {
	return func(started *Started) error {
		if fn == nil {
			return fmt.Errorf("%w: connection details function must not be nil", ErrInvalidServer)
		}
		started.details = fn
		return nil
	}
}

// Name Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (s *Started) Name() string {
	return s.name
}

// Host Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (s *Started) Host(ctx context.Context) (string, error) {
	host, err := s.container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("%s host: %w", s.name, err)
	}
	return host, nil
}

// MappedPort Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (s *Started) MappedPort(ctx context.Context, containerPort string) (string, error) {
	port, err := s.container.MappedPort(ctx, containerPort)
	if err != nil {
		return "", fmt.Errorf("%s mapped port %s: %w", s.name, containerPort, err)
	}
	return port.Port(), nil
}

// Endpoint Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (s *Started) Endpoint(ctx context.Context, containerPort, scheme string) (string, error) {
	endpoint, err := s.container.PortEndpoint(ctx, containerPort, scheme)
	if err != nil {
		return "", fmt.Errorf("%s endpoint %s: %w", s.name, containerPort, err)
	}
	return endpoint, nil
}

// ConnectionDetails Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (s *Started) ConnectionDetails(ctx context.Context) (ConnectionDetails, error) {
	details, err := s.details(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("%s connection details: %w", s.name, err)
	}
	return details.Clone(), nil
}

// RegisterCleanup Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
func (s *Started) RegisterCleanup(ctx context.Context, tb testing.TB) {
	tb.Helper()
	testcleanup.Register(ctx, tb, s.name, s.container)
}

// Terminate Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func (s *Started) Terminate(ctx context.Context) error {
	if err := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, s.container); err != nil {
		return fmt.Errorf("%s terminate: %w", s.name, err)
	}
	return nil
}
