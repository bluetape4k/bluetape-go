package graph

import (
	"encoding/json"
	"math"
)

type pathStepKind uint8

const (
	pathStepInvalid pathStepKind = iota
	pathStepVertex
	pathStepEdge
)

// PathStep는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type PathStep struct {
	kind   pathStepKind
	vertex Vertex
	edge   Edge
}

// VertexStep는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func VertexStep(vertex Vertex) (PathStep, error) {
	if err := vertex.Validate(); err != nil {
		return PathStep{}, validationError(ErrInvalidPath, "vertex", "invalid vertex step", err)
	}
	return PathStep{kind: pathStepVertex, vertex: vertex}, nil
}

// EdgeStep는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func EdgeStep(edge Edge) (PathStep, error) {
	if err := edge.Validate(); err != nil {
		return PathStep{}, validationError(ErrInvalidPath, "edge", "invalid edge step", err)
	}
	return PathStep{kind: pathStepEdge, edge: edge}, nil
}

// IsVertex는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (step PathStep) IsVertex() bool {
	return step.kind == pathStepVertex
}

// IsEdge는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (step PathStep) IsEdge() bool {
	return step.kind == pathStepEdge
}

// Vertex는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (step PathStep) Vertex() (Vertex, bool) {
	if !step.IsVertex() {
		return Vertex{}, false
	}
	return step.vertex, true
}

// Edge는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (step PathStep) Edge() (Edge, bool) {
	if !step.IsEdge() {
		return Edge{}, false
	}
	return step.edge, true
}

// Validate는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (step PathStep) Validate() error {
	switch step.kind {
	case pathStepVertex:
		if err := step.vertex.Validate(); err != nil {
			return validationError(ErrInvalidPath, "vertex", "invalid vertex step", err)
		}
		return nil
	case pathStepEdge:
		if err := step.edge.Validate(); err != nil {
			return validationError(ErrInvalidPath, "edge", "invalid edge step", err)
		}
		return nil
	default:
		return validationError(ErrInvalidPath, "step", "missing step value", nil)
	}
}

// MarshalJSON는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (step PathStep) MarshalJSON() ([]byte, error) {
	if err := step.Validate(); err != nil {
		return nil, err
	}
	if step.IsVertex() {
		return json.Marshal(pathStepJSON{Vertex: &step.vertex})
	}
	return json.Marshal(pathStepJSON{Edge: &step.edge})
}

// UnmarshalJSON는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (step *PathStep) UnmarshalJSON(data []byte) error {
	var decoded pathStepJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return validationError(ErrInvalidPath, "step", "decode failed", err)
	}
	hasVertex := decoded.Vertex != nil
	hasEdge := decoded.Edge != nil
	if hasVertex == hasEdge {
		return validationError(ErrInvalidPath, "step", "expected exactly one vertex or edge", nil)
	}
	if hasVertex {
		created, err := VertexStep(*decoded.Vertex)
		if err != nil {
			return err
		}
		*step = created
		return nil
	}
	created, err := EdgeStep(*decoded.Edge)
	if err != nil {
		return err
	}
	*step = created
	return nil
}

type pathStepJSON struct {
	Vertex *Vertex `json:"vertex,omitempty"`
	Edge   *Edge   `json:"edge,omitempty"`
}

// Path는 graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
//
// Path는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
// 세부 조건은 GraphML, NDJSON, CSV, Neo4j 계약과 caller-owned graph model을 따른다.
type Path struct {
	steps       []PathStep
	totalWeight float64
}

// EmptyPath는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func EmptyPath() Path {
	return Path{}
}

// NewPath는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
// 이 주석은 graph format, backend requirement, traversal, serialization 조건을 설명한다.
func NewPath(steps ...PathStep) (Path, error) {
	weight := 0.0
	for _, step := range steps {
		if step.IsEdge() {
			weight++
		}
	}
	return NewWeightedPath(weight, steps...)
}

// NewWeightedPath는 graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
// 이 주석은 graph format, backend requirement, traversal, serialization 조건을 설명한다.
func NewWeightedPath(weight float64, steps ...PathStep) (Path, error) {
	if !validWeight(weight) {
		return Path{}, validationError(ErrInvalidPath, "total_weight", "invalid path weight", nil)
	}
	copied := make([]PathStep, len(steps))
	for i, step := range steps {
		if err := step.Validate(); err != nil {
			return Path{}, err
		}
		copied[i] = step
	}
	return Path{steps: copied, totalWeight: weight}, nil
}

// Steps는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) Steps() []PathStep {
	if len(path.steps) == 0 {
		return nil
	}
	copied := make([]PathStep, len(path.steps))
	copy(copied, path.steps)
	return copied
}

// Vertices는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) Vertices() []Vertex {
	if len(path.steps) == 0 {
		return nil
	}
	vertices := make([]Vertex, 0, len(path.steps))
	for _, step := range path.steps {
		if vertex, ok := step.Vertex(); ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

// Edges는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) Edges() []Edge {
	if len(path.steps) == 0 {
		return nil
	}
	edges := make([]Edge, 0, len(path.steps))
	for _, step := range path.steps {
		if edge, ok := step.Edge(); ok {
			edges = append(edges, edge)
		}
	}
	return edges
}

// Length는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) Length() int {
	count := 0
	for _, step := range path.steps {
		if step.IsEdge() {
			count++
		}
	}
	return count
}

// TotalWeight는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) TotalWeight() float64 {
	return path.totalWeight
}

// IsEmpty는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (path Path) IsEmpty() bool {
	return len(path.steps) == 0
}

// Validate는 graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
// 이 주석은 graph format, backend requirement, traversal, serialization 조건을 설명한다.
func (path Path) Validate() error {
	if !validWeight(path.totalWeight) {
		return validationError(ErrInvalidPath, "total_weight", "invalid path weight", nil)
	}
	for _, step := range path.steps {
		if err := step.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// MarshalJSON는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (path Path) MarshalJSON() ([]byte, error) {
	if err := path.Validate(); err != nil {
		return nil, err
	}
	steps := path.steps
	if steps == nil {
		steps = []PathStep{}
	}
	return json.Marshal(pathJSON{
		Steps:       steps,
		TotalWeight: path.totalWeight,
	})
}

// UnmarshalJSON는 graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (path *Path) UnmarshalJSON(data []byte) error {
	var decoded pathDecodeJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return validationError(ErrInvalidPath, "json", "decode failed", err)
	}
	if decoded.Steps == nil {
		return validationError(ErrInvalidPath, "steps", "missing steps", nil)
	}
	if decoded.TotalWeight == nil {
		return validationError(ErrInvalidPath, "total_weight", "missing path weight", nil)
	}
	created, err := NewWeightedPath(*decoded.TotalWeight, *decoded.Steps...)
	if err != nil {
		return err
	}
	*path = created
	return nil
}

type pathJSON struct {
	Steps       []PathStep `json:"steps"`
	TotalWeight float64    `json:"total_weight"`
}

type pathDecodeJSON struct {
	Steps       *[]PathStep `json:"steps"`
	TotalWeight *float64    `json:"total_weight"`
}

func validWeight(weight float64) bool {
	return weight >= 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0)
}
