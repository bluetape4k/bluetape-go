package graph

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ElementID identifies a vertex or edge across graph adapters and examples.
type ElementID string

// NewElementID creates an ElementID from a non-blank string.
func NewElementID(value string) (ElementID, error) {
	normalized := strings.TrimSpace(value)
	id := ElementID(normalized)
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// ElementIDFromInt creates an ElementID from a non-negative integer backend ID.
func ElementIDFromInt(value int64) (ElementID, error) {
	if value < 0 {
		return "", validationError(ErrInvalidElementID, "id", "negative integer id", nil)
	}
	return ElementID(strconv.FormatInt(value, 10)), nil
}

// MustElementID creates an ElementID or panics for invalid constants and fixtures.
func MustElementID(value string) ElementID {
	id, err := NewElementID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the ID string.
func (id ElementID) String() string {
	return string(id)
}

// Validate returns ErrInvalidElementID when the ID is blank.
func (id ElementID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return validationError(ErrInvalidElementID, "id", "blank value", nil)
	}
	return nil
}

// MarshalJSON encodes the ID as a JSON string after validation.
func (id ElementID) MarshalJSON() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes and validates a JSON string ID.
func (id *ElementID) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return validationError(ErrInvalidElementID, "id", "expected string", err)
	}
	parsed, err := NewElementID(value)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// Label names a graph vertex or edge category.
type Label string

// NewLabel creates a Label from a non-blank string.
func NewLabel(value string) (Label, error) {
	normalized := strings.TrimSpace(value)
	label := Label(normalized)
	if err := label.Validate(); err != nil {
		return "", err
	}
	return label, nil
}

// String returns the label string.
func (label Label) String() string {
	return string(label)
}

// Validate returns ErrInvalidLabel when the label is blank.
func (label Label) Validate() error {
	if strings.TrimSpace(string(label)) == "" {
		return validationError(ErrInvalidLabel, "label", "blank value", nil)
	}
	return nil
}

// MarshalJSON encodes the label as a JSON string after validation.
func (label Label) MarshalJSON() ([]byte, error) {
	if err := label.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(label.String())
}

// UnmarshalJSON decodes and validates a JSON string label.
func (label *Label) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return validationError(ErrInvalidLabel, "label", "expected string", err)
	}
	parsed, err := NewLabel(value)
	if err != nil {
		return err
	}
	*label = parsed
	return nil
}

// Properties stores caller-owned graph metadata with shallow defensive copying.
type Properties map[string]any

// Clone returns a shallow copy of the property map.
func (p Properties) Clone() Properties {
	if p == nil {
		return nil
	}
	clone := make(Properties, len(p))
	for key, value := range p {
		clone[key] = value
	}
	return clone
}

// Vertex is a graph vertex value with an ID, label, and shallow properties.
type Vertex struct {
	id         ElementID
	label      Label
	properties Properties
}

// NewVertex creates a validated vertex from typed ID and label values.
func NewVertex(id ElementID, label Label, properties Properties) (Vertex, error) {
	if err := id.Validate(); err != nil {
		return Vertex{}, err
	}
	if err := label.Validate(); err != nil {
		return Vertex{}, err
	}
	return Vertex{id: id, label: label, properties: properties.Clone()}, nil
}

// ParseVertex creates a vertex from raw string ID and label values.
func ParseVertex(id string, label string, properties Properties) (Vertex, error) {
	parsedID, err := NewElementID(id)
	if err != nil {
		return Vertex{}, err
	}
	parsedLabel, err := NewLabel(label)
	if err != nil {
		return Vertex{}, err
	}
	return NewVertex(parsedID, parsedLabel, properties)
}

// ID returns the vertex ID.
func (v Vertex) ID() ElementID {
	return v.id
}

// Label returns the vertex label.
func (v Vertex) Label() Label {
	return v.label
}

// Properties returns a shallow defensive copy of vertex properties.
func (v Vertex) Properties() Properties {
	return v.properties.Clone()
}

// Validate returns ErrInvalidVertex when the vertex invariants are broken.
func (v Vertex) Validate() error {
	if err := v.id.Validate(); err != nil {
		return validationError(ErrInvalidVertex, "id", "invalid vertex id", err)
	}
	if err := v.label.Validate(); err != nil {
		return validationError(ErrInvalidVertex, "label", "invalid vertex label", err)
	}
	return nil
}

// MarshalJSON encodes a vertex with id, label, and properties fields.
func (v Vertex) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(vertexJSON{
		ID:         v.id,
		Label:      v.label,
		Properties: v.properties,
	})
}

