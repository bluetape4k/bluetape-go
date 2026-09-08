package graphtest

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

const (
	// DefaultStartupTimeout 값은 factory와 readiness 기본 상한이다.
	DefaultStartupTimeout = 90 * time.Second
	// DefaultCaseTimeout 값은 각 core/optional callback 기본 상한이다.
	DefaultCaseTimeout = 10 * time.Second
	// DefaultCleanupTimeout 값은 fixture cleanup 기본 상한이다.
	DefaultCleanupTimeout = 30 * time.Second
	// DefaultCloseTimeout 값은 adapter close 기본 상한이다.
	DefaultCloseTimeout = 10 * time.Second
	// MaxStartupTimeout 값은 caller가 지정할 수 있는 startup 최대값이다.
	MaxStartupTimeout = 5 * time.Minute
	// MaxCaseTimeout 값은 caller가 지정할 수 있는 case 최대값이다.
	MaxCaseTimeout = time.Minute
	// MaxCleanupTimeout 값은 caller가 지정할 수 있는 cleanup 최대값이다.
	MaxCleanupTimeout = 2 * time.Minute
	// MaxCloseTimeout 값은 caller가 지정할 수 있는 close 최대값이다.
	MaxCloseTimeout = time.Minute
	// MaxResultLimit 값은 각 materialized result의 최대 caller 상한이다.
	MaxResultLimit = 1024
)

// Config 값은 한 conformance run의 timeout과 result 상한을 고정한다.
type Config struct {
	// StartupTimeout 값은 factory와 provider readiness 상한이다.
	StartupTimeout time.Duration
	// CaseTimeout 값은 각 core 또는 capability callback 상한이다.
	CaseTimeout time.Duration
	// CleanupTimeout 값은 fixture cleanup 상한이다.
	CleanupTimeout time.Duration
	// CloseTimeout 값은 adapter close 상한이다.
	CloseTimeout time.Duration
	// MaxVertices 값은 한 read callback이 materialize할 수 있는 vertex 최대값이다.
	MaxVertices int
	// MaxEdges 값은 한 read callback이 materialize할 수 있는 edge 최대값이다.
	MaxEdges int
	// MaxTraversalResults 값은 한 traversal callback이 materialize할 수 있는 key 최대값이다.
	MaxTraversalResults int
}

// DefaultConfig 함수는 모든 필드가 유효한 기본 설정을 반환한다.
func DefaultConfig() Config {
	return Config{
		StartupTimeout: DefaultStartupTimeout, CaseTimeout: DefaultCaseTimeout,
		CleanupTimeout: DefaultCleanupTimeout, CloseTimeout: DefaultCloseTimeout,
		MaxVertices: 16, MaxEdges: 16, MaxTraversalResults: 32,
	}
}

// ProviderMetadata 값은 안전한 low-cardinality provider 진단값을 보존한다.
type ProviderMetadata struct {
	// Name 값은 credential을 포함하지 않는 low-cardinality provider 이름이다.
	Name string
	// Version 값은 관찰된 provider major/minor와 일치하는 안전한 버전이다.
	Version string
	// ImageReference 값은 mutable tag가 아닌 digest-pinned image reference다.
	ImageReference string
}

// Fixture 값은 backend-neutral semantic graph와 unique namespace를 보존한다.
type Fixture struct {
	namespace string
	vertices  []graph.Vertex
	edges     []graph.Edge
}

// Namespace 함수는 이 fixture의 안전한 unique namespace를 반환한다.
func (f Fixture) Namespace() string {
	return f.namespace
}

// Vertices 함수는 caller mutation과 격리된 vertex clone을 반환한다.
func (f Fixture) Vertices() []graph.Vertex {
	return cloneVertices(f.vertices)
}

// Edges 함수는 caller mutation과 격리된 edge clone을 반환한다.
func (f Fixture) Edges() []graph.Edge {
	return cloneEdges(f.edges)
}

// Validate 함수는 fixture invariant를 검사한다.
func (f Fixture) Validate() error {
	return validateFixture(f)
}

// Started 함수는 blocking provider I/O가 실제 cancellation boundary에 도달했음을 알린다.
type Started func()

// Adapter 값은 test-only semantic operation과 lifecycle callback을 묶는다.
type Adapter struct {
	// VerifyConnectivity 함수는 backend readiness를 다시 확인한다.
	VerifyConnectivity func(context.Context) error
	// CreateFixture 함수는 namespace로 격리된 semantic fixture를 만든다.
	CreateFixture func(context.Context, Fixture) error
	// ReadVertices 함수는 MaxVertices+1 query limit을 적용한 뒤 vertex를 반환한다.
	ReadVertices func(context.Context, Fixture) ([]graph.Vertex, error)
	// ReadEdges 함수는 MaxEdges+1 query limit을 적용한 뒤 edge를 반환한다.
	ReadEdges func(context.Context, Fixture) ([]graph.Edge, error)
	// InvalidOperation 함수는 provider-native 오류 분류를 검증할 오류를 만든다.
	InvalidOperation func(context.Context, Fixture) error
	// BlockUntilCanceled 함수는 blocking I/O 시작 뒤 Started를 정확히 한 번 호출한다.
	BlockUntilCanceled func(context.Context, Fixture, Started) error
	// CleanupFixture 함수는 같은 fixture에 반복 호출해도 안전해야 한다.
	CleanupFixture func(context.Context, Fixture) error
	// Close 함수는 adapter가 소유한 client 또는 driver를 정확히 한 번 닫는다.
	Close func(context.Context) error
	// Traverse 함수는 지원할 때 MaxTraversalResults+1 query limit을 적용한다.
	Traverse func(context.Context, Fixture) ([]string, error)
	// IsProviderError 함수는 직접 및 wrapped provider 오류를 분류한다.
	IsProviderError func(error) bool
}

// Factory 함수는 검증된 Config로 한 run이 소유할 Adapter를 만든다.
type Factory func(context.Context, testing.TB, Config) (Adapter, error)

// Capability 값은 optional conformance scenario의 안정적인 key다.
type Capability string

// CapabilityTraversal 값은 directed logical-key traversal 검증을 뜻한다.
const CapabilityTraversal Capability = "traversal"

// Support 값은 optional capability 활성 여부 또는 안전한 비활성 reason code를 보존한다.
type Support struct {
	// Enabled 값은 capability scenario 실행 여부다.
	Enabled bool
	// ReasonCode 값은 비활성 capability의 안전한 stable reason code다.
	ReasonCode string
}

// Capabilities 값은 알려진 optional capability의 완전한 snapshot source다.
type Capabilities map[Capability]Support

// Harness 값은 provider metadata, factory와 capability declaration을 묶는다.
type Harness struct {
	// Provider 값은 validation과 redacted diagnostics에 쓰는 metadata다.
	Provider ProviderMetadata
	// New 함수는 이 run이 소유할 adapter를 만든다.
	New Factory
	// Capabilities 값은 runner가 시작 전에 snapshot할 optional capability 선언이다.
	Capabilities Capabilities
}
