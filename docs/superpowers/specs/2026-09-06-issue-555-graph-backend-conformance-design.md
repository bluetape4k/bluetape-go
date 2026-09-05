# Issue #555 Graph Backend Conformance 설계

Issue: #555
Date: 2026-09-06
Milestone: 0.22.0
Work type: Type A full feature
Target package: `graph/graphtest`

## 문제와 현재 근거

`graph`는 `Vertex`, `Edge`, `Path`와 validation error를 제공하는 model-only package다.
`graph/neo4j`는 Neo4j Go driver를 통해 Cypher를 실행하고 결과를 공통 model로 변환하며,
Neo4j와 Memgraph Testcontainers 테스트가 backend별 시나리오를 따로 가진다. 이 상태에서
FalkorDB나 AGE 후보를 추가하면 공통 의미를 예제나 복제된 테스트로 추정하게 된다.

Issue #555는 새 production repository abstraction을 요구하지 않는다. 기존 backend가
실제로 보장하는 노드/엣지 의미, cancellation과 error boundary를 같은 runner로 검증하는
test-support contract가 필요하다.

현재 저장소의 설계 anchor:

- `graph/types.go`: 공통 `ElementID`, `Vertex`, `Edge`, `Path` 값 model
- `graph/errors.go`: validation sentinel과 예약된 `ErrUnsupportedCapability`
- `graph/neo4j/client.go`: connectivity, write, vertex/edge read와 driver error wrapping
- `graph/neo4j/client_test.go`, `graph/neo4j/memgraph_test.go`: 현재 backend 시나리오
- `leader/leadertest`, `ratelimit/ratelimittest`: public test harness와 strict runner 선례
- Issue #527 provider conformance design:
  `docs/superpowers/specs/2026-07-12-issue-527-provider-conformance-design.md`

외부 계약 근거:

- Issue #555: <https://github.com/bluetape4k/bluetape-go/issues/555>
- Go `testing.TB`: <https://pkg.go.dev/testing#TB>
- Neo4j Go transaction guidance:
  <https://neo4j.com/docs/go-manual/current/transactions/>
- Memgraph Bolt v4 compatibility:
  <https://memgraph.com/blog/memgraph-1-2-release-implementing-the-bolt-protocol-v4>

## 목표

- `graph/graphtest`가 backend-neutral semantic fixture와 strict conformance runner를
  제공한다.
- Neo4j와 Memgraph가 동일한 core scenario를 skip 없이 실행한다.
- provider-specific Cypher/Gremlin/native query와 driver lifecycle은 adapter 안에 둔다.
- optional traversal/query 차이만 capability metadata로 표현하고 누락 이유를 강제한다.
- cancellation, provider error 분류, 오류 redaction과 bounded cleanup을 공통 검증한다.
- 향후 backend가 복제된 scenario 없이 같은 진입점으로 compatibility를 증명하게 한다.

## 비목표

- 새 graph backend 추가
- production `Repository`, `Session`, transaction, schema 또는 query DSL 신설
- Cypher와 Gremlin의 의미 차이를 숨기는 최저 공통분모 query abstraction
- managed external graph service를 CI prerequisite로 사용
- backend container, credential, retry 또는 운영 정책의 공통 소유
- provider의 성능이나 feature completeness 순위 주장

## 채택한 접근

공개 test-support package `graph/graphtest`에 `Harness`, semantic `Fixture`, 함수 필드 기반
`Adapter`, optional `Capabilities`와 `Run`을 둔다. Runner는 raw query를 모르며
`graph.Vertex`, `graph.Edge`, `graph.ElementID`와 error만 관찰한다. Backend test는
자신의 query language와 client를 closure 안에서 사용한다.

strict core callback은 전부 필수고 capability로 끌 수 없다. 선택적인 traversal만
명시적 capability와 reason을 갖는다. 이 경계는 새 backend가 어려운 core case를
`skip`해 false PASS를 만드는 것을 막으면서 query language 차이는 숨기지 않는다.

## 공개 API 기준

구현 계획은 다음 shape를 compile-check 기준으로 삼는다. 세부 fixture field는 구현 전
test-spec에서 값까지 고정하되 raw query field는 추가하지 않는다.

