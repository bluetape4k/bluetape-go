# Issue #555 Graph Backend Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `graph/graphtest`에 backend-neutral strict conformance runner를 추가하고 Neo4j와 Memgraph가 같은 core 및 traversal 계약을 skip 없이 검증하게 한다.

**Architecture:** 공개 test-support package는 semantic `Fixture`, 함수 필드 기반 `Adapter`, strict `Harness`와 runner만 소유하며 production `graph`와 `graph/neo4j` API는 바꾸지 않는다. Runner는 validation, timeout, cancellation join, cleanup/close 순서, redacted diagnostics와 bounded result를 통제하고, provider별 고정 query와 Testcontainers lifecycle은 `graph/neo4j/conformance_test.go`의 adapter closure가 소유한다. 기존 provider test는 새 suite와 나란히 실행해 parity를 증명한 뒤 겹치는 integration body만 별도 commit에서 제거한다.

**Tech Stack:** Go 1.26.3, 표준 라이브러리 `context`/`crypto/rand`/`errors`/`testing`/`time`, 기존 `graph` model, Neo4j Go Driver v6, 기존 Testcontainers Go 및 `internal/testcleanup`

---

## 실행 경계

- **복잡도:** HIGH. 공개 test API, callback lifecycle, cancellation join, secret redaction, 두 Docker backend의 migration이 결합된다.
- **변경 유형:** Type A full feature. 구현 단계에서는 `$bluetape-go-patterns`, `$test-driven-development`, 완료 전 `$verification-before-completion`을 적용한다.
- **새 dependency:** 추가하지 않는다. `go.mod`와 `go.sum`은 `make tidy-check`에서 무변경이어야 한다.
- **Production API:** `graph/*.go`와 `graph/neo4j/*.go`의 exported production surface는 변경하지 않는다.
- **실행 순서:** Task 0 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10. Task 6의 fake-only 검증이 녹색이 된 뒤에만 Docker-backed Task 7을 시작한다.
- **Heavy test 직렬화:** `graph/neo4j` Testcontainers test와 저장소 전체 Testcontainers gate는 동시에 실행하지 않는다. 반복 conformance는 독립 `go test -count=1 -timeout=10m` process 세 개를 shell loop에서 순차 실행한다.
- **쓰기 범위:** 아래 `Create`/`Modify` 경로만 변경한다. `graph/provider_benchmark_test.go`의 digest는 읽기 기준으로만 사용하고 수정하지 않는다.

## 파일 구조

- `graph/graphtest/doc.go`: package boundary와 사용 목적.
- `graph/graphtest/types.go`: 공개 Config, ProviderMetadata, Fixture, Adapter, Capability, Harness API.
- `graph/graphtest/fixture.go`: cryptographic namespace, immutable semantic fixture, clone/canonical comparison helpers.
- `graph/graphtest/validate.go`: config/metadata/harness/capability/classifier fail-closed validation.
- `graph/graphtest/lifecycle.go`: callback panic recovery, timeout, cancellation join, cleanup와 close orchestration.
- `graph/graphtest/runner.go`: public `Run`/`RunWithConfig`와 ordered strict scenarios.
- `graph/graphtest/*_test.go`: Docker 없는 fake harness validation, lifecycle, redaction, examples.
- `graph/neo4j/conformance_test.go`: digest-pinned Neo4j/Memgraph factories와 fixed-query adapters.
- `graph/neo4j/client_test.go`, `graph/neo4j/memgraph_test.go`: parity 뒤 shared core와 중복되는 integration body 제거; pure conversion/constructor assertions 보존.
- README locale pair, root package table, `CHANGELOG.md`, `WIP.md`: public contract와 0.22.0 진행 근거 동기화.
- `docs/review/2026-09-06-issue-555-risk-prediction.md`: 구현 전에 고정하고 최종 검증에서 실제 신호를 덧붙이는 Step 3-P 증거.

### Task 0: Step 3-P 위험 예측을 구현 전에 고정

**Files:**
- Create: `docs/review/2026-09-06-issue-555-risk-prediction.md`

- [ ] **Step 1: 승인 artifact와 source 무변경 상태 확인**

Run:

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check
```

Expected: 승인된 spec/plan/review 문서만 branch에 있고 `graph/**/*.go` source 변경은 없다.

- [ ] **Step 2: 위험, 조기 신호, 완화, 중단/rollback owner 기록**

위 risk 문서에 callback non-cooperation, signal-timeout join, cleanup/close 경합, factory partial resource, pre-materialization `limit+1`, query submission loop, metadata/query/credential 노출, digest/readiness drift, old/new parity 손실, 10분 suite budget과 local Testcontainers flake를 각각 한 row로 기록한다. 각 row는 관찰 명령, PASS/PENDING/BLOCKED 판정, 복구 commit 또는 rerun owner를 포함한다. 이 문서는 source commit보다 먼저 생성한다.

- [ ] **Step 3: 위험 예측 artifact commit**

```bash
git add docs/review/2026-09-06-issue-555-risk-prediction.md
git commit -m "graph conformance 구현 전에 실패 신호를 고정한다" \
  -m "Constraint: lifecycle과 Docker provider 위험은 source 편집 전에 관찰 기준이 필요하다" \
  -m "Rejected: 구현 뒤 회고만 기록 | 예측과 실제 결과를 구분할 수 없다" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Directive: 위험 row를 삭제하지 말고 최종 관찰 결과만 덧붙인다" \
  -m "Tested: git diff --check; approved artifact read-back" -m "Not-tested: source와 provider 실행은 후속 task"
```

### Task 1: 공개 API와 strict validation 고정

**Files:**
- Create: `graph/graphtest/doc.go`
- Create: `graph/graphtest/types.go`
- Create: `graph/graphtest/validate.go`
- Test: `graph/graphtest/types_test.go`

- [ ] **Step 1: compile-time 공개 API와 기본값 test 작성**

`graph/graphtest/types_test.go`를 다음 내용으로 만든다.

```go
package graphtest

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	want := Config{
		StartupTimeout: 90 * time.Second, CaseTimeout: 10 * time.Second,
		CleanupTimeout: 30 * time.Second, CloseTimeout: 10 * time.Second,
		MaxVertices: 16, MaxEdges: 16, MaxTraversalResults: 32,
	}
	if got := DefaultConfig(); got != want {
		t.Fatalf("DefaultConfig() = %#v, want %#v", got, want)
	}
}

func TestRunWithConfigRejectsZeroConfigBeforeFactory(t *testing.T) {
	called := false
	h := Harness{
		Provider: ProviderMetadata{Name: "fake", Version: "1.0.0", ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New: func(context.Context, testing.TB, Config) (Adapter, error) {
			called = true
			return Adapter{}, nil
		},
		Capabilities: Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "not-implemented"}},
	}
	if err := validateHarness(h, Config{}); err == nil {
		t.Fatal("validateHarness() error = nil, want invalid config")
	}
	if called {
		t.Fatal("factory called for invalid config")
	}
}
```

- [ ] **Step 2: RED 확인**

Run: `go test -count=1 ./graph/graphtest -run 'Test(DefaultConfig|RunWithConfigRejectsZeroConfigBeforeFactory)$'`

Expected: FAIL with `no required module provides package github.com/bluetape4k/bluetape-go/graph/graphtest` 또는 undefined `graphtest` API.

- [ ] **Step 3: package doc와 공개 type 구현**

`graph/graphtest/doc.go`:

```go
// Package graphtest는 graph backend의 공통 의미, cancellation, cleanup 및
// 오류 경계를 검증하는 공개 test-support harness를 제공한다.
package graphtest
```

`graph/graphtest/types.go`:

```go
package graphtest

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

const (
	// DefaultStartupTimeout은 factory와 readiness 기본 상한이다.
	DefaultStartupTimeout = 90 * time.Second
	// DefaultCaseTimeout은 각 core/optional callback 기본 상한이다.
	DefaultCaseTimeout = 10 * time.Second
	// DefaultCleanupTimeout은 fixture cleanup 기본 상한이다.
	DefaultCleanupTimeout = 30 * time.Second
	// DefaultCloseTimeout은 adapter close 기본 상한이다.
	DefaultCloseTimeout = 10 * time.Second
	// MaxStartupTimeout은 caller가 지정할 수 있는 startup 최대값이다.
	MaxStartupTimeout = 5 * time.Minute
	// MaxCaseTimeout은 caller가 지정할 수 있는 case 최대값이다.
	MaxCaseTimeout = time.Minute
	// MaxCleanupTimeout은 caller가 지정할 수 있는 cleanup 최대값이다.
	MaxCleanupTimeout = 2 * time.Minute
	// MaxCloseTimeout은 caller가 지정할 수 있는 close 최대값이다.
	MaxCloseTimeout = time.Minute
	// MaxResultLimit은 각 materialized result의 최대 caller 상한이다.
	MaxResultLimit = 1024
)

// Config는 한 conformance run의 timeout과 result 상한을 고정한다.
type Config struct {
	// StartupTimeout은 factory와 provider readiness 상한이다.
	StartupTimeout time.Duration
	// CaseTimeout은 각 core 또는 capability callback 상한이다.
	CaseTimeout time.Duration
	// CleanupTimeout은 fixture cleanup 상한이다.
	CleanupTimeout time.Duration
	// CloseTimeout은 adapter close 상한이다.
	CloseTimeout time.Duration
	// MaxVertices는 한 read callback이 materialize할 수 있는 vertex 최대값이다.
	MaxVertices int
	// MaxEdges는 한 read callback이 materialize할 수 있는 edge 최대값이다.
	MaxEdges int
	// MaxTraversalResults는 한 traversal callback이 materialize할 수 있는 key 최대값이다.
	MaxTraversalResults int
}

// DefaultConfig는 모든 필드가 유효한 기본 설정을 반환한다.
func DefaultConfig() Config {
	return Config{
		StartupTimeout: DefaultStartupTimeout, CaseTimeout: DefaultCaseTimeout,
		CleanupTimeout: DefaultCleanupTimeout, CloseTimeout: DefaultCloseTimeout,
		MaxVertices: 16, MaxEdges: 16, MaxTraversalResults: 32,
	}
}

// ProviderMetadata는 안전한 low-cardinality provider 진단값을 보존한다.
type ProviderMetadata struct {
	// Name은 credential을 포함하지 않는 low-cardinality provider 이름이다.
	Name string
	// Version은 관찰된 provider major/minor와 일치하는 안전한 버전이다.
	Version string
	// ImageReference는 mutable tag가 아닌 digest-pinned image reference다.
	ImageReference string
}

// Fixture는 backend-neutral semantic graph와 unique namespace를 보존한다.
type Fixture struct {
	namespace string
	vertices []graph.Vertex
	edges []graph.Edge
}

// Namespace는 이 fixture의 안전한 unique namespace를 반환한다.
func (f Fixture) Namespace() string { return f.namespace }
// Vertices는 caller mutation과 격리된 vertex clone을 반환한다.
func (f Fixture) Vertices() []graph.Vertex { return cloneVertices(f.vertices) }
// Edges는 caller mutation과 격리된 edge clone을 반환한다.
func (f Fixture) Edges() []graph.Edge { return cloneEdges(f.edges) }
// Validate는 fixture invariant를 검사한다.
func (f Fixture) Validate() error { return validateFixture(f) }

// Started는 blocking provider I/O가 실제 cancellation boundary에 도달했음을 알린다.
type Started func()

// Adapter는 test-only semantic operation과 lifecycle callback을 묶는다.
type Adapter struct {
	// VerifyConnectivity는 backend readiness를 다시 확인한다.
	VerifyConnectivity func(context.Context) error
	// CreateFixture는 namespace로 격리된 semantic fixture를 만든다.
	CreateFixture func(context.Context, Fixture) error
	// ReadVertices는 MaxVertices+1 query limit을 적용한 뒤 vertex를 반환한다.
	ReadVertices func(context.Context, Fixture) ([]graph.Vertex, error)
	// ReadEdges는 MaxEdges+1 query limit을 적용한 뒤 edge를 반환한다.
	ReadEdges func(context.Context, Fixture) ([]graph.Edge, error)
	// InvalidOperation은 provider-native 오류 분류를 검증할 오류를 만든다.
	InvalidOperation func(context.Context, Fixture) error
	// BlockUntilCanceled는 blocking I/O 시작 뒤 Started를 정확히 한 번 호출한다.
	BlockUntilCanceled func(context.Context, Fixture, Started) error
	// CleanupFixture는 같은 fixture에 반복 호출해도 안전해야 한다.
	CleanupFixture func(context.Context, Fixture) error
	// Close는 adapter가 소유한 client 또는 driver를 정확히 한 번 닫는다.
	Close func(context.Context) error
	// Traverse는 지원할 때 MaxTraversalResults+1 query limit을 적용한다.
	Traverse func(context.Context, Fixture) ([]string, error)
	// IsProviderError는 직접 및 wrapped provider 오류를 분류한다.
	IsProviderError func(error) bool
}

