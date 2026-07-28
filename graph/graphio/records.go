package graphio

import "github.com/bluetape4k/bluetape-go/graph"

// RecordKind는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type RecordKind string

const (
	// RecordVertex는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	RecordVertex RecordKind = "vertex"
	// RecordEdge는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	RecordEdge RecordKind = "edge"
)

// Record는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type Record struct {
	Kind   RecordKind
	Vertex graph.Vertex
	Edge   graph.Edge
}

// VertexRecord는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func VertexRecord(vertex graph.Vertex) (Record, error) {
	record := Record{Kind: RecordVertex, Vertex: vertex}
	return record, record.Validate()
}

// EdgeRecord는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func EdgeRecord(edge graph.Edge) (Record, error) {
	record := Record{Kind: RecordEdge, Edge: edge}
	return record, record.Validate()
}

// Validate는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (r Record) Validate() error {
	vertexOK := r.Vertex.Validate() == nil
	edgeOK := r.Edge.Validate() == nil
	switch r.Kind {
	case RecordVertex:
		if !vertexOK || edgeOK {
			return wrap(ErrInvalidRecord, "", PhaseValidate, Location{}, "record", "", "expected exactly one valid vertex", nil)
		}
	case RecordEdge:
		if !edgeOK || vertexOK {
			return wrap(ErrInvalidRecord, "", PhaseValidate, Location{}, "record", "", "expected exactly one valid edge", nil)
		}
	default:
		return wrap(ErrInvalidRecord, "", PhaseValidate, Location{}, "kind", "", "unknown record kind", nil)
	}
	return nil
}