```go
package graphtest

const (
    DefaultStartupTimeout = 90 * time.Second
    DefaultCaseTimeout = 10 * time.Second
    DefaultCleanupTimeout = 30 * time.Second
    DefaultCloseTimeout = 10 * time.Second
    MaxStartupTimeout = 5 * time.Minute
    MaxCaseTimeout = time.Minute
    MaxCleanupTimeout = 2 * time.Minute
    MaxCloseTimeout = time.Minute
    MaxResultLimit = 1024
)

type Config struct {
    StartupTimeout      time.Duration
    CaseTimeout         time.Duration
    CleanupTimeout      time.Duration
    CloseTimeout        time.Duration
    MaxVertices         int
    MaxEdges            int
    MaxTraversalResults int
}

func DefaultConfig() Config

type ProviderMetadata struct {
    Name           string
    Version        string
    ImageReference string
}

type Fixture struct { /* private semantic data */ }

func (f Fixture) Namespace() string
func (f Fixture) Vertices() []graph.Vertex
func (f Fixture) Edges() []graph.Edge
func (f Fixture) Validate() error

type Started func()

type Adapter struct {
    VerifyConnectivity func(context.Context) error
    CreateFixture      func(context.Context, Fixture) error
    ReadVertices       func(context.Context, Fixture) ([]graph.Vertex, error)
    ReadEdges          func(context.Context, Fixture) ([]graph.Edge, error)
    InvalidOperation   func(context.Context, Fixture) error
    BlockUntilCanceled func(context.Context, Fixture, Started) error
    CleanupFixture     func(context.Context, Fixture) error
    Close              func(context.Context) error
    Traverse           func(context.Context, Fixture) ([]string, error)
    IsProviderError    func(error) bool
}

type Factory func(context.Context, testing.TB, Config) (Adapter, error)

type Capability string

const CapabilityTraversal Capability = "traversal"

type Support struct {
    Enabled    bool
    ReasonCode string
}

type Capabilities map[Capability]Support

type Harness struct {
    Provider     ProviderMetadata
    New          Factory
    Capabilities Capabilities
}

func Run(t *testing.T, harness Harness)
func RunWithConfig(t *testing.T, harness Harness, config Config)
```

`Adapter`는 interface가 아니라 함수 필드 묶음이다. Production client를 억지로 공통
interface에 맞추지 않으며 test가 필요한 semantic operation만 노출한다.

`Run`은 `DefaultConfig`를 사용한다. 기본 결과 상한은 `MaxVertices=16`, `MaxEdges=16`,
`MaxTraversalResults=32`다. 모든 duration과 결과 상한은 positive여야 한다. Runner는
config를 한 번 normalize하고 같은 값을 factory와 모든 case에 전달한다.
`RunWithConfig`는 `Config{}`, 일부 zero 값, negative 값과 상한 초과 duration을 default로
보정하지 않고 named harness-validation failure로 거절한다. Default를 원하는 caller는
`Run` 또는 `DefaultConfig` 전체를 사용한다.
각 timeout은 대응하는 exported maximum 이하여야 하며 세 result limit은
`1..MaxResultLimit`이어야 한다. Boundary와 초과 입력은 config table test와
compile-checked `ExampleRunWithConfig`로 고정한다.
`StartupTimeout`은 factory/readiness, `CaseTimeout`은 각 core/optional operation,
`CleanupTimeout`은 각 fixture cleanup, `CloseTimeout`은 adapter close에 각각 적용한다.
Timeout 뒤 join grace는 `min(CloseTimeout, CaseTimeout/10)`이다.

`ProviderMetadata.Name`, `Version`, `ImageReference`는 nonblank, single-line이고 각각
64, 64, 256 byte 이하인 낮은 cardinality 진단 값이다. Control character, credential이
포함된 URI와 query text는 거절한다. `ImageReference`는 tag와 digest를 모두 포함한
immutable reference여야 한다.
Runner는 config와 metadata를 factory 호출 전에 검증한다. Zero/invalid metadata는 값을
출력하지 않고 `graphtest: invalid provider metadata` category로 실패한다.

## Fixture와 비교 계약

- Runner는 각 `Run`에 충돌하지 않는 unique namespace를 만든다.
- Namespace는 `crypto/rand` 16 byte를 lowercase hex로 인코딩하고 package prefix를 붙인다.
  허용 문자는 `[a-z0-9_-]`, 최대 길이는 64다. Random source 실패는 factory 호출 전
  named harness failure다.
- Fixture는 최소 두 vertex와 하나의 directed edge를 포함한다.
- Vertex는 서로 다른 label과 scalar properties를, edge는 명시적인 type, start/end와
  scalar properties를 가진다.
- backend-generated element ID는 저장 전 알 수 없을 수 있으므로 fixture의 logical key를
  property로 보존한다. Runner는 logical key로 정렬한 뒤 label/type/endpoints/properties를
  비교한다.
