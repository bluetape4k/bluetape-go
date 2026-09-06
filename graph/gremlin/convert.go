package gremlin

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/bluetape4k/bluetape-go/graph"
)

// VertexFromValue는 공식 Gremlin vertex 또는 명시적 map shape를 graph.Vertex로 변환한다.
func VertexFromValue(value any) (graph.Vertex, error) {
	switch value := value.(type) {
	case *gremlingo.Vertex:
		if value == nil {
			return graph.Vertex{}, invalidResult("nil vertex")
		}
		return vertexFromElement(value.Element)
	case gremlingo.Vertex:
		return vertexFromElement(value.Element)
	case map[string]any:
		return vertexFromMap(value)
	case map[any]any:
		converted, err := stringMap(value)
		if err != nil {
			return graph.Vertex{}, err
		}
		return vertexFromMap(converted)
	default:
		return graph.Vertex{}, invalidResult("value is not a vertex")
	}
}

// EdgeFromValue는 공식 Gremlin edge 또는 명시적 map shape를 graph.Edge로 변환한다.
func EdgeFromValue(value any) (graph.Edge, error) {
	switch value := value.(type) {
	case *gremlingo.Edge:
		if value == nil {
			return graph.Edge{}, invalidResult("nil edge")
		}
		return edgeFromElement(*value)
	case gremlingo.Edge:
		return edgeFromElement(value)
	case map[string]any:
		return edgeFromMap(value)
	case map[any]any:
		converted, err := stringMap(value)
		if err != nil {
			return graph.Edge{}, err
		}
		return edgeFromMap(converted)
	default:
		return graph.Edge{}, invalidResult("value is not an edge")
	}
}

func vertexFromElement(element gremlingo.Element) (graph.Vertex, error) {
	id, err := elementString(element.Id)
	if err != nil {
		return graph.Vertex{}, err
	}
	label := strings.TrimSpace(element.Label)
	if label == "" {
		return graph.Vertex{}, invalidResult("vertex label is blank")
	}
	properties, err := propertiesFromValue(element.Properties)
	if err != nil {
		return graph.Vertex{}, err
	}
	return graph.ParseVertex(id, label, properties)
}

