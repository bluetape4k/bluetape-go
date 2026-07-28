package neo4j

import (
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
)

// VertexFromNode는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
//
// 이 주석은 graph format, backend requirement, traversal, serialization 조건을 설명한다.
// 세부 조건은 GraphML, NDJSON, CSV, Neo4j 계약과 caller-owned graph model을 따른다.
// 세부 조건은 GraphML, NDJSON, CSV, Neo4j 계약과 caller-owned graph model을 따른다.
func VertexFromNode(node dbtype.Node) (graph.Vertex, error) {
	id, err := elementID(node.ElementId)
	if err != nil {
		return graph.Vertex{}, errorWith(ErrInvalidRecord, "adapt node id", err)
	}
	label, err := nodeLabel(node.Labels)
	if err != nil {
		return graph.Vertex{}, errorWith(ErrInvalidRecord, "adapt node label", err)
	}
	vertex, err := graph.NewVertex(id, label, graph.Properties(node.Props))
	if err != nil {
		return graph.Vertex{}, errorWith(ErrInvalidRecord, "adapt node", err)
	}
	return vertex, nil
}

// EdgeFromRelationship는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func EdgeFromRelationship(relationship dbtype.Relationship) (graph.Edge, error) {
	id, err := elementID(relationship.ElementId)
	if err != nil {
		return graph.Edge{}, errorWith(ErrInvalidRecord, "adapt relationship id", err)
	}
	start, err := elementID(relationship.StartElementId)
	if err != nil {
		return graph.Edge{}, errorWith(ErrInvalidRecord, "adapt relationship start", err)
	}
	end, err := elementID(relationship.EndElementId)
	if err != nil {
		return graph.Edge{}, errorWith(ErrInvalidRecord, "adapt relationship end", err)
	}
	label, err := graph.NewLabel(relationship.Type)
	if err != nil {
		return graph.Edge{}, errorWith(ErrInvalidRecord, "adapt relationship type", err)
	}
	edge, err := graph.NewEdge(id, label, graph.EdgeEndpoints{Start: start, End: end}, graph.Properties(relationship.Props))
	if err != nil {
		return graph.Edge{}, errorWith(ErrInvalidRecord, "adapt relationship", err)
	}
	return edge, nil
}

func elementID(elementID string) (graph.ElementID, error) {
	return graph.NewElementID(elementID)
}

func nodeLabel(labels []string) (graph.Label, error) {
	normalized := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	if len(normalized) == 0 {
		return "", graph.ErrInvalidLabel
	}
	return graph.NewLabel(normalized[0])
}