- Adapter는 backend-specific metadata를 공통 `Properties`에 섞지 않는다.
- 반환 slice 순서는 계약이 아니다. Runner가 deterministic key로 정렬한다.
- `ReadVertices`와 `ReadEdges`는 각각 config 상한보다 하나 많은 결과까지만 backend에서
  읽는다. Provider query는 fixture-derived filter를 적용하고 `limit+1`을 사용한다. Runner는
  sort 전에 상한 초과를 fail-closed로 거절한다.
- `Traverse`도 `MaxTraversalResults+1`까지만 읽고 runner가 결과 비교 전에 상한을 검사한다.
- `CreateFixture`, 각 read와 `CleanupFixture`는 각각 한 번의 query submission으로 batch한다.
  Vertex/edge별 query loop는 허용하지 않는다.
- Fixture accessor는 caller mutation이 다음 scenario에 새지 않도록 clone을 반환한다.

## Lifecycle과 소유권

- Provider test가 container, server, credential과 driver 연결 설정을 소유한다. Container는
  digest-pinned image와 `DefaultStartupTimeout` 또는 명시적인 positive startup budget으로
  시작하고 readiness를 deadline까지 retry한다.
- Provider test는 container 생성 직후 `internal/testcleanup.Register` 또는 동등한
  30초 bounded termination을 등록한다. CI의 Ryuk 활성 여부에 cleanup을 의존하지 않는다.
- `Harness.New`는 startup context와 설정으로 한 번의 `Run`에서 사용할 driver/client와
  adapter를 만든다. Factory error는 zero `Adapter`를 반환하고 자신이 부분 생성한
  driver/client를 반환 전에 닫아야 한다. Partial cleanup은 취소된 startup context를
  재사용하지 않고 `context.WithTimeout(context.WithoutCancel(startupCtx), CloseTimeout)`을
  사용하며 close error를 원래 factory error와 함께 보존한다.
- Runner는 모든 subtest가 끝난 뒤 `Adapter.Close`를 직접 정확히 한 번 호출한다.
  Provider test는 adapter close를 별도 `defer`/`Cleanup`으로 중복 등록하지 않는다.
- Runner는 scenario마다 unique `Fixture`를 만들고 성공/실패와 관계없이
  `CleanupFixture`를 bounded cleanup context로 호출한다.
- Cleanup은 원래 test context cancellation을 상속하지 않되 고정 timeout을 갖는다.
- `CleanupFixture`는 empty, partial-create, complete, already-cleaned fixture에 idempotent하다.
  Runner는 `CreateFixture` 호출 전에 cleanup을 예약해 partial failure도 회수한다.
- `Close`는 adapter가 소유한 client/driver만 닫는다. Runner가 container를 직접 닫지 않는다.
- `Run`과 내부 subtest는 `t.Parallel`을 사용하지 않는다. 공유 container resource와
  deterministic cleanup 순서를 보호한다.
- 한 core subtest가 실패하면 이후 scenario를 실행하지 않는다. 단, 해당 fixture cleanup과
  adapter close는 반드시 시도한다.
- Lifecycle 순서는 `fixture cleanup → adapter/client close → Run 반환 → container terminate`다.
  Scenario, cleanup, close 오류는 `errors.Join` 의미로 모두 보존하되 runner 출력에는
  provider, phase와 redacted category만 남긴다.
- Callback panic은 runner boundary에서 recover해 named failure로 바꾸고 cleanup/close를
  계속한다.
- Callback이 context를 지키면 config timeout 안에 끝나야 한다. Timeout 시 runner는 cancel하고
  위에서 정의한 join grace 동안 기다린다. Callback이 join된 뒤에만 fixture cleanup과 adapter
  close를 실행한다. Join되지 않으면 활성 callback과 driver를 동시에 닫지 않고 외부
  `go test -timeout`이 process를 fail-stop할 때까지 join한다. Goroutine을 분리해 버리거나
  non-cooperative callback을 성공/bounded PASS로 보고하지 않는다.

## Strict core scenario

모든 backend가 다음 scenario를 skip 없이 통과해야 한다.

1. **Harness validation:** factory와 모든 core callback/classifier가 non-nil이고 잘못된
   capability 조합을 거절한다.
2. **Connectivity:** bounded context 안에서 backend 연결을 확인한다.
3. **Empty read:** isolated namespace에서 vertex/edge가 empty slice로 관측된다.
4. **Create/read vertices:** logical key, label과 properties가 손실 없이 공통 model로
   반환된다.
