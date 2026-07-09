# graph/graphio/graphml

[English](README.md) | [한국어](README.ko.md)

`graph/graphio/graphml`은 `graphio.Record` 값을 위한 bounded GraphML subset을
import/export합니다. Named producer나 workshop example에서 XML GraphML
interoperability가 필요할 때 사용하고, 더 단순한 기본 record boundary는 NDJSON 또는
paired CSV로 유지합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/graph/graphio/graphml"
```

## 사용 예

```go
var output bytes.Buffer
report, err := graphml.Write(ctx, &output, records, graphml.WriteOptions{})
if err != nil {
	return err
}

records, report, err = graphml.Read(ctx, strings.NewReader(output.String()), graphml.ReadOptions{})
```

## 지원 subset

- 하나의 `graphml` root와 하나의 directed `graph`.
- `for="node"`, `for="edge"`, `for="all"` 범위의 `key` declaration.
- GraphML `string`, `boolean`, `int`, `long`, `float`, `double` attribute type을
  가진 scalar `data` 값.
- 명시적 ID를 가진 `node`와 `edge` record.
- `label` data key는 graph label이 되고, 나머지 data key는 `graph.Properties`로
  변환됩니다.
- Duplicate vertex ID, duplicate edge ID, missing endpoint, unknown data key,
  malformed value, oversized input은 fail-closed 처리합니다.

## 비목표

- Nested graph, directed/undirected 혼합 graph, hyperedge, port, yEd/yFiles
  visual styling, arbitrary XML extension payload, compression, path ownership,
  filesystem ownership, graph database import/export semantics.
- 문서화된 subset과 test를 넘어서는 Gephi, NetworkX, Neo4j APOC, yEd broad
  compatibility claim.

## 안전성

Read는 `ReadOptions.MaxInputBytes`를 통해 bounded whole-document parser를
사용합니다. Parser는 XML directive, declaration이 아닌 processing instruction,
unsupported element, `data` 내부 nested payload를 거부합니다. Context cancellation은
parse/write loop 전과 중간에 확인하지만, 이미 block된 I/O를 깨우려면 caller-owned
reader close나 deadline이 필요합니다.

## 테스트

```bash
go test -count=1 ./graph/graphio/graphml
go test -race -count=1 ./graph/graphio/graphml
```
