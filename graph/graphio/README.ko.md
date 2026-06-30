# graph/graphio

[English](README.md) | [한국어](README.ko.md)

`graph.Vertex`와 `graph.Edge` 값을 위한 stream-oriented graph import/export
helper입니다.

이 패키지는 record boundary에만 집중합니다. NDJSON과 paired CSV stream은
지원하지만 graph database client, GraphML parser, repository/session contract,
schema DSL, query DSL, compression, encryption, path ownership, atomic file
replacement, backend adapter는 제공하지 않습니다.

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

NDJSON은 한 줄에 vertex 또는 edge envelope 하나를 저장합니다. Slice helper는
vertex를 edge보다 먼저 써서 import 시 edge가 이미 등장한 vertex만 참조하는지
검증할 수 있게 합니다.

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

CSV는 vertex stream과 edge stream을 나눠 사용합니다. Streaming writer는 명시적
`PropertyColumns`가 필요하고, finite slice helper는 전달받은 record에서 property
column을 찾을 수 있습니다. Streaming reader는 무제한 buffering 없이 missing
endpoint를 잡기 위해 vertex를 모두 읽은 뒤 edge를 읽도록 강제합니다.

## 안전 기본값

- `ReadOptions`는 duplicate vertex와 missing endpoint를 기본적으로 실패 처리합니다.
- `MaxLineBytes`, `MaxRecordBytes`, `MaxFieldBytes`, `MaxColumns`,
  `MaxRecords`는 기본 상한을 가집니다.
- `UnlimitedRecords`는 trusted input에서만 명시적으로 선택합니다.
- CSV read는 logical row를 `encoding/csv`에 넘기기 전에 `MaxRecordBytes`를
  검사합니다.
- CSV write는 기본적으로 `CSVFormulaEscape`를 사용합니다. Spreadsheet로 열
  가능성이 없는 graph interchange 출력에서만 `CSVFormulaRaw`를 사용합니다.
- Error와 report는 record ID를 redaction하고 raw payload를 보관하지 않습니다.
- Context cancellation은 record operation 사이에서 확인합니다. 이미 block된
  underlying `io.Reader` 또는 `io.Writer`를 깨우려면 caller-owned close/deadline
  동작이 필요합니다.
- NDJSON unknown field와 CSV의 reserved/property가 아닌 column은 첫 package
  slice에서 무시합니다. Strict schema rejection은 그 호환 계약을 필요로 하는
  importer가 생긴 뒤로 미룹니다.

## 미지원 범위

| Capability | Owner |
|---|---|
| GraphML import/export | NDJSON/CSV adoption evidence 이후 follow-up |
| Compression/encryption wrapper | Cross-package I/O policy로 이관 |
| Path import/export semantics | Graph algorithm 또는 backend adapter가 traversal contract를 정한 뒤 |
| Atomic file replacement와 filesystem ownership | 현재는 caller-owned |
| Backend adapter binding | #50 |
| Domain examples | #51 |

## Test

```bash
go test -count=1 ./graph/graphio
go test -race -count=1 ./graph/graphio
```