// Factory는 검증된 Config로 한 run이 소유할 Adapter를 만든다.
type Factory func(context.Context, testing.TB, Config) (Adapter, error)
// Capability는 optional conformance scenario의 안정적인 key다.
type Capability string

// CapabilityTraversal은 directed logical-key traversal 검증을 뜻한다.
const CapabilityTraversal Capability = "traversal"

// Support는 optional capability 활성 여부 또는 안전한 비활성 reason code를 보존한다.
type Support struct {
	// Enabled는 capability scenario 실행 여부다.
	Enabled bool
	// ReasonCode는 비활성 capability의 안전한 stable reason code다.
	ReasonCode string
}

// Capabilities는 알려진 optional capability의 완전한 snapshot source다.
type Capabilities map[Capability]Support

// Harness는 provider metadata, factory와 capability declaration을 묶는다.
type Harness struct {
	// Provider는 validation과 redacted diagnostics에 쓰는 metadata다.
	Provider ProviderMetadata
	// New는 이 run이 소유할 adapter를 만든다.
	New Factory
	// Capabilities는 runner가 시작 전에 snapshot할 optional capability 선언이다.
	Capabilities Capabilities
}
```

`Factory`는 검증된 `Config`를 closure에 캡처해야 한다. `ReadVertices`, `ReadEdges`, `Traverse` 구현은 각각 대응 상한의 `limit+1`을 backend query에 적용한 뒤에만 slice를 materialize한다. Runner의 slice 길이 검사는 방어적 2차 gate이며, first-party provider는 Task 7의 request-builder 단위 테스트와 query-submission counter로 pre-materialization limit을 증명한다. Iterator/stream API는 이번 승인된 공개 shape를 바꾸므로 추가하지 않는다.

- [ ] **Step 4: fail-closed validation 구현**

`graph/graphtest/validate.go`에 다음 package-private validation entrypoint와 helper를 추가한다.

```go
package graphtest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/bluetape4k/bluetape-go/graph"
)

