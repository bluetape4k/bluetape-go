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

type Fixture struct { /* private semantic data */ }

func (f Fixture) Namespace() string
func (f Fixture) Vertices() []graph.Vertex
func (f Fixture) Edges() []graph.Edge
func (f Fixture) Validate() error

type Adapter struct {
    VerifyConnectivity func(context.Context) error
    CreateFixture      func(context.Context, Fixture) error
    ReadVertices       func(context.Context, Fixture) ([]graph.Vertex, error)
    ReadEdges          func(context.Context, Fixture) ([]graph.Edge, error)
    InvalidOperation   func(context.Context, Fixture) error
    BlockUntilCanceled func(context.Context) error
    CleanupFixture     func(context.Context, Fixture) error
    Close              func(context.Context) error
    Traverse           func(context.Context, Fixture) ([]graph.ElementID, error)
    IsProviderError    func(error) bool
}

type Factory func(testing.TB) (Adapter, error)

type Capability string

const CapabilityTraversal Capability = "traversal"

type Support struct {
    Enabled bool
    Reason  string
}

type Capabilities map[Capability]Support

type Harness struct {
    New          Factory
    Capabilities Capabilities
}

func Run(t *testing.T, harness Harness)
```

`Adapter`는 interface가 아니라 함수 필드 묶음이다. Production client를 억지로 공통
interface에 맞추지 않으며 test가 필요한 semantic operation만 노출한다.

## Fixture와 비교 계약

- Runner는 각 `Run`에 충돌하지 않는 unique namespace를 만든다.
- Fixture는 최소 두 vertex와 하나의 directed edge를 포함한다.
- Vertex는 서로 다른 label과 scalar properties를, edge는 명시적인 type, start/end와
  scalar properties를 가진다.
- backend-generated element ID는 저장 전 알 수 없을 수 있으므로 fixture의 logical key를
  property로 보존한다. Runner는 logical key로 정렬한 뒤 label/type/endpoints/properties를
  비교한다.
- Adapter는 backend-specific metadata를 공통 `Properties`에 섞지 않는다.
- 반환 slice 순서는 계약이 아니다. Runner가 deterministic key로 정렬한다.
- Fixture accessor는 caller mutation이 다음 scenario에 새지 않도록 clone을 반환한다.

## Lifecycle과 소유권

- Provider test가 container, server, credential과 driver 연결 설정을 소유한다.
- `Harness.New`는 그 설정으로 한 번의 `Run`에서 사용할 driver/client와 adapter를 만든다.
  Runner는 모든 subtest가 끝난 뒤 `Adapter.Close`를 정확히 한 번 호출하며, adapter는
  자신이 만든 driver/client를 닫는다.
- Container cleanup은 provider test가 `testing.TB.Cleanup`에 먼저 등록한다. Adapter
  close가 LIFO 순서로 container 종료보다 먼저 실행되도록 등록 순서를 검증한다.
- Runner는 scenario마다 unique `Fixture`를 만들고 성공/실패와 관계없이
  `CleanupFixture`를 bounded cleanup context로 호출한다.
- Cleanup은 원래 test context cancellation을 상속하지 않되 고정 timeout을 갖는다.
- `Close`는 adapter가 소유한 client/driver만 닫는다. Runner가 container를 직접 닫지 않는다.
- `Run`과 내부 subtest는 `t.Parallel`을 사용하지 않는다. 공유 container resource와
  deterministic cleanup 순서를 보호한다.
- 한 core subtest가 실패하면 이후 scenario를 실행하지 않는다. 단, 해당 fixture cleanup과
  adapter close는 반드시 시도한다.

## Strict core scenario

모든 backend가 다음 scenario를 skip 없이 통과해야 한다.

1. **Harness validation:** factory와 모든 core callback/classifier가 non-nil이고 잘못된
   capability 조합을 거절한다.
2. **Connectivity:** bounded context 안에서 backend 연결을 확인한다.
3. **Empty read:** isolated namespace에서 vertex/edge가 empty slice로 관측된다.
4. **Create/read vertices:** logical key, label과 properties가 손실 없이 공통 model로
   반환된다.
5. **Create/read edge:** type, properties와 directed start/end가 정확히 보존된다.
6. **Cancellation:** 이미 취소된 context와 in-flight cancellation을 `errors.Is`로
   보존하고 늦은 mutation을 남기지 않는다.
7. **Provider error:** invalid operation이 provider-classified error를 반환하며 query,
   credential 또는 fixture payload를 오류 문자열에 노출하지 않는다.
8. **Cleanup:** fixture가 bounded cleanup으로 제거되고 다음 empty read에 남지 않는다.
9. **Close:** adapter close가 bounded context에서 끝나며 반복 호출 정책은 provider
   closure 안에서 안전하게 처리한다.

Core scenario에서 `t.Skip`, 빈 reason 또는 nil callback을 허용하지 않는다.

## Optional capability 계약

첫 optional capability는 `CapabilityTraversal` 하나다.

- `Enabled=true`면 `Traverse` callback이 필수고 runner가 directed path의 element ID
  순서를 검증한다.
- `Enabled=false`면 `Traverse`는 nil이어야 하고 `Reason`은 trim 후 nonblank여야 한다.
- 알려지지 않은 capability, enabled인데 callback이 없는 경우, disabled인데 reason이 없는
  경우는 harness validation failure다.
- Neo4j와 Memgraph는 현재 선택한 공통 traversal을 지원하므로 둘 다 enabled로 같은
  scenario를 실행한다.
- Optional scenario 결과는 test output에 capability 이름과 provider가 준 reason을 남긴다.
  Core test 성공과 섞어 전체 compatibility처럼 표시하지 않는다.

## Context와 오류 계약

- 모든 callback context는 non-nil이어야 한다.
- Runner가 만드는 operation context는 고정된 case timeout을 갖는다.
- Pre-canceled context는 provider I/O 전에 `context.Canceled` 또는
  `context.DeadlineExceeded`를 보존한다.
- In-flight cancellation callback은 실제 blocking boundary에 도달한 뒤 cancel되고 bounded
  시간 안에 반환해야 한다.
- `IsProviderError`는 nil, context errors, graph validation sentinel과 unrelated raw error에
  `false`를 반환해야 한다.
- Invalid operation의 wrapped provider error와 그 error를 다시 `%w`로 감싼 값에는
  `true`를 반환해야 한다.
- Classifier panic은 runner가 recover해 named harness-validation failure로 보고한다.
- Error message에는 Cypher/Gremlin text, credentials, URI secret, 전체 parameters 또는
  fixture marker를 포함하지 않는다.

## 실패 모드와 대응

1. **공통 interface가 production abstraction으로 굳는다.** `graphtest` 안의 함수 필드
   adapter로 제한하고 production `graph`에는 provider interface를 추가하지 않는다.
2. **Provider가 어려운 core test를 skip한다.** Core callback을 필수화하고 capability
   metadata는 traversal 같은 명시적 extension에만 허용한다.
3. **공유 container state가 scenario 사이에 샌다.** unique namespace와 per-scenario
   bounded cleanup, empty-state read-back을 함께 검증한다.
4. **Cancellation test가 dispatch 전에만 끝나 false PASS한다.** Adapter의
   `BlockUntilCanceled`가 blocking boundary 도달을 보장하고 runner가 handshake 뒤 cancel한다.
5. **Error classifier가 항상 true다.** nil/context/validation/raw cause negative probe와
   wrapped provider error positive probe를 모두 실행한다.
6. **Result ordering 차이가 의미 차이로 오인된다.** Logical key로 canonical sort한 뒤
   directed endpoints와 semantic fields를 비교한다.
7. **Cleanup이 취소된 test context를 재사용해 실패한다.** `context.WithoutCancel` 기반의
   별도 bounded cleanup context를 사용하고 failure에도 close까지 진행한다.
8. **Error redaction test가 실제 query를 로그에 남긴다.** 고정 secret marker를 fixture에
   넣고 메시지에는 marker의 부재만 검사하며 값을 test output에 출력하지 않는다.

## Harness 자체 테스트

`graph/graphtest`는 외부 service 없이 fake adapter로 runner 자체를 검증한다.

- complete harness가 모든 core/optional scenario를 실행함
- nil factory/core callback/classifier와 잘못된 capability 조합을 fail-closed로 거절함
- core failure 뒤 다음 scenario는 중단하지만 cleanup/close는 실행함
- timeout/cancellation과 cleanup timeout을 bounded하게 보고함
- classifier의 always-true, always-false와 panic을 거절함
- 반환 slice mutation이 fixture 내부 상태에 영향을 주지 않음
- failure message가 secret marker와 provider raw payload를 노출하지 않음

공개 `MemoryHarness`는 추가하지 않는다. 이 issue의 목적은 provider compatibility이며
in-memory implementation이 실제 backend 검증을 대신하게 하지 않는다.

## 기존 backend 적용

- Neo4j와 Memgraph adapter를 `graph/neo4j/conformance_test.go`에서 구성한다.
- 기존 container helper와 인증/driver 설정을 재사용한다.
- Query 문자열과 parameters는 adapter closure 안에만 둔다.
- 현재 중복된 create/read/cancellation/error scenario는 shared runner로 이동한다.
- `VertexFromNode`, `EdgeFromRelationship`, record conversion의 순수 단위 테스트는
  backend conformance와 목적이 다르므로 기존 package test에 남긴다.
- Testcontainers suite는 shared port/resource를 고려해 순차 실행한다.
- FalkorDB/AGE는 이번 issue에서 구현하거나 compatibility PASS로 표시하지 않는다.

## 문서와 공개 경계

- `graph/graphtest/README.md`와 `graph/graphtest/README.ko.md`를 추가한다.
- `graph/README.md`, `graph/README.ko.md`에 model-only production 경계와 test-support
  사용법을 함께 기록한다.
- `graph/neo4j` README locale pair에는 Neo4j/Memgraph가 shared suite를 실행함을 기록한다.
- Root README package table은 public test-support package 노출 방식과 일치하게 갱신한다.
- `CHANGELOG.md`의 `[Unreleased]` 아래 독자 대상 항목은 한국어로 작성한다.
- API와 lifecycle이 작은 runner 중심이므로 diagram은 기본 N/A다. 구현 후 README가
  lifecycle을 명확히 설명하지 못할 때만 별도 diagram review를 연다.

## 호환성과 migration

Production `graph`와 `graph/neo4j` API를 변경하지 않으므로 기존 caller migration은 없다.
기존 backend test는 shared runner 호출로 이동하지만 conversion unit test는 유지한다.
`graph/graphtest`는 공개 test-support API이므로 이후 core callback 삭제, scenario 완화,
capability 의미 변경은 compatibility change로 다루고 release note를 제공한다.

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
- CI는 managed external graph service 없이 deterministic하게 실행된다.
- README locale pair가 production model 경계와 새 backend 참여 방법을 같은 의미로 설명한다.
- Targeted test, race, canonical repository gate와 exact-head GitHub CI가 성공한다.
- Step 6-R 7-Tier review가 `P0=0 P1=0`을 기록한다.

## DoD

- `[ ]` `graph/graphtest` public harness와 validation 구현
- `[ ]` Fake adapter 기반 harness self-test 구현
- `[ ]` Neo4j/Memgraph shared strict core와 traversal suite 연결
- `[ ]` 기존 중복 scenario 정리와 conversion unit test 보존
- `[ ]` package/root README locale pair와 CHANGELOG 동기화
- `[ ]` targeted test, race, `make ci` 성공
- `[ ]` Step 6-R 7-Tier review `P0=0 P1=0`
- `[ ]` Issue #555 metadata와 PR DoD read-back 완료
