package graph

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ElementID graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type ElementID string

// NewElementID graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewElementID(value string) (ElementID, error) {
	normalized := strings.TrimSpace(value)
	id := ElementID(normalized)
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// ElementIDFromInt graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func ElementIDFromInt(value int64) (ElementID, error) {
	if value < 0 {
		return "", validationError(ErrInvalidElementID, "id", "negative integer id", nil)
	}
	return ElementID(strconv.FormatInt(value, 10)), nil
}

// MustElementID graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func MustElementID(value string) ElementID {
	id, err := NewElementID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// String graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (id ElementID) String() string {
	return string(id)
}

// Validate graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (id ElementID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return validationError(ErrInvalidElementID, "id", "blank value", nil)
	}
	return nil
}

// MarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (id ElementID) MarshalJSON() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(id.String())
}

// UnmarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// Label graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Label string

// NewLabel graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewLabel(value string) (Label, error) {
	normalized := strings.TrimSpace(value)
	label := Label(normalized)
	if err := label.Validate(); err != nil {
		return "", err
	}
	return label, nil
}

// String graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (label Label) String() string {
	return string(label)
}

// Validate graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (label Label) Validate() error {
	if strings.TrimSpace(string(label)) == "" {
		return validationError(ErrInvalidLabel, "label", "blank value", nil)
	}
	return nil
}

// MarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (label Label) MarshalJSON() ([]byte, error) {
	if err := label.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(label.String())
}

// UnmarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// Properties graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Properties map[string]any

// Clone graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
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

// Vertex graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type Vertex struct {
	id         ElementID
	label      Label
	properties Properties
}

// NewVertex graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewVertex(id ElementID, label Label, properties Properties) (Vertex, error) {
	if err := id.Validate(); err != nil {
		return Vertex{}, err
	}
	if err := label.Validate(); err != nil {
		return Vertex{}, err
	}
	return Vertex{id: id, label: label, properties: properties.Clone()}, nil
}

// ParseVertex graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
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

// ID graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (v Vertex) ID() ElementID {
	return v.id
}

// Label graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (v Vertex) Label() Label {
	return v.label
}

// Properties graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (v Vertex) Properties() Properties {
	return v.properties.Clone()
}

// Validate graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (v Vertex) Validate() error {
	if err := v.id.Validate(); err != nil {
		return validationError(ErrInvalidVertex, "id", "invalid vertex id", err)
	}
	if err := v.label.Validate(); err != nil {
		return validationError(ErrInvalidVertex, "label", "invalid vertex label", err)
	}
	return nil
}

// MarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// UnmarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// EdgeEndpoints graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type EdgeEndpoints struct {
	Start ElementID
	End   ElementID
}

// Validate graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (endpoints EdgeEndpoints) Validate() error {
	if err := endpoints.Start.Validate(); err != nil {
		return validationError(ErrInvalidElementID, "start", "invalid start endpoint", err)
	}
	if err := endpoints.End.Validate(); err != nil {
		return validationError(ErrInvalidElementID, "end", "invalid end endpoint", err)
	}
	return nil
}

// RawEdgeEndpoints graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type RawEdgeEndpoints struct {
	Start string
	End   string
}

// Edge graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type Edge struct {
	id         ElementID
	label      Label
	endpoints  EdgeEndpoints
	properties Properties
}

// NewEdge graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
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

// ParseEdge graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
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

// ID graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e Edge) ID() ElementID {
	return e.id
}

// Label graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e Edge) Label() Label {
	return e.label
}

// StartID graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e Edge) StartID() ElementID {
	return e.endpoints.Start
}

// EndID graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e Edge) EndID() ElementID {
	return e.endpoints.End
}

// Properties graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e Edge) Properties() Properties {
	return e.properties.Clone()
}

// Validate graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
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

// MarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// UnmarshalJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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