var (
	errInvalidConfig = errors.New("graphtest: invalid config")
	errInvalidProvider = errors.New("graphtest: invalid provider metadata")
	errInvalidHarness = errors.New("graphtest: invalid harness")
	reasonCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	metadataPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	imagePattern = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*:[A-Za-z0-9_][A-Za-z0-9._-]{0,127}@sha256:[0-9a-f]{64}$`)
)

func validateHarness(h Harness, cfg Config) error {
	if err := validateConfig(cfg); err != nil { return err }
	if err := validateProvider(h.Provider); err != nil { return err }
	if h.New == nil { return errInvalidHarness }
	if len(h.Capabilities) != 1 { return errInvalidHarness }
	support, ok := h.Capabilities[CapabilityTraversal]
	if !ok { return errInvalidHarness }
	if support.Enabled {
		if support.ReasonCode != "" { return errInvalidHarness }
	} else if !reasonCodePattern.MatchString(support.ReasonCode) {
		return errInvalidHarness
	}
	return nil
}

func validateConfig(c Config) error {
	if c.StartupTimeout <= 0 || c.StartupTimeout > MaxStartupTimeout ||
		c.CaseTimeout <= 0 || c.CaseTimeout > MaxCaseTimeout ||
		c.CleanupTimeout <= 0 || c.CleanupTimeout > MaxCleanupTimeout ||
		c.CloseTimeout <= 0 || c.CloseTimeout > MaxCloseTimeout ||
		c.MaxVertices < 1 || c.MaxVertices > MaxResultLimit ||
		c.MaxEdges < 1 || c.MaxEdges > MaxResultLimit ||
		c.MaxTraversalResults < 1 || c.MaxTraversalResults > MaxResultLimit {
		return errInvalidConfig
	}
	return nil
}

func validateProvider(p ProviderMetadata) error {
	if !metadataPattern.MatchString(p.Name) || !metadataPattern.MatchString(p.Version) {
		return errInvalidProvider
	}
	if strings.TrimSpace(p.ImageReference) == "" || len(p.ImageReference) > 256 ||
		strings.ContainsAny(p.ImageReference, "\r\n") || strings.IndexFunc(p.ImageReference, unicode.IsControl) >= 0 {
		return errInvalidProvider
	}
	if strings.Contains(p.ImageReference, "://") || strings.Contains(p.ImageReference, "?") || !imagePattern.MatchString(p.ImageReference) {
		return errInvalidProvider
	}
	return nil
}

func validateAdapter(a Adapter, caps Capabilities) error {
	if a.VerifyConnectivity == nil || a.CreateFixture == nil || a.ReadVertices == nil ||
		a.ReadEdges == nil || a.InvalidOperation == nil || a.BlockUntilCanceled == nil ||
		a.CleanupFixture == nil || a.Close == nil || a.IsProviderError == nil {
		return errInvalidHarness
	}
	if caps[CapabilityTraversal].Enabled != (a.Traverse != nil) { return errInvalidHarness }
	for _, probe := range []error{nil, context.Canceled, context.DeadlineExceeded, graph.ErrInvalidVertex, errors.New("raw cause")} {
		matched, panicked := classify(a.IsProviderError, probe)
		if panicked || matched { return errInvalidHarness }
	}
	return nil
}

func classify(fn func(error) bool, err error) (matched bool, panicked bool) {
	defer func() { panicked = recover() != nil }()
	return fn(err), false
}

func category(phase string, err error) error {
	if err == nil { return nil }
	return &phaseError{phase: phase, cause: err}
}

func imageDigest(reference string) string {
	_, digest, _ := strings.Cut(reference, "@")
	return digest
}

type phaseError struct { phase string; cause error }
func (e *phaseError) Error() string { return fmt.Sprintf("graphtest: %s failed", e.phase) }
func (e *phaseError) Unwrap() error { return e.cause }
```

- [ ] **Step 5: validation matrix 보강**

`graph/graphtest/types_test.go`에 boundary table을 추가한다.

```go
func TestValidateRejectsUnsafeMetadataAndConfigBoundaries(t *testing.T) {
	t.Parallel()
	valid := DefaultConfig()
	base := Harness{
		Provider: ProviderMetadata{Name: "fake", Version: "1.0.0", ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New: func(context.Context, testing.TB, Config) (Adapter, error) { return Adapter{}, nil },
		Capabilities: Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "unsupported-by-fake"}},
	}
	for _, tc := range []struct{name string; mutate func(*Harness, *Config)}{
		{"zero-startup", func(_ *Harness, c *Config) { c.StartupTimeout = 0 }},
		{"startup-over-max", func(_ *Harness, c *Config) { c.StartupTimeout = MaxStartupTimeout + time.Nanosecond }},
		{"result-over-max", func(_ *Harness, c *Config) { c.MaxVertices = MaxResultLimit + 1 }},
		{"credential-uri", func(h *Harness, _ *Config) { h.Provider.ImageReference = "bolt://user:secret@example" }},
		{"newline-name", func(h *Harness, _ *Config) { h.Provider.Name = "fake\nquery" }},
		{"credential-like-name", func(h *Harness, _ *Config) { h.Provider.Name = "user:secret" }},
		{"credential-like-version", func(h *Harness, _ *Config) { h.Provider.Version = "https://token@example" }},
		{"credential-image", func(h *Harness, _ *Config) { h.Provider.ImageReference = "registry/path:user:secret@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }},
		{"shell-image", func(h *Harness, _ *Config) { h.Provider.ImageReference = "repo/image:1;echo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }},
		{"bidi-image", func(h *Harness, _ *Config) { h.Provider.ImageReference = "repo/image:\u202e1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }},
		{"line-separator-image", func(h *Harness, _ *Config) { h.Provider.ImageReference = "repo/image:1\u2028@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }},
		{"missing-capability", func(h *Harness, _ *Config) { h.Capabilities = nil }},
		{"unsafe-reason", func(h *Harness, _ *Config) { h.Capabilities[CapabilityTraversal] = Support{ReasonCode: "not supported\nMATCH (n)"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg := base, valid
			h.Capabilities = Capabilities{CapabilityTraversal: base.Capabilities[CapabilityTraversal]}
			tc.mutate(&h, &cfg)
			if err := validateHarness(h, cfg); err == nil { t.Fatal("validateHarness() error = nil") }
		})
	}
}
```

- [ ] **Step 6: GREEN 확인**

Run: `gofmt -w graph/graphtest && go test -count=1 ./graph/graphtest`

Expected: PASS; Docker container 0개 생성.

- [ ] **Step 7: Lore commit**

```bash
git add graph/graphtest/doc.go graph/graphtest/types.go graph/graphtest/validate.go graph/graphtest/types_test.go
git commit -m "graph backend 계약을 실행 전에 거절할 수 있게 한다" \
  -m "Constraint: Config와 provider metadata는 resource 생성 전에 검증한다" \
  -m "Rejected: production graph interface 추가 | test-support 경계를 넘는다" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: 새 core callback은 capability로 끌 수 없게 유지한다" \
  -m "Tested: go test -count=1 ./graph/graphtest" -m "Not-tested: provider Testcontainers 연결"
```

### Task 2: immutable semantic fixture와 bounded comparison 구현

**Files:**
- Create: `graph/graphtest/fixture.go`
- Test: `graph/graphtest/fixture_test.go`
- Modify: `graph/graphtest/types.go`

- [ ] **Step 1: namespace, clone, sort-before-limit 금지 test 작성**

`graph/graphtest/fixture_test.go`:

```go
package graphtest

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestNewFixtureIsValidAndDefensivelyCopied(t *testing.T) {
	f, err := newFixture()
	if err != nil { t.Fatalf("newFixture() error = %v", err) }
	if err := f.Validate(); err != nil { t.Fatalf("Validate() error = %v", err) }
	if !strings.HasPrefix(f.Namespace(), "btgc_") || len(f.Namespace()) != len("btgc_")+32 { t.Fatalf("namespace = %q", f.Namespace()) }
	vertices := f.Vertices()
	vertices[0], _ = graph.ParseVertex("changed", "Changed", nil)
	if f.Vertices()[0].ID().String() == "changed" { t.Fatal("fixture leaked vertex mutation") }
}

func TestCanonicalVerticesRejectsLimitBeforeSorting(t *testing.T) {
	f, _ := newFixture()
	got := append(f.Vertices(), f.Vertices()[0])
	if _, err := canonicalVertices(got, 2); err == nil { t.Fatal("canonicalVertices() error = nil") }
}
```

- [ ] **Step 2: RED 확인**

Run: `go test -count=1 ./graph/graphtest -run 'Test(NewFixture|CanonicalVertices)'`

Expected: FAIL with undefined `newFixture` and `canonicalVertices`.

- [ ] **Step 3: fixture와 clone 구현**

`graph/graphtest/fixture.go`:

```go
package graphtest

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

const logicalKeyProperty = "btgc_key"

func newFixture() (Fixture, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil { return Fixture{}, errors.New("graphtest: namespace generation failed") }
	ns := "btgc_" + hex.EncodeToString(raw[:])
	left, err := graph.ParseVertex("left", "BTGCSource", graph.Properties{logicalKeyProperty: "left", "rank": int64(1), "namespace": ns})
	if err != nil { return Fixture{}, err }
	right, err := graph.ParseVertex("right", "BTGCTarget", graph.Properties{logicalKeyProperty: "right", "active": true, "namespace": ns})
	if err != nil { return Fixture{}, err }
	edge, err := graph.ParseEdge("left-right", "BTGC_LINKS", graph.RawEdgeEndpoints{Start: "left", End: "right"}, graph.Properties{logicalKeyProperty: "left-right", "weight": int64(7), "namespace": ns})
	if err != nil { return Fixture{}, err }
	return Fixture{namespace: ns, vertices: []graph.Vertex{left, right}, edges: []graph.Edge{edge}}, nil
}

func validateFixture(f Fixture) error {
	if len(f.namespace) > 64 || !strings.HasPrefix(f.namespace, "btgc_") { return errInvalidHarness }
	if len(f.vertices) != 2 || len(f.edges) != 1 { return errInvalidHarness }
	for _, v := range f.vertices { if err := v.Validate(); err != nil { return errInvalidHarness } }
	for _, e := range f.edges { if err := e.Validate(); err != nil { return errInvalidHarness } }
	return nil
}

func cloneVertices(src []graph.Vertex) []graph.Vertex {
	out := make([]graph.Vertex, len(src))
	for i, v := range src { out[i], _ = graph.NewVertex(v.ID(), v.Label(), v.Properties()) }
	return out
}

func cloneEdges(src []graph.Edge) []graph.Edge {
	out := make([]graph.Edge, len(src))
	for i, e := range src { out[i], _ = graph.NewEdge(e.ID(), e.Label(), graph.EdgeEndpoints{Start: e.StartID(), End: e.EndID()}, e.Properties()) }
	return out
}

func logicalKey(properties graph.Properties) (string, bool) {
	value, ok := properties[logicalKeyProperty].(string)
	return value, ok && value != ""
}

func canonicalVertices(values []graph.Vertex, limit int) ([]graph.Vertex, error) {
	if len(values) > limit { return nil, errors.New("graphtest: vertex result limit exceeded") }
	out := cloneVertices(values)
	for _, v := range out { if _, ok := logicalKey(v.Properties()); !ok { return nil, errInvalidHarness } }
	sort.Slice(out, func(i, j int) bool { a, _ := logicalKey(out[i].Properties()); b, _ := logicalKey(out[j].Properties()); return a < b })
	return out, nil
}

func canonicalEdges(values []graph.Edge, limit int) ([]graph.Edge, error) {
	if len(values) > limit { return nil, errors.New("graphtest: edge result limit exceeded") }
	out := cloneEdges(values)
	for _, e := range out { if _, ok := logicalKey(e.Properties()); !ok { return nil, errInvalidHarness } }
	sort.Slice(out, func(i, j int) bool { a, _ := logicalKey(out[i].Properties()); b, _ := logicalKey(out[j].Properties()); return a < b })
	return out, nil
}
```

- [ ] **Step 4: fixture/canonical GREEN 확인**

Run: `gofmt -w graph/graphtest && go test -count=1 ./graph/graphtest -run 'Test(NewFixture|CanonicalVertices)'`

Expected: PASS.

- [ ] **Step 5: Lore commit**

```bash
git add graph/graphtest/types.go graph/graphtest/fixture.go graph/graphtest/fixture_test.go
git commit -m "backend 결과를 동일한 semantic fixture로 비교한다" \
  -m "Constraint: backend element ID와 반환 순서는 계약이 아니다" \
  -m "Rejected: raw query를 Fixture에 저장 | provider 경계를 누출한다" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: 결과 상한은 sort 이전에 fail closed로 검사한다" \
  -m "Tested: go test -count=1 ./graph/graphtest" -m "Not-tested: Neo4j와 Memgraph 변환"
```

### Task 3: adapter와 capability 실행 기준 복사 검증

**Files:**
- Modify: `graph/graphtest/validate.go`
- Test: `graph/graphtest/validate_test.go`

- [ ] **Step 1: callback/classifier/capability mutation RED test 작성**

`graph/graphtest/validate_test.go`에 complete adapter factory와 negative probes를 둔다.

```go
package graphtest

import (
	"context"
	"errors"
	"testing"
)

func completeAdapter() Adapter {
	return Adapter{
		VerifyConnectivity: func(context.Context) error { return nil },
		CreateFixture: func(context.Context, Fixture) error { return nil },
		ReadVertices: func(context.Context, Fixture) ([]graph.Vertex, error) { return []graph.Vertex{}, nil },
		ReadEdges: func(context.Context, Fixture) ([]graph.Edge, error) { return []graph.Edge{}, nil },
		InvalidOperation: func(context.Context, Fixture) error { return errors.New("provider") },
		BlockUntilCanceled: func(ctx context.Context, _ Fixture, started Started) error { started(); <-ctx.Done(); return ctx.Err() },
		CleanupFixture: func(context.Context, Fixture) error { return nil },
		Close: func(context.Context) error { return nil },
		Traverse: func(context.Context, Fixture) ([]string, error) { return []string{"left", "right"}, nil },
		IsProviderError: func(err error) bool { return err != nil && err.Error() == "provider" },
	}
}

func TestValidateAdapterRejectsMissingAndOverbroadClassifier(t *testing.T) {
	a := completeAdapter()
	caps := Capabilities{CapabilityTraversal: {Enabled: true}}
	if err := validateAdapter(a, caps); err != nil { t.Fatalf("validateAdapter() error = %v", err) }
	a.ReadEdges = nil
	if err := validateAdapter(a, caps); err == nil { t.Fatal("nil ReadEdges accepted") }
	a = completeAdapter()
	a.IsProviderError = func(error) bool { return true }
	if err := validateAdapter(a, caps); err == nil { t.Fatal("always-true classifier accepted") }
}

func TestSnapshotCapabilitiesIsIndependent(t *testing.T) {
	source := Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "unsupported-by-fake"}}
	got := snapshotCapabilities(source)
	source[CapabilityTraversal] = Support{Enabled: true}
	if got[CapabilityTraversal].Enabled { t.Fatal("snapshot observed caller mutation") }
}
```

이 파일 import에 `github.com/bluetape4k/bluetape-go/graph`를 함께 넣는다.

- [ ] **Step 2: RED 확인**

Run: `go test -count=1 ./graph/graphtest -run 'Test(ValidateAdapter|SnapshotCapabilities)'`

Expected: FAIL with undefined `snapshotCapabilities`; classifier nested-positive probe도 아직 구현되지 않았으므로 validation 부족이 드러난다.

- [ ] **Step 3: capability 실행 기준 복사와 classifier positive probe 구현**

`graph/graphtest/validate.go`에 다음 helper를 추가하고 `validateAdapter`의 negative probe 뒤에 호출한다.

```go
type providerProbeError struct{ cause error }

func (*providerProbeError) Error() string { return "graphtest: provider probe failed" }
func (e *providerProbeError) Unwrap() error { return e.cause }

func snapshotCapabilities(source Capabilities) Capabilities {
	out := make(Capabilities, len(source))
	for key, support := range source { out[key] = support }
	return out
}

func validatePositiveClassifier(classifier func(error) bool, providerErr error) error {
	if providerErr == nil { return errInvalidHarness }
	matched, panicked := classify(classifier, providerErr)
	if panicked || !matched { return errInvalidHarness }
	matched, panicked = classify(classifier, fmt.Errorf("nested: %w", providerErr))
	if panicked || !matched { return errInvalidHarness }
	return nil
}
```

`validateAdapter`는 `InvalidOperation`을 validation 중 호출하지 않는다. 실제 fixture와 bounded context가 준비된 provider-error scenario에서 반환된 오류를 `validatePositiveClassifier`에 전달한다. 이 순서를 `runner.go` test에서 고정한다.

- [ ] **Step 4: GREEN과 race 확인**

Run: `gofmt -w graph/graphtest && go test -race -count=1 ./graph/graphtest`

Expected: PASS, `WARNING: DATA RACE` 0건.

- [ ] **Step 5: Lore commit**

```bash
git add graph/graphtest/validate.go graph/graphtest/validate_test.go
git commit -m "callback과 capability 검증을 fail closed로 고정한다" \
  -m "Constraint: caller의 map mutation은 실행 중 계약을 바꾸지 못한다" \
  -m "Rejected: classifier를 단일 positive case로만 신뢰 | always true 오탐을 놓친다" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: provider error probe는 raw cause를 로그에 출력하지 않는다" \
  -m "Tested: go test -race -count=1 ./graph/graphtest" -m "Not-tested: 실제 driver error 분류"
```

### Task 4: timeout, panic, cancellation join과 lifecycle 순서 구현

**Files:**
- Create: `graph/graphtest/lifecycle.go`
- Test: `graph/graphtest/lifecycle_test.go`

- [ ] **Step 1: cleanup/close order와 join-before-cleanup RED test 작성**

`graph/graphtest/lifecycle_test.go`:

```go
package graphtest

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestRunCancellationJoinsBeforeCleanupAndClose(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(value string) { mu.Lock(); defer mu.Unlock(); events = append(events, value) }
	a := completeAdapter()
	a.BlockUntilCanceled = func(ctx context.Context, _ Fixture, started Started) error {
		record("started"); started(); <-ctx.Done(); record("joined"); return ctx.Err()
	}
	a.CleanupFixture = func(context.Context, Fixture) error { record("cleanup"); return nil }
	a.Close = func(context.Context) error { record("close"); return nil }
	f, _ := newFixture()
	cfg := DefaultConfig(); cfg.CaseTimeout = 50 * time.Millisecond
	if err := exerciseCancellation(context.Background(), a, f, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("exerciseCancellation() error = %v, want context.Canceled", err)
	}
	if err := cleanupAndClose(context.Background(), a, f, cfg); err != nil { t.Fatalf("cleanupAndClose() error = %v", err) }
	if want := []string{"started", "joined", "cleanup", "close"}; !reflect.DeepEqual(events, want) { t.Fatalf("events = %v, want %v", events, want) }
}
```

- [ ] **Step 2: RED 확인**

Run: `go test -count=1 ./graph/graphtest -run TestRunCancellationJoinsBeforeCleanupAndClose`

Expected: FAIL with undefined `exerciseCancellation` and `cleanupAndClose`.

- [ ] **Step 3: bounded operation과 cancellation handshake 구현**

`graph/graphtest/lifecycle.go`:

```go
package graphtest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type callbackResult[T any] struct { value T; err error; panicValue any; contextErr error }

var (
	errCancellationReturnedBeforeStart = errors.New("graphtest: cancellation returned before start")
	errCancellationStartTimeout = errors.New("graphtest: cancellation start timeout")
	errCancellationDuplicateStart = errors.New("graphtest: started called more than once")
)

func call[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) callbackResult[T] {
	if err := ctx.Err(); err != nil { return callbackResult[T]{contextErr: err} }
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan callbackResult[T], 1)
	go func() {
		result := callbackResult[T]{}
		defer func() { result.panicValue = recover(); done <- result }()
		result.value, result.err = fn(opCtx)
	}()
	select {
	case result := <-done:
		if err := opCtx.Err(); err != nil { result.contextErr = err }
		return result
	case <-opCtx.Done():
		cancel()
		result := <-done
		result.contextErr = opCtx.Err()
		return result
	}
}

func callbackError[T any](phase string, result callbackResult[T]) error {
	if result.panicValue != nil {
		return fmt.Errorf("graphtest: %s panic", phase)
	}
	if result.contextErr != nil { return fmt.Errorf("graphtest: %s context: %w", phase, result.contextErr) }
	return category(phase, result.err)
}

func callbackStatus[T any](result callbackResult[T]) (status, category string, timedOut bool) {
	switch {
	case result.panicValue != nil:
		return "error", "panic", false
	case errors.Is(result.contextErr, context.DeadlineExceeded):
		return "error", "timeout", true
	case result.contextErr != nil:
		return "error", "canceled", false
	case result.err != nil:
		return "error", "provider", false
	default:
		return "ok", "none", false
	}
}

func exerciseCancellation(parent context.Context, a Adapter, f Fixture, cfg Config) error {
	ctx, cancel := context.WithTimeout(parent, cfg.CaseTimeout)
	defer cancel()
	startedCh := make(chan struct{})
	cancelIssued := make(chan struct{})
	var once sync.Once
	var duplicate atomic.Bool
	done := make(chan callbackResult[struct{}], 1)
	go func() {
		result := callbackResult[struct{}]{}
		defer func() { result.panicValue = recover(); done <- result }()
		result.err = a.BlockUntilCanceled(ctx, f, func() {
			called := false
			once.Do(func() { called = true; close(startedCh) })
			if !called { duplicate.Store(true) }
			<-cancelIssued
		})
	}()
	timer := time.NewTimer(cfg.CaseTimeout)
	defer timer.Stop()
	select {
	case <-startedCh:
		cancel()
		close(cancelIssued)
	case result := <-done:
		cancel(); close(cancelIssued)
		if result.panicValue != nil { return errors.New("graphtest: cancellation callback panic") }
		return errCancellationReturnedBeforeStart
	case <-timer.C:
		cancel(); close(cancelIssued)
		result := <-done
		return errors.Join(errCancellationStartTimeout, callbackError("cancellation", result))
	}
	grace := cfg.CaseTimeout / 10
	if cfg.CloseTimeout < grace { grace = cfg.CloseTimeout }
	graceTimer := time.NewTimer(grace)
	defer graceTimer.Stop()
	var result callbackResult[struct{}]
	select {
	case result = <-done:
	case <-graceTimer.C:
		result = <-done
	}
	if duplicate.Load() { return errCancellationDuplicateStart }
	if result.panicValue != nil { return errors.New("graphtest: cancellation callback panic") }
	return result.err
}

func cleanupAndClose(parent context.Context, a Adapter, f Fixture, cfg Config) error {
	base := context.WithoutCancel(parent)
	cleanup := call(base, cfg.CleanupTimeout, func(ctx context.Context) (struct{}, error) { return struct{}{}, a.CleanupFixture(ctx, f) })
	closeResult := call(base, cfg.CloseTimeout, func(ctx context.Context) (struct{}, error) { return struct{}{}, a.Close(ctx) })
	return errors.Join(callbackError("fixture cleanup", cleanup), callbackError("close", closeResult))
}
```

위 코드 import에 `fmt`를 포함한다. `call`의 timeout branch와 `Started` 신호 timeout branch는 callback을 abandon하지 않고 join한다. callback이 context를 무시하면 외부 `go test -timeout`이 fail-stop을 담당하며, cleanup/close와 활성 callback이 겹치지 않는다. `join grace = min(CloseTimeout, CaseTimeout/10)`은 timeout diagnostic을 시작하는 기준으로 계산하되 grace 경과 뒤에도 goroutine을 분리하지 않는다.

- [ ] **Step 4: panic, missing/duplicate Started, canceled cleanup context matrix 추가**

동일 test file에 table test를 추가해 factory/callback/cleanup/close `panic`, signal 전 반환, signal 누락, 중복 signal을 각각 named error로 검증하고 `CleanupFixture`가 `context.WithoutCancel(parent)`에서 파생된 positive deadline context를 받는지 검사한다. Pre-canceled context에서는 callback 호출 count가 0이어야 하며, cancellation callback은 `CaseTimeout` deadline이 설정된 context를 받아야 한다. Callback 완료와 deadline이 동시에 준비된 경우, 취소 뒤 nil success, deadline 직전/직후 late success는 `context.Canceled` 또는 `context.DeadlineExceeded`가 성공보다 우선하고 `errors.Is`로 관찰돼야 한다. `Started`는 runner가 cancel을 발행할 때까지 callback으로 복귀하지 않으므로 signal 직후 가짜 `context.Canceled` 반환이 실제 cancel보다 앞설 수 없다. Cleanup/close panic row는 `run`이 nil을 반환하지 않으며 raw panic value가 출력되지 않음을 확인한다. 각 row는 `CaseTimeout=50ms`를 사용하며 non-cooperative callback과 signal-timeout 뒤 join을 거부하는 callback은 unit process를 멈출 수 있으므로 직접 실행하지 않고 subprocess test에서 `go test -timeout=200ms`의 non-zero 종료를 검증한다.

```go
func TestCleanupIgnoresParentCancellationButKeepsDeadline(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background()); cancel()
	a := completeAdapter()
	a.CleanupFixture = func(ctx context.Context, _ Fixture) error {
		if ctx.Err() != nil { return errors.New("cleanup inherited cancellation") }
		if _, ok := ctx.Deadline(); !ok { return errors.New("cleanup missing deadline") }
		return nil
	}
	f, _ := newFixture()
	if err := cleanupAndClose(parent, a, f, DefaultConfig()); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 5: GREEN과 race 확인**

Run: `gofmt -w graph/graphtest && go test -race -count=1 ./graph/graphtest -run 'Test(RunCancellation|Cleanup)'`

Expected: PASS, goroutine leak report와 race 0건.

- [ ] **Step 6: Lore commit**

```bash
git add graph/graphtest/lifecycle.go graph/graphtest/lifecycle_test.go
git commit -m "callback 종료 뒤에만 graph resource를 정리한다" \
  -m "Constraint: non cooperative callback은 bounded PASS로 위장할 수 없다" \
  -m "Rejected: timeout 뒤 goroutine 분리 | driver close와 callback을 경합시킨다" \
  -m "Confidence: medium" -m "Scope-risk: broad" \
  -m "Directive: cleanup과 close는 callback join 이후 순서를 유지한다" \
  -m "Tested: go test -race -count=1 ./graph/graphtest -run Test" -m "Not-tested: 실제 driver blocking boundary"
```

### Task 5: ordered strict core와 optional traversal runner 구현

**Files:**
- Create: `graph/graphtest/runner.go`
- Create: `graph/graphtest/fake_test.go`
- Test: `graph/graphtest/runner_test.go`

- [ ] **Step 1: runner RED test가 사용할 deterministic fake adapter 작성**

`graph/graphtest/fake_test.go`에 mutex로 보호한 in-memory state와 callback별 query counter를 둔다. Fixture accessor로 받은 value만 clone하고 raw query surface는 만들지 않는다. Factory가 호출될 때마다 fresh state를 만들며 typed `providerProbeError`와 `errors.As` classifier를 사용한다. `CreateFixture`는 `f.Vertices()`/`f.Edges()`를 저장하고 read callback은 저장한 slice의 clone을 반환한다. `CleanupFixture`는 두 slice를 비우고 query counter를 한 번 증가시키며 `Close`는 exactly-once counter를 증가시킨다. Cancellation callback은 `Started`를 정확히 한 번 호출한 뒤 `ctx.Done()`을 기다린다. Traversal은 `[]string{"left", "right"}`를 반환한다.

```go
package graphtest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

type fakeState struct {
	mu sync.Mutex
	vertices []graph.Vertex
	edges []graph.Edge
	queries map[string]int
	closed int
}

func validFakeHarness(mutate func(*Adapter)) Harness {
	return Harness{
		Provider: ProviderMetadata{Name: "fake", Version: "1.0.0", ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New: func(context.Context, testing.TB, Config) (Adapter, error) {
			state := &fakeState{queries: make(map[string]int)}
			a := Adapter{
				VerifyConnectivity: func(ctx context.Context) error { return ctx.Err() },
				CreateFixture: func(ctx context.Context, f Fixture) error {
					if err := ctx.Err(); err != nil { return err }
					state.mu.Lock(); defer state.mu.Unlock()
					state.queries["create"]++
					state.vertices, state.edges = f.Vertices(), f.Edges()
					return nil
				},
				ReadVertices: func(ctx context.Context, _ Fixture) ([]graph.Vertex, error) {
					if err := ctx.Err(); err != nil { return nil, err }
					state.mu.Lock(); defer state.mu.Unlock()
					state.queries["vertices"]++
					return cloneVertices(state.vertices), nil
				},
				ReadEdges: func(ctx context.Context, _ Fixture) ([]graph.Edge, error) {
					if err := ctx.Err(); err != nil { return nil, err }
					state.mu.Lock(); defer state.mu.Unlock()
					state.queries["edges"]++
					return cloneEdges(state.edges), nil
				},
				InvalidOperation: func(context.Context, Fixture) error { return &providerProbeError{cause: errors.New("fake cause")} },
				BlockUntilCanceled: func(ctx context.Context, _ Fixture, started Started) error { started(); <-ctx.Done(); return ctx.Err() },
				CleanupFixture: func(context.Context, Fixture) error {
					state.mu.Lock(); defer state.mu.Unlock()
					state.queries["cleanup"]++
					state.vertices, state.edges = nil, nil
					return nil
				},
				Close: func(context.Context) error {
					state.mu.Lock(); defer state.mu.Unlock()
					state.closed++
					if state.closed > 1 { return errors.New("fake adapter closed more than once") }
					return nil
				},
				Traverse: func(context.Context, Fixture) ([]string, error) {
					state.mu.Lock(); defer state.mu.Unlock()
					state.queries["traverse"]++
					return []string{"left", "right"}, nil
				},
				IsProviderError: func(err error) bool { var target *providerProbeError; return errors.As(err, &target) },
			}
			if mutate != nil { mutate(&a) }
			return a, nil
		},
		Capabilities: Capabilities{CapabilityTraversal: {Enabled: true}},
	}
}
```

- [ ] **Step 2: core 순서, fail-fast, cleanup/close exactly-once RED test 작성**

`graph/graphtest/runner_test.go`는 `run` package-private 함수가 error를 반환하도록 검증한다. Public `Run`은 이 error를 redacted `t.Fatal`로 변환한다.

```go
package graphtest

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestRunExecutesCoreAndTraversalWithExactlyOnceClose(t *testing.T) {
	var closed atomic.Int64
	h := validFakeHarness(func(a *Adapter) { a.Close = func(context.Context) error { closed.Add(1); return nil } })
	if err := run(context.Background(), t, h, DefaultConfig()); err != nil { t.Fatalf("run() error = %v", err) }
	if got := closed.Load(); got != 1 { t.Fatalf("close count = %d, want 1", got) }
}

func TestRunStopsAfterCoreFailureButCleansAndCloses(t *testing.T) {
	var cleaned, closed atomic.Int64
	h := validFakeHarness(func(a *Adapter) {
		a.ReadVertices = func(context.Context, Fixture) ([]graph.Vertex, error) { return nil, errors.New("secret-marker") }
		a.CleanupFixture = func(context.Context, Fixture) error { cleaned.Add(1); return nil }
		a.Close = func(context.Context) error { closed.Add(1); return nil }
	})
	err := run(context.Background(), t, h, DefaultConfig())
	if err == nil { t.Fatal("run() error = nil") }
	if strings.Contains(err.Error(), "secret-marker") { t.Fatal("run() disclosed the secret marker") }
	if cleaned.Load() == 0 || closed.Load() != 1 { t.Fatalf("cleanup=%d close=%d", cleaned.Load(), closed.Load()) }
}
```

이 test file import에 `strings`와 `github.com/bluetape4k/bluetape-go/graph`를 포함한다. Step 1의 `validFakeHarness`를 사용하므로 RED 원인은 runner entrypoint 부재로 한정한다.

- [ ] **Step 3: RED 확인**

Run: `go test -count=1 ./graph/graphtest -run '^TestRun'`

Expected: FAIL with undefined `run`; `validFakeHarness` 자체는 compile된다.

- [ ] **Step 4: public entrypoint와 ordered case table 구현**

`graph/graphtest/runner.go`에 다음 entrypoint와 orchestration skeleton을 구현한다.

```go
package graphtest

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Run은 t.Context를 parent로 사용해 기본 Config로 strict conformance suite를 실행한다.
func Run(t *testing.T, harness Harness) { t.Helper(); RunWithConfig(t, harness, DefaultConfig()) }

// RunWithConfig는 t.Context의 cancellation/deadline을 소유하지 않고 전달한다.
// Config의 zero value나 partial value는 허용하지 않는다.
func RunWithConfig(t *testing.T, harness Harness, config Config) {
	t.Helper()
	if err := run(t.Context(), t, harness, config); err != nil { t.Fatal(err) }
}

func run(parent context.Context, t *testing.T, h Harness, cfg Config) (returnErr error) {
	if err := validateHarness(h, cfg); err != nil { return err }
	caps := snapshotCapabilities(h.Capabilities)
	startup := call(parent, cfg.StartupTimeout, func(ctx context.Context) (Adapter, error) { return h.New(ctx, t, cfg) })
	if err := callbackError("factory", startup); err != nil { return err }
	adapter := startup.value
	defer func() {
		if recover() != nil { returnErr = errors.Join(returnErr, errors.New("graphtest: runner panic")) }
		closeStarted := time.Now()
		closeResult := call(context.WithoutCancel(parent), cfg.CloseTimeout, func(ctx context.Context) (struct{}, error) { return struct{}{}, adapter.Close(ctx) })
		closeStatus, closeCategory, closeTimedOut := callbackStatus(closeResult)
		t.Logf("graphtest provider=%s phase=close status=%s category=%s timeout=%t duration=%s", h.Provider.Name, closeStatus, closeCategory, closeTimedOut, time.Since(closeStarted))
		returnErr = errors.Join(returnErr, callbackError("close", closeResult))
	}()
	if err := validateAdapter(adapter, caps); err != nil { return err }
	t.Logf("graphtest provider=%s version=%s image_digest=%s phase=start", h.Provider.Name, h.Provider.Version, imageDigest(h.Provider.ImageReference))
	defer func(start time.Time) { t.Logf("graphtest provider=%s version=%s phase=run duration=%s", h.Provider.Name, h.Provider.Version, time.Since(start)) }(time.Now())
	traversalSupport := caps[CapabilityTraversal]
	if traversalSupport.Enabled {
		t.Logf("graphtest provider=%s capability=%s status=enabled", h.Provider.Name, CapabilityTraversal)
	} else {
		t.Logf("graphtest provider=%s capability=%s status=disabled reason=%s", h.Provider.Name, CapabilityTraversal, traversalSupport.ReasonCode)
	}

	var scenarioErr error
	for _, tc := range []struct{name string; run func(context.Context, Adapter, Fixture, Config) error}{
		{"connectivity", caseConnectivity}, {"empty-read", caseEmptyRead}, {"create-read", caseCreateRead},
		{"cancellation", caseCancellation}, {"provider-error", caseProviderError}, {"cleanup", caseCleanup},
	} {
		fixture, fixtureErr := newFixture()
		if fixtureErr != nil { scenarioErr = fixtureErr; break }
		scenarioErr = runScenario(parent, t, h.Provider.Name, tc.name, adapter, fixture, cfg, tc.run)
		if scenarioErr != nil { break }
	}
	if scenarioErr == nil && caps[CapabilityTraversal].Enabled {
		fixture, fixtureErr := newFixture()
		if fixtureErr != nil { scenarioErr = fixtureErr } else {
			scenarioErr = runScenario(parent, t, h.Provider.Name, "traversal", adapter, fixture, cfg, caseTraversal)
		}
	}
	return scenarioErr
}

func runScenario(parent context.Context, t *testing.T, provider, name string, adapter Adapter, fixture Fixture, cfg Config, fn func(context.Context, Adapter, Fixture, Config) error) (returnErr error) {
	started := time.Now()
	defer func() {
		if recover() != nil { returnErr = errors.Join(returnErr, errors.New("graphtest: scenario panic")) }
		cleanupStarted := time.Now()
		cleanupResult := call(context.WithoutCancel(parent), cfg.CleanupTimeout, func(ctx context.Context) (struct{}, error) { return struct{}{}, adapter.CleanupFixture(ctx, fixture) })
		cleanupStatus, cleanupCategory, cleanupTimedOut := callbackStatus(cleanupResult)
		t.Logf("graphtest provider=%s phase=fixture-cleanup status=%s category=%s timeout=%t duration=%s", provider, cleanupStatus, cleanupCategory, cleanupTimedOut, time.Since(cleanupStarted))
		returnErr = errors.Join(returnErr, callbackError("fixture cleanup", cleanupResult))
		scenarioStatus := "ok"; if returnErr != nil { scenarioStatus = "error" }
		t.Logf("graphtest provider=%s phase=%s status=%s duration=%s", provider, name, scenarioStatus, time.Since(started))
	}()
	return fn(parent, adapter, fixture, cfg)
}

func bounded[T any](parent context.Context, timeout time.Duration, phase string, fn func(context.Context) (T, error)) (T, error) {
	result := call(parent, timeout, fn)
	if err := callbackError(phase, result); err != nil { var zero T; return zero, err }
	return result.value, nil
}
```

- [ ] **Step 5: core case functions 구현**

같은 파일에 `caseConnectivity`, `caseEmptyRead`, `caseCreateRead`, `caseCancellation`, `caseProviderError`, `caseCleanup`, `caseTraversal`을 추가한다. 각 callback은 정확히 한 번 호출하고 read/traversal은 slice length를 sort 전에 검사한다. `caseCreateRead`는 actual vertex의 `btgc_key`로 backend ID→logical key map을 만든 뒤 edge start/end를 logical key로 변환해 fixture edge와 비교한다. `caseProviderError`는 returned error를 출력하지 않고 classifier positive/nested-positive 검증과 error string secret marker 부재만 검사한다.

```go
func caseConnectivity(ctx context.Context, a Adapter, _ Fixture, cfg Config) error {
	_, err := bounded(ctx, cfg.CaseTimeout, "connectivity", func(ctx context.Context) (struct{}, error) { return struct{}{}, a.VerifyConnectivity(ctx) })
	return err
}

func caseEmptyRead(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	vertices, err := bounded(ctx, cfg.CaseTimeout, "empty vertices", func(ctx context.Context) ([]graph.Vertex, error) { return a.ReadVertices(ctx, f) })
	if err != nil { return err }
	if len(vertices) != 0 { return errors.New("graphtest: empty vertex read was not empty") }
	edges, err := bounded(ctx, cfg.CaseTimeout, "empty edges", func(ctx context.Context) ([]graph.Edge, error) { return a.ReadEdges(ctx, f) })
	if err != nil { return err }
	if len(edges) != 0 { return errors.New("graphtest: empty edge read was not empty") }
	return nil
}

func caseCancellation(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	canceled, cancel := context.WithCancel(ctx); cancel()
	_, err := bounded(canceled, cfg.CaseTimeout, "pre-canceled read", func(ctx context.Context) ([]graph.Vertex, error) { return a.ReadVertices(ctx, f) })
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return errors.New("graphtest: pre-canceled context was not preserved")
	}
	err = exerciseCancellation(ctx, a, f, cfg)
	if errors.Is(err, errCancellationReturnedBeforeStart) || errors.Is(err, errCancellationStartTimeout) || errors.Is(err, errCancellationDuplicateStart) {
		return err
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return errors.New("graphtest: in-flight cancellation was not preserved")
	}
	return nil
}

func caseProviderError(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	_, err := bounded(ctx, cfg.CaseTimeout, "provider error", func(ctx context.Context) (struct{}, error) { return struct{}{}, a.InvalidOperation(ctx, f) })
	if err == nil { return errors.New("graphtest: invalid operation returned nil") }
	return validatePositiveClassifier(a.IsProviderError, err)
}

func caseCreateRead(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	if _, err := bounded(ctx, cfg.CaseTimeout, "create fixture", func(ctx context.Context) (struct{}, error) { return struct{}{}, a.CreateFixture(ctx, f) }); err != nil { return err }
	vertices, err := bounded(ctx, cfg.CaseTimeout, "read vertices", func(ctx context.Context) ([]graph.Vertex, error) { return a.ReadVertices(ctx, f) })
	if err != nil { return err }
	actualVertices, err := canonicalVertices(vertices, cfg.MaxVertices)
	if err != nil { return err }
	expectedVertices, _ := canonicalVertices(f.Vertices(), cfg.MaxVertices)
	if len(actualVertices) != len(expectedVertices) { return errors.New("graphtest: vertex count mismatch") }
	actualIDs := make(map[string]string, len(actualVertices))
	for i := range actualVertices {
		actualKey, ok := logicalKey(actualVertices[i].Properties())
		if !ok { return errors.New("graphtest: vertex logical key missing") }
		expectedKey, _ := logicalKey(expectedVertices[i].Properties())
		if actualKey != expectedKey || actualVertices[i].Label() != expectedVertices[i].Label() ||
			!reflect.DeepEqual(actualVertices[i].Properties(), expectedVertices[i].Properties()) {
			return errors.New("graphtest: vertex semantic mismatch")
		}
		actualIDs[actualVertices[i].ID().String()] = actualKey
	}
	edges, err := bounded(ctx, cfg.CaseTimeout, "read edges", func(ctx context.Context) ([]graph.Edge, error) { return a.ReadEdges(ctx, f) })
	if err != nil { return err }
	actualEdges, err := canonicalEdges(edges, cfg.MaxEdges)
	if err != nil { return err }
	expectedEdges, _ := canonicalEdges(f.Edges(), cfg.MaxEdges)
	if len(actualEdges) != len(expectedEdges) { return errors.New("graphtest: edge count mismatch") }
	for i := range actualEdges {
		if actualEdges[i].Label() != expectedEdges[i].Label() || !reflect.DeepEqual(actualEdges[i].Properties(), expectedEdges[i].Properties()) ||
			actualIDs[actualEdges[i].StartID().String()] != expectedEdges[i].StartID().String() ||
			actualIDs[actualEdges[i].EndID().String()] != expectedEdges[i].EndID().String() {
			return errors.New("graphtest: edge semantic mismatch")
		}
	}
	return nil
}

func caseCleanup(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	if _, err := bounded(ctx, cfg.CaseTimeout, "create cleanup fixture", func(ctx context.Context) (struct{}, error) { return struct{}{}, a.CreateFixture(ctx, f) }); err != nil { return err }
	cleanup := call(context.WithoutCancel(ctx), cfg.CleanupTimeout, func(cleanupCtx context.Context) (struct{}, error) { return struct{}{}, a.CleanupFixture(cleanupCtx, f) })
	if err := callbackError("cleanup", cleanup); err != nil { return err }
	vertices, err := bounded(ctx, cfg.CaseTimeout, "cleanup read vertices", func(ctx context.Context) ([]graph.Vertex, error) { return a.ReadVertices(ctx, f) })
	if err != nil { return err }
	if len(vertices) != 0 { return errors.New("graphtest: cleanup left vertices") }
	edges, err := bounded(ctx, cfg.CaseTimeout, "cleanup read edges", func(ctx context.Context) ([]graph.Edge, error) { return a.ReadEdges(ctx, f) })
	if err != nil { return err }
	if len(edges) != 0 { return errors.New("graphtest: cleanup left edges") }
	return nil
}

func caseTraversal(ctx context.Context, a Adapter, f Fixture, cfg Config) error {
	if _, err := bounded(ctx, cfg.CaseTimeout, "create traversal fixture", func(ctx context.Context) (struct{}, error) { return struct{}{}, a.CreateFixture(ctx, f) }); err != nil { return err }
	keys, err := bounded(ctx, cfg.CaseTimeout, "traversal", func(ctx context.Context) ([]string, error) { return a.Traverse(ctx, f) })
	if err != nil { return err }
	if len(keys) > cfg.MaxTraversalResults { return errors.New("graphtest: traversal result limit exceeded") }
	if !slices.Equal(keys, []string{"left", "right"}) { return errors.New("graphtest: traversal semantic mismatch") }
	return nil
}
```

imports에 `reflect`, `slices`, `github.com/bluetape4k/bluetape-go/graph`를 포함한다. Error message에는 actual property map을 format하지 않는다.

- [ ] **Step 6: GREEN 확인**

Run: `gofmt -w graph/graphtest && go test -count=1 ./graph/graphtest -run '^TestRun'`

Expected: PASS; failure-case output에 `secret-marker`, query, URI, credential 문자열이 없음.

- [ ] **Step 7: Lore commit**

```bash
git add graph/graphtest/runner.go graph/graphtest/runner_test.go graph/graphtest/fake_test.go
git commit -m "strict graph scenario를 한 runner 순서로 실행한다" \
  -m "Constraint: core failure 뒤 후속 scenario는 중단하되 cleanup과 close는 계속한다" \
  -m "Rejected: provider 자유형 scenario | semantic 비교를 우회할 수 있다" \
  -m "Confidence: medium" -m "Scope-risk: broad" \
  -m "Directive: callback별 query submission은 한 번이고 결과 상한은 materialization 전에 적용한다" \
  -m "Tested: go test -count=1 ./graph/graphtest -run ^TestRun" -m "Not-tested: Docker backend parity"
```

### Task 6: fake harness self-test와 compile-checked example 완성

**Files:**
- Create: `graph/graphtest/example_test.go`
- Modify: `graph/graphtest/runner_test.go`
- Modify: `graph/graphtest/lifecycle_test.go`

- [ ] **Step 1: self-test matrix 추가**

`runner_test.go`와 `lifecycle_test.go`에 다음 named rows를 각각 독립 subtest로 추가한다.

- nil factory와 각 nil core callback/classifier
- `Capabilities` nil/empty/missing/unknown, enabled-with-nil callback, disabled-with-callback, unsafe reason
- factory error/panic, callback error/panic, cleanup error/panic, close error/panic
- classifier always-true/always-false/panic과 nested provider error
- pre-canceled context, pre-canceled callback이 context를 무시하는 subprocess fail-stop, missing/duplicate/early `Started`, signal-timeout cancel 뒤 join
- `MaxVertices+1`, `MaxEdges+1`, `MaxTraversalResults+1`이 sort 전에 실패
- callback별 query counter가 한 invocation에서 정확히 1. Cleanup core scenario는 idempotence를 증명하기 위해 `caseCleanup`과 runner finalizer가 같은 fixture를 순서대로 두 번 호출하되, 각 invocation은 query submission 1회이고 나머지 scenario의 cleanup은 1회다.
- `caseCleanup`의 cleanup panic/error/timeout도 `call`과 `callbackError("cleanup", ...)` 경계에서 raw cause 없이 named error로 변환된다.
- event-trace adapter로 실제 `run(...)`을 호출해 `started → joined → fixture-cleanup → close` 순서와 scenario panic/partial-create error 뒤 cleanup·close를 검증한다.
- cleanup idempotence는 empty, partial-create, complete, already-cleaned 네 상태를 분리하고 invocation별 logical query submission 1회를 확인한다.
- capability map과 fixture accessor 반환 slice를 호출 중 변형해도 실행 기준 복사본은 불변
- `ProviderMetadata.Name`/`Version`의 credential-like marker가 factory 전에 거절되고, secret marker가 factory/connectivity/operation/cleanup/close failure string과 captured test output에 없음
- core failure 뒤 다음 core와 traversal은 호출되지 않지만 cleanup/close는 호출됨
- `RunWithConfig`는 `t.Context()`를 parent로 사용한다. Pre-canceled parent는 factory callback을 0회 호출하고 context error를 보존하며, live parent deadline은 factory operation context에 전파된다. Cleanup/close는 `context.WithoutCancel`에서 새 상한을 받는다.

각 row는 `run(context.Background(), t, harness, cfg)`의 error category와 counter를 직접 비교한다. Raw error의 `Error()`를 assertion message에 출력하지 말고 `errors.Is/As`와 고정 category만 검사한다.

- [ ] **Step 2: external package compile example와 executable example test 작성**

`graph/graphtest/example_test.go`:

```go
package graphtest_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphtest"
)