5. **Create/read edge:** type, properties와 directed start/end가 정확히 보존된다.
6. **Cancellation:** 이미 취소된 context와 handshake 뒤의 in-flight cancellation을
   `errors.Is`로 보존하고 늦은 mutation을 남기지 않는다.
7. **Provider error:** invalid operation이 provider-classified error를 반환하며 query,
   credential 또는 fixture payload를 오류 문자열에 노출하지 않는다.
8. **Cleanup:** fixture가 bounded cleanup으로 제거되고 다음 empty read에 남지 않는다.
9. **Close:** adapter close가 `CloseTimeout` context를 지키며 정확히 한 번 끝난다.

Core scenario에서 `t.Skip`, 빈 reason 또는 nil callback을 허용하지 않는다.

## Optional capability 계약

첫 optional capability는 `CapabilityTraversal` 하나다.

- `Enabled=true`면 `Traverse` callback이 필수고 runner가 directed path의 logical key
  순서를 검증한다.
- 실제 반환값은 backend-generated `graph.ElementID`가 아니라 fixture vertex의 logical key다.
  시작 vertex부터 도착 vertex까지 vertex key만 순서대로 반환하고 edge ID는 포함하지 않는다.
  Runner는 fixture의 logical-key mapping으로 결과를 검증한다.
- `Enabled=false`면 `Traverse`는 nil이어야 하고 `ReasonCode`가 필요하다. Code는
  `[a-z0-9][a-z0-9._-]{0,63}`만 허용하며 자연어, URI, query, newline을 출력하지 않는다.
- `Capabilities`는 정확히 `CapabilityTraversal` key를 포함해야 한다. Nil/empty/map key 누락,
  알려지지 않은 capability, enabled인데 callback이 없는 경우, disabled인데 reason code가 없는
  경우는 harness validation failure다.
- Runner는 시작할 때 map을 defensive copy하고 key를 정렬한다. 실행 중 caller mutation은
  현재 run에 영향을 주지 않는다.
- Neo4j와 Memgraph는 현재 선택한 공통 traversal을 지원하므로 둘 다 enabled로 같은
  scenario를 실행한다.
- Optional scenario 결과는 test output에 capability 이름과 provider가 준 reason code를 남긴다.
  이때 raw text가 아니라 검증된 `ReasonCode`만 남긴다.
  Core test 성공과 섞어 전체 compatibility처럼 표시하지 않는다.

## Context와 오류 계약

- 모든 callback context는 non-nil이어야 한다.
- Runner가 만드는 operation context는 고정된 case timeout을 갖는다.
- Pre-canceled context는 provider I/O 전에 `context.Canceled` 또는
  `context.DeadlineExceeded`를 보존한다.
- `BlockUntilCanceled`는 provider I/O가 실제 blocking boundary에 도달한 뒤 `Started`를
  정확히 한 번 호출한다. Runner는 `CaseTimeout` 안에 신호를 기다린 뒤 context를 cancel하고
  completion을 join한다. 신호 전 반환, 무신호, 중복 신호와 cancel 뒤 late mutation은 실패다.
- `IsProviderError`는 nil, context errors, graph validation sentinel과 unrelated raw error에
  `false`를 반환해야 한다.
- Invalid operation의 wrapped provider error와 그 error를 다시 `%w`로 감싼 값에는
  `true`를 반환해야 한다.
- Classifier panic은 runner가 recover해 named harness-validation failure로 보고한다.
- Error message에는 Cypher/Gremlin text, credentials, URI secret, 전체 parameters,
  caller-controlled column 또는 fixture marker를 포함하지 않는다. 이 규칙은 factory,
  connectivity, operation, cleanup과 close 실패에 모두 적용한다. Raw cause는 error chain에서
  `errors.Is/As`로만 관찰하고 test output에는 format하지 않는다.
- Provider query text는 고정 문자열 또는 allowlisted identifier 조합으로 만들고 모든
  fixture-derived 값은 parameter로 bind한다. Result column은 고정 allowlist만 사용한다.

## 실패 모드와 대응

1. **공통 interface가 production abstraction으로 굳는다.** `graphtest` 안의 함수 필드
   adapter로 제한하고 production `graph`에는 provider interface를 추가하지 않는다.
2. **Provider가 어려운 core test를 skip한다.** Core callback을 필수화하고 capability
   metadata는 traversal 같은 명시적 extension에만 허용한다.
