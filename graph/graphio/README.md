# graph/graphio

[English](README.md) | [한국어](README.ko.md)

Stream-oriented graph import and export helpers for `graph.Vertex` and
`graph.Edge` values.

This package deliberately stays at the record boundary. It supports NDJSON and
paired CSV streams without introducing a graph database client,
repository/session contract, schema DSL, query DSL, compression, encryption,
path ownership, atomic file replacement, or backend adapter. The optional
`graph/graphio/graphml` subpackage owns the bounded GraphML subset.

## Record Flow

![Graph I/O Record Flow](../../docs/images/readme-diagrams/graph-io-record-flow.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/graph/graphio"
```

## NDJSON

```go
var output bytes.Buffer
writer := graphio.NewNDJSONWriter(ctx, &output, graphio.WriteOptions{})
if err := writer.WriteRecord(record); err != nil {
	return err
}
report, err := writer.Close()
```

NDJSON stores one vertex or edge envelope per line. Slice helpers write vertices
before edges so imports can validate that an edge only references vertices that
have already appeared in the stream.

## CSV

```go
var vertices bytes.Buffer
var edges bytes.Buffer
writer := graphio.NewCSVWriter(ctx, graphio.CSVWriterStreams{
	Vertices: &vertices,
	Edges:    &edges,
}, graphio.CSVWriteOptions{
	PropertyColumns: []string{"name", "since"},
	FormulaPolicy:   graphio.CSVFormulaRaw,
})
if err := writer.WriteVertex(vertex); err != nil {
	return err
}
if err := writer.WriteEdge(edge); err != nil {
	return err
}
report, err := writer.Close()
```

CSV uses paired streams: one for vertices and one for edges. Streaming writers
require explicit `PropertyColumns`; finite slice helpers can discover property
columns from the provided records. Streaming readers require callers to consume
all vertices before edges so missing endpoints are caught without buffering an
unbounded input.

## GraphML

```go
var output bytes.Buffer
report, err := graphml.Write(ctx, &output, records, graphml.WriteOptions{})
if err != nil {
	return err
}

records, report, err = graphml.Read(ctx, strings.NewReader(output.String()), graphml.ReadOptions{})
```

Import GraphML through
`github.com/bluetape4k/bluetape-go/graph/graphio/graphml` when a named producer
or workshop example needs XML interoperability. The first slice supports only a
directed property graph subset: `graphml`, `key`, `graph`, `node`, `edge`, and
scalar `data` values. `label` data keys become graph labels; other key/data
pairs become `graph.Properties`.

## Safety Defaults

- `ReadOptions` defaults to fail-closed duplicate vertex and missing endpoint
  policies.
- `MaxLineBytes`, `MaxRecordBytes`, `MaxFieldBytes`, `MaxColumns`, and
  `MaxRecords` are bounded by default.
- `UnlimitedRecords` is an explicit trusted-input opt-in.
- CSV reads enforce `MaxRecordBytes` before passing a logical row to
  `encoding/csv`.
- CSV writes default to `CSVFormulaEscape`; use `CSVFormulaRaw` only when the
  output is for graph interchange, not spreadsheet opening.
- Errors and reports keep record IDs redacted and do not retain raw payloads.
- Context cancellation is checked between record operations. A blocking
  underlying `io.Reader` or `io.Writer` still needs caller-owned close or
  deadline behavior to unblock in-flight I/O.
- NDJSON unknown fields and CSV non-reserved, non-property columns are ignored
  in this first package slice; strict schema rejection is deferred until another
  importer needs that compatibility contract.
- GraphML reads require bounded whole-document input, reject XML directives and
  non-declaration processing instructions, and fail closed for unknown keys,
  nested graphs, mixed directed/undirected graphs, hyperedges, ports, yFiles
  visual payloads, and arbitrary XML extensions.

## Unsupported Capabilities

| Capability | Owner |
|---|---|
| Broad GraphML/yEd/yFiles compatibility | Deferred beyond the bounded [`graph/graphio/graphml`](graphml) subset; see [issue #433 research](../../docs/research/2026-07-09-issue-433-graphml-graphio-evaluation.md) |
| Compression/encryption wrappers | Deferred to cross-package I/O policy |
| Path import/export semantics | Deferred until graph algorithms or backend adapters define traversal contracts |
| Atomic file replacement and filesystem ownership | Caller-owned for now |
| Backend adapter binding | #50 |
| Domain examples | #51 |

## Test

```bash
go test -count=1 ./graph/graphio
go test -count=1 ./graph/graphio/graphml
go test -race -count=1 ./graph/graphio
go test -race -count=1 ./graph/graphio/graphml
```
