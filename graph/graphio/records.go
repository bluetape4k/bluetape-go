package graphio

import "github.com/bluetape4k/bluetape-go/graph"

// RecordKind identifies whether a Record contains a vertex or edge.
type RecordKind string

const (
	// RecordVertex identifies a vertex record.
	RecordVertex RecordKind = "vertex"
	// RecordEdge identifies an edge record.
	RecordEdge RecordKind = "edge"
)

// Record is a one-of vertex or edge value for neutral graph streams.
type Record struct {
	Kind   RecordKind
	Vertex graph.Vertex
	Edge   graph.Edge
}

// VertexRecord creates a vertex record.
func VertexRecord(vertex graph.Vertex) (Record, error) {
	record := Record{Kind: RecordVertex, Vertex: vertex}
	return record, record.Validate()
}

// EdgeRecord creates an edge record.
func EdgeRecord(edge graph.Edge) (Record, error) {
	record := Record{Kind: RecordEdge, Edge: edge}
	return record, record.Validate()
}

// Validate verifies that exactly one graph value matches Kind.
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