3. **공유 container state가 scenario 사이에 샌다.** unique namespace와 per-scenario
   bounded cleanup, empty-state read-back을 함께 검증한다.
4. **Cancellation test가 dispatch 전에만 끝나 false PASS한다.** Adapter의 `Started`
   handshake가 blocking boundary 도달을 보장하고 runner가 신호 뒤 cancel한다.
5. **Error classifier가 항상 true다.** nil/context/validation/raw cause negative probe와
   wrapped provider error positive probe를 모두 실행한다.
6. **Result ordering 차이가 의미 차이로 오인된다.** Logical key로 canonical sort한 뒤
   directed endpoints와 semantic fields를 비교한다.
7. **Cleanup이 취소된 test context를 재사용해 실패한다.** `context.WithoutCancel` 기반의
   별도 bounded cleanup context를 사용하고 failure에도 close까지 진행한다.
8. **Error redaction test가 실제 query를 로그에 남긴다.** 고정 secret marker를 fixture에
   넣고 메시지에는 marker의 부재만 검사하며 값을 test output에 출력하지 않는다.
9. **잘못된 filter가 전체 graph를 materialize한다.** 모든 read/traversal query에
   namespace predicate와 `limit+1`을 넣고 runner가 sort 전에 상한을 검사한다.
10. **Factory partial failure나 panic이 resource를 남긴다.** Factory self-cleanup,
    idempotent fixture cleanup, exactly-once close와 container fallback을 순서대로 검증한다.

## Harness 자체 테스트

`graph/graphtest`는 외부 service 없이 fake adapter로 runner 자체를 검증한다.

- complete harness가 모든 core/optional scenario를 실행함
- nil factory/core callback/classifier와 잘못된 capability 조합을 fail-closed로 거절함
- core failure 뒤 다음 scenario는 중단하지만 cleanup/close는 실행함
- timeout/cancellation과 cleanup timeout을 bounded하게 보고함
- started handshake의 missing/duplicate/early-return과 cancel 후 join을 검증함
- classifier의 always-true, always-false와 panic을 거절함
- oversized result를 sort 전에 거절하고 callback별 query submission count를 검증함
- nil/empty/missing/mutated capability map과 unsafe reason code를 거절함
- factory partial failure, cleanup/close error와 panic에서도 lifecycle 순서를 지킴
- 활성 callback이 join되기 전에 cleanup/close가 호출되지 않음을 검증함
- `Config{}`/partial config와 zero/unsafe metadata를 factory 호출 전에 거절함
- 반환 slice mutation이 fixture 내부 상태에 영향을 주지 않음
- failure message가 secret marker와 provider raw payload를 노출하지 않음

공개 `MemoryHarness`는 추가하지 않는다. 이 issue의 목적은 provider compatibility이며
in-memory implementation이 실제 backend 검증을 대신하게 하지 않는다.

## 기존 backend 적용

- Neo4j와 Memgraph adapter를 `graph/neo4j/conformance_test.go`에서 구성한다.
- 기존 container helper와 인증/driver 설정을 재사용한다.
- Provider images는 `graph/provider_benchmark_test.go`에서 검증한 Neo4j/Memgraph digest-pinned
  reference를 재사용한다. Startup은 90초 안에서 retry하고 실제 observed version이 metadata와
  맞는지 확인한다.
- Query 문자열과 parameters는 adapter closure 안에만 둔다.
- 첫 migration 단계에서는 기존 create/read/cancellation/error scenario와 shared runner를
  함께 실행해 parity를 확인한다. Targeted test와 exact-head CI가 통과한 다음 commit에서
  shared core와 겹치는 body만 제거한다. 문제가 생기면 그 제거 commit만 되돌린다.
- `VertexFromNode`, `EdgeFromRelationship`, record conversion의 순수 단위 테스트는
  backend conformance와 목적이 다르므로 기존 package test에 남긴다.
- Testcontainers suite는 shared port/resource를 고려해 순차 실행한다.
- Runner는 시작 시 sanitized provider/name/image/version을 한 번 기록하고, 각 named case의
  phase와 duration 및 cleanup/close 결과를 `t.Logf`에 기록한다. URI, credential, query와
  parameter는 기록하지 않는다.
- Neo4j와 Memgraph 두 suite의 기본 총 실행 budget은 10분 이내다. Broad retry로 flake를
  숨기지 않고 실패한 provider/case를 별도 targeted rerun해 attempt와 duration을 보존한다.
- FalkorDB/AGE는 이번 issue에서 구현하거나 compatibility PASS로 표시하지 않는다.