// UnmarshalJSON decodes and validates a vertex JSON object.
func (v *Vertex) UnmarshalJSON(data []byte) error {
	var decoded vertexJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return validationError(ErrInvalidVertex, "json", "decode failed", err)
	}
	created, err := NewVertex(decoded.ID, decoded.Label, decoded.Properties)
	if err != nil {
		return err
	}
	*v = created
	return nil
}

type vertexJSON struct {
	ID         ElementID  `json:"id"`
	Label      Label      `json:"label"`
	Properties Properties `json:"properties"`
}

// EdgeEndpoints names the directed start and end IDs for an edge.
type EdgeEndpoints struct {
	Start ElementID
	End   ElementID
}

// Validate returns ErrInvalidElementID when either endpoint ID is invalid.
func (endpoints EdgeEndpoints) Validate() error {
	if err := endpoints.Start.Validate(); err != nil {
		return validationError(ErrInvalidElementID, "start", "invalid start endpoint", err)
	}
	if err := endpoints.End.Validate(); err != nil {
		return validationError(ErrInvalidElementID, "end", "invalid end endpoint", err)
	}
	return nil
}

// RawEdgeEndpoints names raw directed endpoint IDs before parsing.
type RawEdgeEndpoints struct {
	Start string
	End   string
}

// Edge is a directed graph edge value with an ID, label, endpoints, and properties.
type Edge struct {
	id         ElementID
	label      Label
	endpoints  EdgeEndpoints
	properties Properties
}

// NewEdge creates a validated directed edge from typed values.
func NewEdge(id ElementID, label Label, endpoints EdgeEndpoints, properties Properties) (Edge, error) {
	if err := id.Validate(); err != nil {
		return Edge{}, err
	}
	if err := label.Validate(); err != nil {
		return Edge{}, err
	}
	if err := endpoints.Validate(); err != nil {
		return Edge{}, err
	}
	return Edge{id: id, label: label, endpoints: endpoints, properties: properties.Clone()}, nil
}

// ParseEdge creates a directed edge from raw string values.
func ParseEdge(id string, label string, endpoints RawEdgeEndpoints, properties Properties) (Edge, error) {
	parsedID, err := NewElementID(id)
	if err != nil {
		return Edge{}, err
	}
	parsedLabel, err := NewLabel(label)
	if err != nil {
		return Edge{}, err
	}
	start, err := NewElementID(endpoints.Start)
	if err != nil {
		return Edge{}, err
	}
	end, err := NewElementID(endpoints.End)
	if err != nil {
		return Edge{}, err
	}
	return NewEdge(parsedID, parsedLabel, EdgeEndpoints{Start: start, End: end}, properties)
}

// ID returns the edge ID.
func (e Edge) ID() ElementID {
	return e.id
}

// Label returns the edge label.
func (e Edge) Label() Label {
	return e.label
}

// StartID returns the directed edge start ID.
func (e Edge) StartID() ElementID {
	return e.endpoints.Start
}

// EndID returns the directed edge end ID.
func (e Edge) EndID() ElementID {
	return e.endpoints.End
}

// Properties returns a shallow defensive copy of edge properties.
func (e Edge) Properties() Properties {
	return e.properties.Clone()
}

// Validate returns ErrInvalidEdge when the edge invariants are broken.
func (e Edge) Validate() error {
	if err := e.id.Validate(); err != nil {
		return validationError(ErrInvalidEdge, "id", "invalid edge id", err)
	}
	if err := e.label.Validate(); err != nil {
		return validationError(ErrInvalidEdge, "label", "invalid edge label", err)
	}
	if err := e.endpoints.Validate(); err != nil {
		return validationError(ErrInvalidEdge, "endpoints", "invalid edge endpoints", err)
	}
	return nil
}

// MarshalJSON encodes an edge with id, label, start, end, and properties fields.
func (e Edge) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(edgeJSON{
		ID:         e.id,
		Label:      e.label,
		Start:      e.endpoints.Start,
		End:        e.endpoints.End,
		Properties: e.properties,
	})
}

// UnmarshalJSON decodes and validates an edge JSON object.
func (e *Edge) UnmarshalJSON(data []byte) error {
	var decoded edgeJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return validationError(ErrInvalidEdge, "json", "decode failed", err)
	}
	created, err := NewEdge(
		decoded.ID,
		decoded.Label,
		EdgeEndpoints{Start: decoded.Start, End: decoded.End},
		decoded.Properties,
	)
	if err != nil {
		return err
	}
	*e = created
	return nil
}

type edgeJSON struct {
	ID         ElementID  `json:"id"`
	Label      Label      `json:"label"`
	Start      ElementID  `json:"start"`
	End        ElementID  `json:"end"`
	Properties Properties `json:"properties"`
}