var errExampleProvider = errors.New("example provider")

type exampleState struct {
	mu sync.Mutex
	vertices []graph.Vertex
	edges []graph.Edge
}

func ExampleRun() {
	_ = exampleHarness() // TestBackend(t)에서 graphtest.Run(t, exampleHarness())로 호출한다.
}

func ExampleRunWithConfig() {
	cfg := graphtest.DefaultConfig()
	cfg.MaxVertices = graphtest.MaxResultLimit
	_ = cfg // TestBackend(t)에서 graphtest.RunWithConfig(t, exampleHarness(), cfg)로 호출한다.
}

func ExampleCapabilities() {
	caps := graphtest.Capabilities{
		graphtest.CapabilityTraversal: {Enabled: false, ReasonCode: "query-language-limit"},
	}
	_ = caps
}

func exampleHarness() graphtest.Harness {
	return graphtest.Harness{
		Provider: graphtest.ProviderMetadata{Name: "example", Version: "1.0.0", ImageReference: "example:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		New: func(context.Context, testing.TB, graphtest.Config) (graphtest.Adapter, error) {
			state := &exampleState{}
			return graphtest.Adapter{
				VerifyConnectivity: func(context.Context) error { return nil },
				CreateFixture: func(_ context.Context, fixture graphtest.Fixture) error {
					state.mu.Lock(); defer state.mu.Unlock()
					state.vertices, state.edges = fixture.Vertices(), fixture.Edges()
					return nil
				},
				ReadVertices: func(context.Context, graphtest.Fixture) ([]graph.Vertex, error) {
					state.mu.Lock(); defer state.mu.Unlock()
					return append([]graph.Vertex(nil), state.vertices...), nil
				},
				ReadEdges: func(context.Context, graphtest.Fixture) ([]graph.Edge, error) {
					state.mu.Lock(); defer state.mu.Unlock()
					return append([]graph.Edge(nil), state.edges...), nil
				},
				InvalidOperation: func(context.Context, graphtest.Fixture) error { return fmt.Errorf("example operation: %w", errExampleProvider) },
				BlockUntilCanceled: func(ctx context.Context, _ graphtest.Fixture, started graphtest.Started) error { started(); <-ctx.Done(); return ctx.Err() },
				CleanupFixture: func(context.Context, graphtest.Fixture) error {
					state.mu.Lock(); defer state.mu.Unlock()
					state.vertices, state.edges = nil, nil
					return nil
				},
				Close: func(context.Context) error { return nil },
				IsProviderError: func(err error) bool { return errors.Is(err, errExampleProvider) },
			}, nil
		},
		Capabilities: graphtest.Capabilities{graphtest.CapabilityTraversal: {Enabled: false, ReasonCode: "query-language-limit"}},
	}
}