func edgeFromElement(edge gremlingo.Edge) (graph.Edge, error) {
	id, err := elementString(edge.Id)
	if err != nil {
		return graph.Edge{}, err
	}
	label := strings.TrimSpace(edge.Label)
	if label == "" {
		return graph.Edge{}, invalidResult("edge label is blank")
	}
	start, err := elementString(edge.OutV.Id)
	if err != nil {
		return graph.Edge{}, err
	}
	end, err := elementString(edge.InV.Id)
	if err != nil {
		return graph.Edge{}, err
	}
	properties, err := propertiesFromValue(edge.Properties)
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.ParseEdge(id, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
}

func vertexFromMap(data map[string]any) (graph.Vertex, error) {
	id, err := elementString(firstValue(data, "id", "@id"))
	if err != nil {
		return graph.Vertex{}, err
	}
	label, err := elementString(firstValue(data, "label", "@label"))
	if err != nil {
		return graph.Vertex{}, err
	}
	properties, err := propertiesFromElementMap(data, "vertex")
	if err != nil {
		return graph.Vertex{}, err
	}
	return graph.ParseVertex(id, label, properties)
}

func edgeFromMap(data map[string]any) (graph.Edge, error) {
	id, err := elementString(firstValue(data, "id", "@id"))
	if err != nil {
		return graph.Edge{}, err
	}
	label, err := elementString(firstValue(data, "label", "@label"))
	if err != nil {
		return graph.Edge{}, err
	}
	start, err := elementString(firstValue(data, "outV", "out", "OUT", "source", "start"))
	if err != nil {
		return graph.Edge{}, err
	}
	end, err := elementString(firstValue(data, "inV", "in", "IN", "target", "end"))
	if err != nil {
		return graph.Edge{}, err
	}
	properties, err := propertiesFromElementMap(data, "edge")
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.ParseEdge(id, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
}

func propertiesFromValue(value any) (graph.Properties, error) {
	if value == nil {
		return nil, nil
	}
	switch value := value.(type) {
	case map[string]any:
		properties := make(graph.Properties, len(value))
		for key, item := range value {
			if strings.TrimSpace(key) == "" {
				return nil, invalidResult("property key is blank")
			}
			properties[key] = normalizePropertyValue(item)
		}
		return properties, nil
	case map[any]any:
		converted, err := stringMap(value)
		if err != nil {
			return nil, err
		}
		return propertiesFromValue(converted)
	default:
		return nil, invalidResult("properties are not a map")
	}
}

func propertiesFromElementMap(data map[string]any, kind string) (graph.Properties, error) {
	if raw := firstValue(data, "properties", "valueMap"); raw != nil {
		return propertiesFromValue(raw)
	}
	properties := make(graph.Properties)
	for key, value := range data {
		switch strings.ToLower(key) {
		case "id", "@id", "label", "@label", "outv", "inv", "out", "in", "source", "target", "start", "end":
			continue
		default:
			if strings.TrimSpace(key) == "" {
				return nil, invalidResult("property key is blank")
			}
			properties[key] = normalizePropertyValue(value)
		}
	}
	if len(properties) == 0 {
		return nil, nil
	}
	return properties, nil
}

func normalizePropertyValue(value any) any {
	if values, ok := value.([]any); ok && len(values) == 1 {
		return normalizePropertyValue(values[0])
	}
	return value
}

func traversalKeys(value any) ([]string, error) {
	return traversalKeysBounded(value, int(^uint(0)>>1))
}

func traversalKeysBounded(value any, limit int) ([]string, error) {
	if limit < 0 {
		return nil, classified(ErrInvalidResult, "traversal result limit is invalid", nil)
	}
	keys := make([]string, 0, minInt(limit, 16))
	var visit func(any, int) error
	visit = func(item any, depth int) error {
		if depth > maxExpansionDepth {
			return invalidResult("nested traversal result is too deep")
		}
		switch item := item.(type) {
		case *gremlingo.Path:
			if item == nil {
				return invalidResult("nil path")
			}
			for _, object := range item.Objects {
				if err := visit(object, depth+1); err != nil {
					return err
				}
			}
			return nil
		case gremlingo.Path:
			copy := item
			return visit(&copy, depth+1)
		}
		if reflected, ok := sliceValue(item); ok {
			for index := 0; index < reflected.Len(); index++ {
				value := reflected.Index(index)
				if !value.CanInterface() {
					return invalidResult("nested traversal item is inaccessible")
				}
				if err := visit(value.Interface(), depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if len(keys) >= limit {
			return classified(ErrInvalidResult, "traversal result limit exceeded", nil)
		}
		key, err := traversalKey(item)
		if err != nil {
			return err
		}
		keys = append(keys, key)
		return nil
	}
	if err := visit(value, 0); err != nil {
		return nil, err
	}
	return keys, nil
}

func traversalKey(value any) (string, error) {
	switch value := value.(type) {
	case *gremlingo.Vertex:
		if value == nil {
			return "", invalidResult("nil traversal vertex")
		}
		return elementString(value.Id)
	case gremlingo.Vertex:
		return elementString(value.Id)
	case *gremlingo.Edge:
		if value == nil {
			return "", invalidResult("nil traversal edge")
		}
		return elementString(value.Id)
	case gremlingo.Edge:
		return elementString(value.Id)
	case map[string]any:
		return elementString(firstValue(value, "btgc_key", "id", "@id"))
	case map[any]any:
		converted, err := stringMap(value)
		if err != nil {
			return "", err
		}
		return traversalKey(converted)
	default:
		return elementString(value)
	}
}

func elementString(value any) (string, error) {
	if value == nil {
		return "", invalidResult("element id is nil")
	}
	switch value := value.(type) {
	case *gremlingo.Vertex:
		if value == nil {
			return "", invalidResult("element id is nil")
		}
		return elementString(value.Id)
	case gremlingo.Vertex:
		return elementString(value.Id)
	case map[string]any:
		return elementString(firstValue(value, "id", "@id"))
	case map[any]any:
		converted, err := stringMap(value)
		if err != nil {
			return "", err
		}
		return elementString(converted)
	case string:
		if strings.TrimSpace(value) == "" {
			return "", invalidResult("element id is blank")
		}
		return value, nil
	case []byte:
		return elementString(string(value))
	case int:
		return strconv.Itoa(value), nil
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32), nil
	default:
		return "", fmt.Errorf("%w: unsupported element id type", ErrInvalidResult)
	}
}

func firstValue(data map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			return value
		}
	}
	return nil
}

func stringMap(value map[any]any) (map[string]any, error) {
	converted := make(map[string]any, len(value))
	for key, item := range value {
		name, ok := key.(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, invalidResult("map key is not a non-blank string")
		}
		converted[name] = item
	}
	return converted, nil
}

func invalidResult(operation string) error {
	return classified(ErrInvalidResult, operation, nil)
}

const maxExpansionDepth = 64

func sliceValue(value any) (reflect.Value, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || (rv.Kind() != reflect.Array && rv.Kind() != reflect.Slice) || rv.Type().Elem().Kind() == reflect.Uint8 {
		return reflect.Value{}, false
	}
	return rv, true
}
