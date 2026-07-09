# graph/graphio/graphml

[English](README.md) | [한국어](README.ko.md)

`graph/graphio/graphml` imports and exports a bounded GraphML subset for
`graphio.Record` values. Use it when a named producer or workshop example needs
XML GraphML interoperability; keep NDJSON or paired CSV as the simpler default
record boundary.

## Import

```go
import "github.com/bluetape4k/bluetape-go/graph/graphio/graphml"
```

## Usage

```go
var output bytes.Buffer
report, err := graphml.Write(ctx, &output, records, graphml.WriteOptions{})
if err != nil {
	return err
}

records, report, err = graphml.Read(ctx, strings.NewReader(output.String()), graphml.ReadOptions{})
```

## Supported Subset

- One `graphml` root with one directed `graph`.
- `key` declarations with `for="node"`, `for="edge"`, or `for="all"`.
- Scalar `data` values with GraphML `string`, `boolean`, `int`, `long`,
  `float`, and `double` attribute types.
- `node` and `edge` records with explicit IDs.
- `label` data keys map to graph labels; other data keys map to
  `graph.Properties`.
- Duplicate vertex IDs, duplicate edge IDs, missing endpoints, unknown data
  keys, malformed values, and oversized inputs fail closed.

## Non-Goals

- Nested graphs, mixed directed/undirected graphs, hyperedges, ports, yEd/yFiles
  visual styling, arbitrary XML extension payloads, compression, path ownership,
  filesystem ownership, and graph database import/export semantics.
- Broad compatibility claims for Gephi, NetworkX, Neo4j APOC, or yEd beyond the
  documented subset and tests.

## Safety

Reads use a bounded whole-document parser through `ReadOptions.MaxInputBytes`.
The parser rejects XML directives, non-declaration processing instructions,
unsupported elements, and nested payloads inside `data`. Context cancellation is
checked before and during parse/write loops; caller-owned reader close or
deadlines are still required to unblock already-blocked I/O.

## Test

```bash
go test -count=1 ./graph/graphio/graphml
go test -race -count=1 ./graph/graphio/graphml
```