func TestExampleHarnessConforms(t *testing.T) {
	graphtest.Run(t, exampleHarness())
}
```

`ExampleRun`과 `ExampleRunWithConfig`는 testing helper의 호출 위치를 보여주는 compile example이고, 같은 external package의 `TestExampleHarnessConforms`가 동일한 stateful adapter를 실제로 실행한다. README에는 `func TestBackend(t *testing.T)` 전체를 복사 가능한 예제로 싣는다. Example classifier는 wrapped sentinel을 `errors.Is`로 분류한다.

- [ ] **Step 3: fake suite 전체 GREEN**

Run: `gofmt -w graph/graphtest && go test -race -count=10 ./graph/graphtest`

Expected: 10회 PASS, race 0건, Docker 접근 0건.

- [ ] **Step 4: Lore commit**

```bash
git add graph/graphtest/example_test.go graph/graphtest/runner_test.go graph/graphtest/lifecycle_test.go
git commit -m "외부 service 없이 conformance runner 실패 경계를 검증한다" \
  -m "Constraint: fake suite는 Docker 없이 항상 실행된다" \
  -m "Rejected: 공개 MemoryHarness | 실제 backend compatibility를 대신할 수 있다" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: failure assertion에서 raw provider payload를 출력하지 않는다" \
  -m "Tested: go test -race -count=10 ./graph/graphtest" -m "Not-tested: provider image readiness"