## 문서와 공개 경계

- `graph/graphtest/README.md`와 `graph/graphtest/README.ko.md`를 추가한다.
- `graph/README.md`, `graph/README.ko.md`에 model-only production 경계와 test-support
  사용법을 함께 기록한다.
- `graph/neo4j` README locale pair에는 Neo4j/Memgraph가 shared suite를 실행함을 기록한다.
- Root README package table은 public test-support package 노출 방식과 일치하게 갱신한다.
- Compile-checked `ExampleRun`과 `ExampleCapabilities`는 fake adapter로 factory, lifecycle,
  classifier, disabled reason code와 caller ownership을 보여준다.
- `CHANGELOG.md`의 `[Unreleased]` 아래 독자 대상 항목은 한국어로 작성한다.
- API와 lifecycle이 작은 runner 중심이므로 diagram은 기본 N/A다. 구현 후 README가
  lifecycle을 명확히 설명하지 못할 때만 별도 diagram review를 연다.

## 호환성과 migration

Production `graph`와 `graph/neo4j` API를 변경하지 않으므로 기존 caller migration은 없다.
기존 backend test는 shared runner 호출로 이동하지만 conversion unit test는 유지한다.
`graph/graphtest`는 공개 test-support API이므로 이후 core callback 삭제, scenario 완화,
capability 의미 변경은 compatibility change로 다루고 release note를 제공한다.

이 issue의 DoD는 shared conformance delivery까지다. Milestone open issue 0,
release-preparation branch, tag와 publication은 모든 0.22.0 issue가 끝난 뒤 별도 release
gate에서 확인한다.

## 거절한 대안

- **Neo4j/Cypher 형태의 공통 test interface:** 향후 Gremlin/native backend에 de facto
  production abstraction을 강요한다.
- **Provider 소유의 자유형 scenario callback:** Runner가 실제 의미를 관찰할 수 없어
  backend가 아무것도 하지 않고 PASS할 수 있다.
- **모든 차이를 capability로 표현:** Strict core가 최저 공통분모로 약화된다.
- **기존 test 복제:** 새 backend마다 의미 drift와 maintenance 비용이 반복된다.
- **Managed service CI:** credential과 외부 가용성을 build correctness에 결합한다.

## 테스트와 검증 명령

```bash
go test -count=1 ./graph/graphtest
go test -count=1 ./graph/neo4j
go test -race -count=1 ./graph/graphtest ./graph/neo4j
go test -count=3 ./graph/neo4j
make fmt-check
make tidy-check
make vet
make lint
make test
```

Docker-backed 검증은 healthy Colima context를 확인하고 다른 Testcontainers package와
순차 실행한다. Shared harness의 fake test는 Docker 없이 항상 실행되어야 한다.

## 수용 기준

- `graph/graphtest`가 위 semantic fixture, strict core, optional capability와 runner를
  제공한다.
- Neo4j와 Memgraph가 같은 strict core와 enabled traversal scenario를 실행한다.
- Core는 skip할 수 없고 optional 미지원에는 nonblank reason이 필요하다.
- Cancellation이 `errors.Is`를 보존하고 cleanup/close가 bounded하게 완료된다.
- Provider error classifier의 positive/negative/redaction probe가 모두 통과한다.
- Result/query count 상한, startup/case/cleanup/close budget과 non-cooperative fail-stop 계약을
  fake 및 provider test로 검증한다.
- CI는 managed external graph service 없이 deterministic하게 실행된다.
- README locale pair가 production model 경계와 새 backend 참여 방법을 같은 의미로 설명한다.
- Targeted test, race, canonical repository gate와 exact-head GitHub CI가 성공한다.
- Step 6-R 7-Tier review가 `P0=0 P1=0`을 기록한다.

## DoD

- `[ ]` `graph/graphtest` public harness와 validation 구현
- `[ ]` Fake adapter 기반 harness self-test 구현
- `[ ]` Neo4j/Memgraph shared strict core와 traversal suite 연결
- `[ ]` 기존/shared parity 검증 뒤 중복 scenario 정리와 conversion unit test 보존
- `[ ]` package/root README locale pair, compile-checked examples, CHANGELOG와 WIP 동기화
- `[ ]` targeted test, race, `make ci` 성공
- `[ ]` exact-head Testcontainers Nightly에서 provider/case timing과 cleanup evidence 확인
- `[ ]` Step 6-R 7-Tier review `P0=0 P1=0`
- `[ ]` Issue #555 metadata와 PR DoD read-back 완료
