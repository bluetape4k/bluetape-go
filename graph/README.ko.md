# graph

[English](README.md) | [한국어](README.ko.md)

bluetape-go I/O helper와 예제에서 공유할 model-only graph 값입니다.

이 패키지는 일부러 작게 유지합니다. Vertex, edge, path, label, ID,
property 값을 검증하지만 graph database client, repository, session,
transaction, schema DSL, query DSL, algorithm engine, backend adapter는 제공하지
않습니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/graph"
```

## Usage

```go
vertex, err := graph.ParseVertex("person-1", "Person", graph.Properties{
	"name": "Alice",
})
if err != nil {
	return err
}

edge, err := graph.ParseEdge(
	"edge-1",
	"KNOWS",
	graph.RawEdgeEndpoints{Start: "person-1", End: "person-2"},
	nil,
)
if err != nil {
	return err
}

vertexStep, err := graph.VertexStep(vertex)
if err != nil {
	return err
}
edgeStep, err := graph.EdgeStep(edge)
if err != nil {
	return err
}
path, err := graph.NewPath(vertexStep, edgeStep)
if err != nil {
	return err
}
```

Caller 입력이나 backend 입력에는 `NewElementID` 또는 `ParseVertex`/`ParseEdge`를
사용합니다. `MustElementID`는 constant와 static test fixture 전용입니다.

## Diagram

![graph model contract map](../docs/images/readme-diagrams/graph-model-contract-map.png)

Contract map은 model-only 경계를 보여줍니다. Scalar ID와 label은 validated
vertex/edge value로 들어가고, properties는 shallow copy만 수행하며, path value는
container로 남습니다. Backend, I/O, query, schema, traversal 책임은 이 package
밖에 있습니다.

![graph parse and path sequence](../docs/images/readme-diagrams/graph-parse-path-sequence.png)

Sequence는 raw input이 scalar construction, vertex/edge 생성, property clone,
path-step construction, `NewPath`, redacted `ValidationError` reporting으로 이어지는
흐름을 보여줍니다.

## Validation Errors

```go
_, err := graph.ParseVertex("", "Person", nil)
if errors.Is(err, graph.ErrInvalidElementID) {
	// caller input used a blank ID
}

var validation *graph.ValidationError
if errors.As(err, &validation) {
	// Field와 Summary는 redacted 상태이며 raw value를 보관하지 않습니다.
}
```

`ValidationError`는 typed error category, field name, redacted summary, cause만
노출합니다. Raw property value, raw ID, raw label은 보관하지 않습니다.

JSON decode는 필수 graph value, 필수 `Path` field, path-step shape를 검증합니다.
하지만 untrusted I/O record를 위한 strict schema validator는 아닙니다.
[`graph/graphio`](graphio/README.ko.md) package가 NDJSON, paired CSV record,
optional bounded GraphML subset의 stream-level size-limit, duplicate-vertex,
missing-endpoint 정책을 담당합니다.

## Path Scope

`Path`는 model container입니다. `NewPath`, `NewWeightedPath`, `Path.Validate`는
step value와 aggregate weight만 확인하고 endpoint continuity, vertex/edge
교대 순서, traversal correctness는 증명하지 않습니다. 그런 더 엄격한 invariant는
이후 algorithm 또는 backend adapter가 담당합니다.

## Raw Record Adaptation

```go
type rawRelationship struct {
	ID    string
	Type  string
	Start string
	End   string
}

