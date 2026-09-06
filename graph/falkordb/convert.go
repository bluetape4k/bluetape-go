package falkordb

import (
	"fmt"
	"strconv"

	"github.com/bluetape4k/bluetape-go/graph"
)

// VertexFromValue는 map 형태의 bounded FalkorDB node 값을 graph.Vertex로 변환한다.
func VertexFromValue(value any) (graph.Vertex, error) {
	data, ok := value.(map[string]any)
	if !ok {
		return graph.Vertex{}, classified(ErrInvalidResult, nil)
	}
	id, err := elementString(data["id"])
	if err != nil {
		return graph.Vertex{}, err
	}
	label := "Vertex"
	if value, ok := data["label"].(string); ok && value != "" {
		label = value
	} else if labels, ok := data["labels"].([]any); ok && len(labels) > 0 {
		label, err = elementString(labels[0])
		if err != nil {
			return graph.Vertex{}, err
		}
	}
	properties, err := propertiesFromValue(data["properties"])
	if err != nil {
		return graph.Vertex{}, err
	}
	return graph.ParseVertex(id, label, properties)
}

// EdgeFromValue는 map 형태의 bounded FalkorDB relationship 값을 graph.Edge로 변환한다.
func EdgeFromValue(value any) (graph.Edge, error) {
	data, ok := value.(map[string]any)
	if !ok {
		return graph.Edge{}, classified(ErrInvalidResult, nil)
	}
	id, err := elementString(data["id"])
	if err != nil {
		return graph.Edge{}, err
	}
	label, err := elementString(data["label"])
	if err != nil {
		return graph.Edge{}, err
	}
	start, err := elementString(firstValue(data, "start_id", "source", "src"))
	if err != nil {
		return graph.Edge{}, err
	}
	end, err := elementString(firstValue(data, "end_id", "destination", "dest"))
	if err != nil {
		return graph.Edge{}, err
	}
	properties, err := propertiesFromValue(data["properties"])
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.ParseEdge(id, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
}

func vertexFromRow(row []any) (graph.Vertex, error) {
	if len(row) == 1 {
		return VertexFromValue(row[0])
	}
	if len(row) < 3 {
		return graph.Vertex{}, classified(ErrInvalidResult, nil)
	}
	id, err := elementString(row[0])
	if err != nil {
		return graph.Vertex{}, err
	}
	label, err := elementString(row[1])
	if err != nil {
		return graph.Vertex{}, err
	}
	properties, err := propertiesFromValue(row[2])
	if err != nil {
		return graph.Vertex{}, err
	}
	return graph.ParseVertex(id, label, properties)
}

func edgeFromRow(row []any) (graph.Edge, error) {
	if len(row) == 1 {
		return EdgeFromValue(row[0])
	}
	if len(row) < 5 {
		return graph.Edge{}, classified(ErrInvalidResult, nil)
	}
	id, err := elementString(row[0])
	if err != nil {
		return graph.Edge{}, err
	}
	label, err := elementString(row[1])
	if err != nil {
		return graph.Edge{}, err
	}
	start, err := elementString(row[2])
	if err != nil {
		return graph.Edge{}, err
	}
	end, err := elementString(row[3])
	if err != nil {
		return graph.Edge{}, err
	}
	properties, err := propertiesFromValue(row[4])
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.ParseEdge(id, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
}

func elementString(value any) (string, error) {
	switch value := value.(type) {
	case string:
		if value == "" {
			return "", classified(ErrInvalidResult, nil)
		}
		return value, nil
	case []byte:
		return elementString(string(value))
	case int:
		return strconv.Itoa(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("%w: element id has unsupported type", ErrInvalidResult)
	}
}

func propertiesFromValue(value any) (graph.Properties, error) {
	if value == nil {
		return nil, nil
	}
	data, ok := value.(map[string]any)
	if !ok {
		return nil, classified(ErrInvalidResult, nil)
	}
	properties := make(graph.Properties, len(data))
	for key, item := range data {
		if key == "" {
			return nil, classified(ErrInvalidResult, nil)
		}
		properties[key] = item
	}
	return properties, nil
}

func firstValue(data map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			return value
		}
	}
	return nil
}