```

### Task 7: Neo4j와 Memgraph shared conformance 연결

**Files:**
- Create: `graph/neo4j/conformance_test.go`
- Read only: `graph/provider_benchmark_test.go`
- Test: `graph/neo4j/conformance_test.go`

- [ ] **Step 1: 두 provider shared suite RED test 작성**

`graph/neo4j/conformance_test.go`의 top-level test부터 만든다. `t.Parallel`은 사용하지 않는다.

```go
package neo4j_test

import (
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph/graphtest"
)

func TestBackendConformance(t *testing.T) {
	started := time.Now()
	for _, tc := range []struct{name string; harness graphtest.Harness}{
		{"neo4j", neo4jConformanceHarness()},
		{"memgraph", memgraphConformanceHarness()},
	} {
		if !t.Run(tc.name, func(t *testing.T) { graphtest.Run(t, tc.harness) }) { return }
	}
	if elapsed := time.Since(started); elapsed > 10*time.Minute {
		t.Fatalf("backend conformance elapsed=%s, want <=10m", elapsed)
	}
}
```

- [ ] **Step 2: RED 확인**

Run: `go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -timeout=10m`

Expected: FAIL with undefined `neo4jConformanceHarness` and `memgraphConformanceHarness`; container는 아직 시작하지 않음.

- [ ] **Step 3: digest와 fixed query surface 고정**

같은 file에 benchmark에서 확인한 immutable references와 query constants를 그대로 선언한다.

```go
const (
	neo4jConformanceImage = "neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5"
	memgraphConformanceImage = "memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1"
	vertexColumn = "n"
	edgeColumn = "r"
	traversalColumn = "keys"
	cancellationColumn = "total"
	cancellationIterations = 1_000_000
	createFixtureQuery = `UNWIND $vertices AS v CREATE (n:BTGraphConformance) SET n = v WITH count(n) AS ignored UNWIND $edges AS e MATCH (a:BTGraphConformance {namespace: e.namespace, btgc_key: e.start}), (b:BTGraphConformance {namespace: e.namespace, btgc_key: e.end}) CREATE (a)-[r:BTGC_LINKS]->(b) SET r = e.props`
	readVerticesQuery = `MATCH (n:BTGraphConformance {namespace: $namespace}) RETURN n LIMIT $limit`
	readEdgesQuery = `MATCH (:BTGraphConformance {namespace: $namespace})-[r:BTGC_LINKS]->(:BTGraphConformance {namespace: $namespace}) RETURN r LIMIT $limit`
	traverseQuery = `MATCH p=(a:BTGraphConformance {namespace: $namespace, btgc_key: $start})-[:BTGC_LINKS]->(b:BTGraphConformance {namespace: $namespace}) RETURN [n IN nodes(p) | n.btgc_key] AS keys LIMIT $limit`
	cancellationQuery = `UNWIND range(1, $iterations) AS i WITH sum(i) AS total RETURN total`
	cleanupQuery = `MATCH (n:BTGraphConformance {namespace: $namespace}) DETACH DELETE n`
	invalidQuery = `RETURN $missing +`
)
```

모든 fixture 값은 parameter로 bind한다. Column은 `vertexColumn`/`edgeColumn`/`traversalColumn`/`cancellationColumn`의 고정 집합만 사용한다. 각 read/traverse request-builder는 `$limit`을 대응 config 값 `+1`로 설정하며 `MaxResultLimit+1` overflow가 없음을 config validation이 보장한다. `TestConformanceReadRequestBuilders`와 `TestConformanceTraversalRequestBuilder`는 각 `MaxVertices`/`MaxEdges`/`MaxTraversalResults`의 기본값·경계값·`MaxResultLimit`에서 생성된 fixed query, namespace, fixed column, bound `limit=config+1`을 캡처하고 application-level logical submission이 정확히 1인지 확인한다. `TestConformancePreCanceledRequestSubmitsNoQuery`는 pre-canceled callback의 logical submission이 0임을 검증한다. Neo4j driver-managed transaction retry는 Bolt wire attempt를 늘릴 수 있으므로 logical callback/submission count와 구분해 duration/log evidence에 기록하며, retry가 발생한 run은 deterministic first-attempt conformance evidence로 사용하지 않는다. Oversized fake result와 세 request-builder test가 모두 녹색이 되기 전에는 Docker suite를 시작하지 않는다.

- [ ] **Step 4: provider factory와 partial cleanup 구현**

`newNeo4jAdapterFactory(image, version, start)` helper를 작성한다. `start`는 provider별 container 시작과 observed version 확인을 담당한다. Container 시작 함수가 `(container, err)`를 반환하면 `err` 검사보다 먼저 non-nil container를 redacting terminator로 감싸 `testcleanup.Register(startupCtx, tb, provider, wrappedContainer)`에 등록한다. 따라서 partial container를 동반한 startup 실패도 30초 상한 안에서 종료된다. Factory와 start helper는 `tb.Fatal`/`tb.Fatalf`, `testcleanup.FormatStartError`, raw `tb.Logf`를 호출하지 않고 sanitized error만 runner에 반환한다. Factory body는 named return과 `defer` panic guard를 사용해 partial driver를 fresh close context로 닫은 뒤 panic을 다시 전달하고, runner boundary가 이를 sanitized factory panic으로 바꾼다. Driver 생성 또는 readiness 실패도 아래 pattern으로 partial client를 정리하고 original error의 `errors.Is/As` chain만 보존한다.

```go
type sanitizedProviderError struct { phase string; cause error }

var errProviderCallbackPanic = errors.New("graph/neo4j: provider callback panic")

func (e *sanitizedProviderError) Error() string { return "graph/neo4j: " + e.phase + " failed" }
func (e *sanitizedProviderError) Unwrap() error { return e.cause }

func sanitizeProviderError(phase string, err error) error {
	if err == nil { return nil }
	return &sanitizedProviderError{phase: phase, cause: err}
}

type redactingTerminator struct { container testcleanup.Terminator }

func (r redactingTerminator) Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) (returnErr error) {
	defer func() {
		if recover() != nil { returnErr = errProviderCallbackPanic }
	}()
	return sanitizeProviderError("container termination", r.container.Terminate(ctx, opts...))
}