relationship := rawRelationship{
	ID: "edge-1", Type: "KNOWS", Start: "person-1", End: "person-2",
}
edge, err := graph.ParseEdge(
	relationship.ID,
	relationship.Type,
	graph.RawEdgeEndpoints{Start: relationship.Start, End: relationship.End},
	nil,
)
```

Broad `any` ID conversion helper는 제공하지 않습니다. Numeric backend ID에는
`ElementIDFromInt`를 사용하고, 다른 adapter는 boundary에서 명시적으로 parse해야
합니다. Raw parse helper는 작은 local example을 위한 convenience constructor라 ID와
label을 별도 string parameter로 유지합니다. 장기 adapter는 named local struct로 raw
record를 매핑한 뒤 호출하는 편이 안전합니다.

## Properties

`Properties`는 `map[string]any`이며 map boundary에서만 shallow defensive copy를
합니다. Nested mutable value는 caller-owned 상태로 남습니다. 이 패키지는
deep-copy, sanitization, trust-boundary primitive가 아니므로 backend/I/O adapter는
trust boundary를 넘기 전에 nested value를 직접 복사하거나 정제해야 합니다.

## Backend conformance test-support

Production `graph`는 계속 model-only 경계를 유지합니다. Backend 구현은 별도
[`graph/graphtest`](graphtest/README.ko.md) test-support package로 graph 의미,
cancellation, provider error 분류, bounded cleanup/close, optional traversal의
strict shared contract를 실행할 수 있습니다. 이 harness는 production repository나
query abstraction이 아닙니다.

`RunWithConfig`에는 모든 필드가 채워진 positive config가 필요합니다. Provider
adapter는 fixed query, bound parameter, `limit+1` result 요청, credential,
readiness, container termination을 소유하고 harness는 callback join, fixture
cleanup, adapter close 순서를 통제합니다.

## Unsupported Capabilities

| Capability | Owner |
|---|---|
| NDJSON, paired CSV, bounded GraphML Graph I/O helper | [`graph/graphio`](graphio/README.ko.md), [`graph/graphio/graphml`](graphio/graphml) |
| Neo4j backend proof | [`graph/neo4j`](neo4j/README.ko.md) |
| Backend conformance test-support | [`graph/graphtest`](graphtest/README.ko.md) |
| Broad GraphML/yEd/yFiles compatibility | Bounded `graphio/graphml` subset 이후로 이관; [issue #433 research](../docs/research/2026-07-09-issue-433-graphml-graphio-evaluation.md) 참고 |
| Neo4j surface 기반 Memgraph compatibility | [`graph/neo4j`](neo4j/README.ko.md) |
| Domain examples | [`examples/graph/observability`](../examples/graph/observability/README.ko.md), [`examples/graph/iamaccess`](../examples/graph/iamaccess/README.ko.md) |
| Repository/session/schema/query/transaction contracts | 여러 backend package가 shared contract를 증명한 뒤 결정 |

`ErrUnsupportedCapability`는 이후 capability boundary를 위해 예약되어 있습니다.
이 패키지의 public API는 아직 이 error를 반환하지 않습니다.

## Workshop Adoption

Package-local graph 예제는
[`examples/graph/observability`](../examples/graph/observability/README.ko.md)와
[`examples/graph/iamaccess`](../examples/graph/iamaccess/README.ko.md)에 둡니다.
Workshop scenario adoption은 issue
[#36](https://github.com/bluetape4k/bluetape-go-workshop/issues/36),
[#50](https://github.com/bluetape4k/bluetape-go-workshop/issues/50),
[#51](https://github.com/bluetape4k/bluetape-go-workshop/issues/51),
[#52](https://github.com/bluetape4k/bluetape-go-workshop/issues/52),
[#69](https://github.com/bluetape4k/bluetape-go-workshop/issues/69)에 추적합니다.

## Release Support

Graph package family는 service/runtime dependency가 없습니다. Release tag 전
rollback은 `graph`, `graph/graphio`, README update, release bookkeeping을 제거하는
것입니다. Release tag 이후에는 Go API 호환성을 유지하거나 breaking release로
미뤄야 합니다.

## Test

```bash
go test -count=1 ./graph
go test -count=1 ./graph/graphio
go test -count=1 ./graph/graphio/graphml
go test -count=1 ./graph/graphtest
go test -p 1 -count=1 -timeout=10m ./graph/neo4j
go test -race -count=1 ./graph
go test -race -count=1 ./graph/graphio
go test -race -count=1 ./graph/graphio/graphml
go test -race -count=1 ./graph/graphtest
go test -p 1 -race -count=1 -timeout=15m ./graph/neo4j
```