func closePartial(startupCtx context.Context, timeout time.Duration, driver neo4jdriver.Driver, original error) error {
	startupErr := sanitizeProviderError("startup", original)
	if driver == nil { return startupErr }
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(startupCtx), timeout)
	defer cancel()
	return errors.Join(startupErr, sanitizeProviderError("partial driver close", driver.Close(closeCtx)))
}
```

Neo4j는 `tcneo4j.Run`에 `neo4jConformanceImage`를 전달한다. Memgraph `GenericContainerRequest.ContainerRequest.Image`는 반드시 `memgraphConformanceImage`이며 mutable tag fallback을 금지하고 `wait.ForListeningPort("7687/tcp")`를 사용한다. 두 start helper는 전달받은 image가 provider별 digest constant와 정확히 같은지 factory resource 생성 전에 검사한다. Driver readiness는 startup deadline 안에서 250ms interval, 개별 attempt 2s로 retry한다. Metadata의 `Name`과 `Version`은 safe identifier validation을 통과한 값만 로그에 남기고, `Version`은 query로 얻은 observed major/minor와 정확히 맞춘다. Mismatch는 factory failure로 닫는다. Unit/subprocess test는 partial container+startup error, driver 생성 뒤 readiness error, partial close error, termination error/panic에 secret marker를 넣고 `fmt.Sprint(err)`와 captured test output 모두 marker/query/URI/credential을 포함하지 않으면서 error 반환 경로의 `errors.Is/As` chain이 유지되는지 검증한다. Panic payload는 원인 chain에 넣지 않고 고정 `errProviderCallbackPanic`으로 바꾼다.

- [ ] **Step 5: adapter closure 구현**

Factory가 반환하는 adapter는 기존 `neo4j.Client`를 재사용한다. `CreateFixture`, read, cleanup은 각각 한 client query submission만 수행한다. Traversal은 driver read transaction에서 `keys` fixed column만 읽는다. Cancellation callback은 transaction의 `Run`이 성공해 blocking result를 얻은 뒤 `Started`를 호출하고 `Consume(ctx)`를 기다린다.

```go
BlockUntilCanceled: func(ctx context.Context, _ graphtest.Fixture, started graphtest.Started) (returnErr error) {
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.CloseTimeout)
		defer cancel()
		returnErr = errors.Join(returnErr, sanitizeProviderError("session close", session.Close(closeCtx)))
	}()
	_, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cancellationQuery, map[string]any{"iterations": cancellationIterations})
		if err != nil { return nil, err }
		started()
		_, err = result.Consume(ctx)
		return nil, err
	})
	if err != nil { returnErr = sanitizeProviderError("blocking operation", err) }
	return returnErr
},
IsProviderError: func(err error) bool { return errors.Is(err, neo4jadapter.ErrDriver) },
```

`BlockUntilCanceled` 함수 literal은 named `returnErr`를 반환하도록 선언한다. Provider-facing wrapped error의 `Error()`에는 query와 params를 넣지 않는다. Cancellation query는 parameterized finite work와 단일 aggregate row로 제한하며 unit test는 `cancellationIterations <= 1_000_000`, 고정 column, parameter binding을 확인한다. `Started` 직후 nil 또는 `context.Canceled`를 즉시 반환하는 fake는 실제 cancel 관찰 없이 성공할 수 없도록 negative lifecycle matrix에 둔다. Provider test는 `Started → cancel → Consume(ctx)` event와 duration을 기록한다. Adapter close만 driver/client를 exactly once 닫고 container termination은 `t.Cleanup`이 `Run` 반환 뒤 수행한다.

- [ ] **Step 6: targeted provider GREEN과 timing evidence**

사전 확인: `colima status && docker context show && docker info >/dev/null`.

Run: `go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -v -timeout=10m`

Expected: Neo4j와 Memgraph named core 및 traversal PASS, 총 10분 이내, cleanup → close → container terminate 순서. 출력에는 provider/name/version, phase, duration만 있고 URI/credential/query/parameters 없음.

- [ ] **Step 7: process별 10분 상한으로 반복 검증**

Run:

```bash
set -o pipefail
for attempt in 1 2 3; do
  go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -v -timeout=10m \
    2>&1 | tee "/tmp/issue-555-conformance-${attempt}.log" || exit 1
done
```

Expected: 독립 process 3회가 각각 10분 이내 PASS하고 첫 실패에서 즉시 중단한다. 실패 시 broad retry하지 않고 `go test -count=1 ./graph/neo4j -run '^TestBackendConformance/(neo4j|memgraph)$' -v -timeout=10m`으로 실패 provider만 1회 재현하며 attempt와 duration을 보존한다.

- [ ] **Step 8: Lore commit**

```bash
git add graph/neo4j/conformance_test.go
git commit -m "Neo4j와 Memgraph가 같은 graph 계약을 증명하게 한다" \
  -m "Constraint: provider는 digest 고정 container와 caller-owned driver lifecycle을 사용한다" \
  -m "Rejected: managed graph service CI | credential과 외부 가용성을 correctness에 결합한다" \
  -m "Confidence: medium" -m "Scope-risk: broad" \
  -m "Directive: query는 고정 문자열과 bound parameter만 사용하고 limit plus one을 유지한다" \
  -m "Tested: three isolated go test -count=1 ./graph/neo4j -run ^TestBackendConformance$ -timeout=10m runs" \
  -m "Not-tested: FalkorDB와 AGE"
```

### Task 8: old/new parity 뒤 중복 integration body 제거

**Files:**
- Modify: `graph/neo4j/client_test.go`
- Modify: `graph/neo4j/memgraph_test.go`
- Test: `graph/neo4j/convert_test.go`
- Test: `graph/neo4j/conformance_test.go`

- [ ] **Step 1: old suite와 shared suite를 같은 tree에서 실행**

Run: `go test -count=1 ./graph/neo4j -run 'Test(ClientReadWriteFailureCancellationAndCleanupWithTestcontainersNeo4j|ClientMemgraphCompatibilityWithGenericContainer|BackendConformance)$' -v -timeout=15m`

Expected: 기존 두 integration test와 shared Neo4j/Memgraph suite 모두 PASS. Vertex label/properties, edge type/direction/properties, cancellation, provider error, cleanup이 양쪽에서 일치.

- [ ] **Step 2: pure unit test 보호선 확인**

Run: `go test -count=1 ./graph/neo4j -run 'Test(ClientRejectsInvalidInputs|VertexFromNode|EdgeFromRelationship|VerticesFromRecords|EdgesFromRecords)'`

Expected: PASS. 이 이름에 해당하는 constructor/conversion test는 삭제하지 않는다.

- [ ] **Step 3: 겹치는 old integration body만 삭제**

삭제 전에 다음 참조 audit을 실행한다.

```bash
rg -n 'startNeo4jDriver|startMemgraphDriver|waitForMemgraphConnectivity' graph/neo4j
rg -n 'TestClientReadWriteFailureCancellationAndCleanupWithTestcontainersNeo4j|TestClientMemgraphCompatibilityWithGenericContainer' graph/neo4j
```

Expected: 각 helper 참조가 제거 대상 old integration test와 helper 정의에만 있다. 다른 test 또는 production file 참조가 있으면 삭제를 중단하고 risk artifact에 `BLOCKED`로 기록한다.

`graph/neo4j/client_test.go`에서 `TestClientReadWriteFailureCancellationAndCleanupWithTestcontainersNeo4j`와 그 test만 사용하는 `startNeo4jDriver`를 제거한다. `graph/neo4j/memgraph_test.go`에서 `TestClientMemgraphCompatibilityWithGenericContainer`, `startMemgraphDriver`, `waitForMemgraphConnectivity`와 그 test만 쓰는 constants/imports를 제거한다. `TestClientRejectsInvalidInputs`, `convert_test.go`, 새 `conformance_test.go`는 유지한다. Provider별 parity evidence는 vertex label/properties, edge type/direction/properties, cancellation, provider error classification, cleanup readback, conversion unit 보존의 여섯 row로 WIP에 기록한다.

- [ ] **Step 4: migration GREEN 확인**

Run: `go test -count=1 ./graph/neo4j -timeout=10m`

Expected: PASS; Neo4j/Memgraph shared suite는 각각 한 번 실행되고 legacy duplicate container는 생성되지 않음.

- [ ] **Step 5: rollback 단위를 분리한 Lore commit**

```bash
git add graph/neo4j/client_test.go graph/neo4j/memgraph_test.go
git commit -m "shared suite와 겹치는 backend scenario 중복을 제거한다" \
  -m "Constraint: old와 new suite의 side by side parity를 먼저 확인했다" \
  -m "Rejected: conversion unit test 제거 | provider conformance와 목적이 다르다" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Directive: 회귀 시 이 제거 commit만 revert하고 shared runner 수정부터 진단한다" \
  -m "Tested: go test -count=1 ./graph/neo4j -timeout=10m" -m "Not-tested: exact-head GitHub CI"
git rev-parse HEAD
git show --stat --oneline HEAD
```

Expected: `git rev-parse HEAD`가 Task 8 제거 commit의 40-hex SHA를 출력하고 `git show`가 두 legacy integration body 제거만 보여 준다. Task 9에서 이 정확한 SHA를 `WIP.md`의 `Migration commit` 항목에, Task 10에서 risk prediction 문서의 old/new parity row에 기록한다.

Rollback: migration 뒤 regress가 발생하면 `WIP.md`에 기록한 exact SHA를 읽고 대상과 경계를 확인한 뒤 제거 commit만 revert한다.

```bash
migration_commit=$(sed -n 's/^- Migration commit: `\([0-9a-f]\{40\}\)`.*/\1/p' WIP.md)
test -n "$migration_commit"
git merge-base --is-ancestor "$migration_commit" HEAD
test "$(git rev-parse "$migration_commit")" = "$migration_commit"
git show --stat --oneline "$migration_commit"
test "$(git diff --name-only "$migration_commit^" "$migration_commit" | sort)" = $'graph/neo4j/client_test.go\ngraph/neo4j/memgraph_test.go'
git revert --no-edit "$migration_commit"
go test -count=1 ./graph/neo4j -timeout=10m
git status --short
```

이 복구는 old/new 병행 상태로만 돌아가며 Harness와 provider adapter commit은 보존해 원인을 좁힌다.

### Task 9: README, release 기록과 최종 gate 동기화

**Files:**
- Create: `graph/graphtest/README.md`
- Create: `graph/graphtest/README.ko.md`
- Modify: `graph/README.md`
- Modify: `graph/README.ko.md`
- Modify: `graph/neo4j/README.md`
- Modify: `graph/neo4j/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `WIP.md`

- [ ] **Step 1: package README locale pair 작성**

두 README에 같은 의미로 다음 내용을 기록한다.

- `graph/graphtest`는 test-support이고 production repository/query abstraction이 아님
- `Run` 기본 config와 `RunWithConfig` strict complete config
- 필수 core callback은 skip 불가, optional traversal 미지원은 safe `ReasonCode` 필수
- Provider가 container/credential/readiness를 소유하고 adapter만 client/driver를 닫음
- `limit+1`, fixed query/column, bound parameter, error redaction 요구
- lifecycle: fixture cleanup → adapter close → Run 반환 → container terminate
- 새 backend 참여 예제는 compile-checked `ExampleRun`, `ExampleCapabilities`와 일치

- [ ] **Step 2: graph/neo4j/root locale pair 갱신**

`graph` README에는 production model-only 경계와 별도 `graph/graphtest` 링크를 추가한다. `graph/neo4j` README에는 Neo4j와 Memgraph가 digest-pinned shared suite를 실행한다고 기록한다. Root package table에는 다음 row를 locale별 문장으로 추가한다.

```markdown
| [`graph/graphtest`](graph/graphtest/README.md) | test | Strict backend conformance harness for graph semantics, cancellation, bounded cleanup, traversal capabilities, and redacted provider errors. |
```

- [ ] **Step 3: CHANGELOG와 WIP 갱신**

`CHANGELOG.md`의 `[Unreleased]` 아래 `### 추가`에 `graph/graphtest` public harness와 Neo4j/Memgraph shared suite를 한국어로 기록한다. `WIP.md`는 0.22.0 Issue #555의 구현/targeted/race/local Testcontainers/CI 상태를 실제 evidence만으로 갱신하고, exact-head Nightly와 Step 6-R은 실행 전까지 미완료로 남긴다.

CHANGELOG 항목은 다음 문장으로 고정한다.

```markdown
### 추가

- `graph/graphtest`에 backend-neutral semantic fixture, skip 불가 strict core,
  traversal capability, cancellation join, bounded cleanup/close와 redacted
  provider error 검증을 제공하는 공개 conformance harness를 추가한다.
- Neo4j와 Memgraph adapter가 digest-pinned Testcontainers 환경에서 같은
  core 및 traversal suite를 실행하도록 통합한다.
```

WIP에는 아래 evidence checklist를 실제 결과와 함께 넣는다.

```markdown
## Issue #555 graph backend conformance

- `[ ]` `go test -race -count=1 ./graph/graphtest`
- `[ ]` 독립 process 3회 `go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -v -timeout=10m`
- `[ ]` `make ci`
- `[ ]` exact-head Testcontainers Nightly
- `[ ]` Step 6-R 7-Tier review `P0=0 P1=0`
- Migration commit: `PENDING`
- HEAD_SHA: `PENDING`
- Base/head: `develop` / `feat/issue-555-graph-conformance`
- PR number/URL: `PENDING`
- Required CI run IDs/URLs/conclusions/observed timestamp: `PENDING`
- Testcontainers Nightly run ID/URL/headSha/conclusion/observed timestamp: `PENDING`
```

Migration commit은 Task 8 직후 `- Migration commit: ` 뒤에 backtick으로 감싼 exact 40-hex SHA 형식으로 교체한다. 나머지 `PENDING` 값은 해당 별도 authority gate가 실행된 직후 exact observed value로 교체한다. PR이나 workflow가 아직 생성되지 않은 단계에서는 임의 번호나 예상 URL을 기록하지 않는다.

- [ ] **Step 4: 문서, link, locale와 example GREEN**

Run sequentially:

```bash
go test -count=1 ./graph/graphtest -run '^(Example|TestExampleHarnessConforms$)'
test -f graph/graphtest/README.md
test -f graph/graphtest/README.ko.md
test -f graph/neo4j/README.md
test -f graph/neo4j/README.ko.md
rg -n 'graph/graphtest|RunWithConfig|limit\+1|cleanup|close|ReasonCode' README.md README.ko.md graph/README.md graph/README.ko.md graph/neo4j/README.md graph/neo4j/README.ko.md graph/graphtest/README.md graph/graphtest/README.ko.md
git diff --check
```

Expected: example와 executable example test PASS, 여덟 locale 문서에서 필수 계약 hit가 모두 존재하고 known relative file target이 존재하며 trailing whitespace 0건. 자동 번역 동등성 helper가 없으므로 writer SPW/KO review가 English/Korean heading·목록·code block·link target의 의미 대응을 line-by-line 기록한다.

- [ ] **Step 5: Lore commit**

```bash
git add graph/graphtest/README.md graph/graphtest/README.ko.md graph/README.md graph/README.ko.md graph/neo4j/README.md graph/neo4j/README.ko.md README.md README.ko.md CHANGELOG.md WIP.md
git commit -m "새 graph backend가 따라야 할 conformance 경계를 공개한다" \
  -m "Constraint: 독자 문서는 한국어 release 기록과 locale 의미를 동기화한다" \
  -m "Rejected: lifecycle diagram 추가 | runner 순서가 짧아 문서만으로 충분하다" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: exact head Nightly 전에는 release evidence를 완료로 표시하지 않는다" \
  -m "Tested: go test -count=1 ./graph/graphtest -run ^Example && git diff --check" \
  -m "Not-tested: exact-head GitHub CI와 Testcontainers Nightly"
```

### Task 10: canonical verification과 Step 3-P 위험 관찰

**Files:**
- Verify only: all task-owned files
- Modify: `docs/review/2026-09-06-issue-555-risk-prediction.md`

- [ ] **Step 1: lightweight gate**

Run sequentially:

```bash
make fmt-check
make tidy-check
make vet
make lint
go test -race -count=1 ./graph/graphtest
```

Expected: 모두 exit 0, `go.mod`/`go.sum` 무변경, race 0건.

- [ ] **Step 2: Docker-backed targeted gate**

Run sequentially, 다른 Testcontainers process와 겹치지 않게 한다.

```bash
go test -count=1 ./graph/neo4j -timeout=10m
set -o pipefail
for attempt in 1 2 3; do
  go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -v -timeout=10m \
    2>&1 | tee "/tmp/issue-555-final-conformance-${attempt}.log" || exit 1
done
go test -race -count=1 ./graph/neo4j -timeout=15m
```

Expected: Neo4j/Memgraph 모두 PASS, total default suite budget 10분 이내, cleanup evidence 존재.

- [ ] **Step 3: canonical repository gate**

Run: `make test && make ci`

Expected: 모두 exit 0. 로컬 2 CPU/4 GiB Colima에서 다른 Testcontainers package가 간헐 실패하면 실패를 PASS로 바꾸지 않는다. 정확한 package/test/error를 보존하고 해당 test를 한 process로 1회 targeted rerun한 뒤 exact-head CI/Nightly에 PENDING evidence로 남긴다.

- [ ] **Step 4: Step 3-P risk prediction에 실제 관찰 결과 추가**

Task 0에서 source 편집 전에 생성한 위험 row를 보존하고 다음 실제 신호, command output 위치, disposition을 PR DoD 및 Step 6-R 입력에 연결한다.

| 위험 | 조기 신호 | 완화/중단 기준 |
| --- | --- | --- |
| callback이 context를 무시해 hang | case timeout 뒤 goroutine 미종료 | cleanup/close를 시작하지 않고 outer `go test -timeout` fail-stop; bounded PASS 금지 |
| cleanup과 close가 callback과 경합 | event trace에서 join 전 cleanup | fake order test 실패 시 provider test 중단 |
| query가 전체 graph를 materialize | adapter query에 namespace/`limit+1` 누락 | static review와 oversized fake test 통과 전 Docker suite 금지 |
| query/credential/fixture secret 노출 | failure output에 marker, URI, query 포함 | 모든 phase redaction test를 blocker로 처리 |
| image/readiness drift | digest mismatch, observed version mismatch, startup timeout | broad retry 없이 provider별 targeted rerun; immutable reference 수정은 별도 review |
| old test 제거로 coverage 손실 | side-by-side parity 또는 conversion test 실패 | Task 8 제거 commit만 revert |
| shared resource flake | provider별 duration 증가, port/container cleanup 실패 | Testcontainers test 직렬화, attempt/duration 보존, Nightly 근거 요구 |

old/new parity row에는 Task 8 직후 얻은 exact migration SHA와 `git show --stat --oneline` 결과를 기록한다. SHA를 path 기반 `git log -1`로 다시 추정하지 않는다.

- [ ] **Step 5: PR 생성 뒤 exact-head CI evidence 계약 확인**

PR 생성은 별도 승인 gate다. 승인된 PR이 생긴 뒤 다음 조회를 실행하고 WIP/PR DoD에 exact value와 관찰 timestamp를 기록한다.

```bash
head_sha=$(git rev-parse HEAD)
pr_number=$(gh pr view --json number --jq '.number')
gh pr view "$pr_number" --json number,url,baseRefName,headRefName,headRefOid,mergeable,reviewDecision,statusCheckRollup,updatedAt
gh pr checks "$pr_number" --json name,state,link,workflow,bucket
```

Expected: base=`develop`, head=`feat/issue-555-graph-conformance`, `headRefOid`=`$head_sha`, required checks terminal success. PR number/URL, 각 required CI run URL·상태와 조회 timestamp를 보존한다. 다른 SHA, skipped required job, 진행 중 job은 PASS가 아니다.

- [ ] **Step 6: exact-head Testcontainers Nightly evidence 계약 확인**

Workflow dispatch 자체는 구현 PR의 별도 승인 gate 뒤에만 수행한다. 승인 후 다음 순서로 exact head와 scope를 고정한다.

```bash
head_sha=$(git rev-parse HEAD)
head_ref=$(git branch --show-current)
gh workflow run nightly-tests.yml --ref "$head_ref" -f scope=testcontainers
run_id=""
for poll in {1..12}; do
  run_id=$(gh run list --workflow nightly-tests.yml --branch "$head_ref" --commit "$head_sha" --event workflow_dispatch --limit 10 --json databaseId,headSha | jq -r --arg head "$head_sha" 'map(select(.headSha == $head))[0].databaseId // empty')
  test -n "$run_id" && break
  sleep 5
done
test -n "$run_id"
nightly_status=0
gh run watch "$run_id" --exit-status || nightly_status=$?
gh run view "$run_id" --json url,headSha,status,conclusion,jobs,createdAt,updatedAt
gh run view "$run_id" --log > /tmp/issue-555-nightly.log
gh run download "$run_id" --name nightly-coverage-go --dir /tmp/issue-555-nightly-artifacts
test "$nightly_status" -eq 0
```

Expected: 최대 60초 안에 exact run ID를 찾고 `headSha`가 `$head_sha`와 정확히 같으며 testcontainers scope의 required job이 terminal success이고 run이 cancelled가 아니다. `make coverage` 또는 `make race`가 workflow 내부 retry 뒤 성공했다면 headline green이어도 release evidence는 `PENDING`이다. 모든 attempt와 첫 오류는 `/tmp/issue-555-nightly.log`와 WIP/PR DoD에 보존한다. `nightly-coverage-go` artifact는 최종 attempt 산출물일 뿐 첫 실패 증거가 아님을 명시하고, release proof에는 first-attempt green 또는 별도 no-retry exact-head conformance run이 필요하다.

- [ ] **Step 7: diff와 scope audit**

Run:

```bash
git diff --check origin/develop...HEAD
git diff --name-only origin/develop...HEAD
git ls-files --error-unmatch docs/superpowers/plans/2026-09-06-issue-555-graph-backend-conformance-plan.md
git status --short
```

Expected: 위 쓰기 범위 밖 code 변경 0건, dependency 변경 0건, uncommitted file 0건.

## 검증 후 stop condition

- Fake harness, race, 두 provider conformance, parity migration, locale docs와 canonical local gate가 녹색이다.
- Exact-head GitHub CI와 Testcontainers Nightly, Step 6-R 7-Tier review가 완료되기 전에는 Issue #555와 PR DoD를 `DONE`으로 표시하지 않는다.
- P0/P1 review finding, redaction leak, non-cooperative bounded PASS, cleanup/close overlap, provider skip, result materialization 상한 위반 중 하나라도 남으면 구현 완료를 선언하지 않는다.
- Release tag/publication과 milestone closure는 Issue #555 범위 밖이며 0.22.0 별도 release gate까지 실행하지 않는다.

## Spec coverage self-review

| 승인 spec 요구 | 구현 task | 검증 evidence |
| --- | --- | --- |
| Config maximum, strict zero/partial rejection | Task 1 | config boundary table, `ExampleRunWithConfig` |
| Provider metadata cardinality, immutable image, pre-factory rejection | Task 1 | unsafe metadata table, factory call counter |
| Semantic fixture, crypto namespace, defensive clone | Task 2 | fixture clone/namespace tests |
| Required callbacks, classifier probes, capability 실행 기준 복사 | Task 3 | validation matrix와 race |
| Timeout, panic recovery, Started handshake, join-before-cleanup | Task 4 | lifecycle order, subprocess fail-stop, race |
| Strict core, bounded result, redacted error, optional traversal | Task 5–6 | fake suite 10회와 secret scan |
| Digest-pinned Neo4j/Memgraph, fixed query, `limit+1`, testcleanup | Task 7 | provider targeted 3회와 timing logs |
| old/new parity와 conversion test 보존 | Task 8 | side-by-side 및 post-removal test |
| README locale pair, root table, CHANGELOG, WIP | Task 9 | examples와 `git diff --check` |
| canonical local gate, exact-head/Nightly/review stop rule | Task 10 | command exit status와 PENDING evidence |

Self-review 결과: 승인 spec의 목표·비목표·공개 API·fixture·lifecycle·core·capability·보안·migration·문서·DoD가 모두 task에 연결된다. Identifier는 `DefaultConfig`, `RunWithConfig`, `CapabilityTraversal`, `BlockUntilCanceled`, `MaxResultLimit`로 일관되며 production API와 dependency 변경은 계획에 없다. 금지된 미완성 지시문은 사용하지 않는다.

## 재실행과 복구 지침

- Fake failure: 실패 출력의 exact test 이름을 그대로 anchored regex로 재실행한다. 예를 들어 `go test -count=1 ./graph/graphtest -run '^TestRunStopsAfterCoreFailureButCleansAndCloses$' -v`로 한 번 재현하고 해당 task commit 안에서 수정한다.
- Provider failure: 실패한 provider subtest만 `-count=1 -v`로 재현한다. 무차별 `-count` 증가는 사용하지 않는다.
- Cleanup leak: container terminate evidence와 `docker ps`의 task-owned image/container만 확인한다. 다른 작업의 container는 중지하지 않는다.
- Migration regression: Task 8 commit만 revert해 old/new 병행 상태로 복구한다.
- Canonical unrelated flake: 정확한 package/test/error와 targeted rerun 결과를 WIP/PR DoD에 `PENDING`으로 기록하고 exact-head CI/Nightly로 판정한다.

## 구현 완료 시 handoff

계획 실행은 다음 두 방식 중 하나를 선택한다.

1. **Subagent-Driven (권장):** task마다 fresh subagent를 배정하고 spec/quality review를 task 사이에 수행한다. REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`.
2. **Inline Execution:** 현재 session에서 task를 순서대로 실행하고 checkpoint마다 review한다. REQUIRED SUB-SKILL: `superpowers:executing-plans`.
