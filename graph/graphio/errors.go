package graphio

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	// ErrInvalidRecord graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidRecord = errors.New("invalid graphio record")
	// ErrInvalidOptions graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrInvalidOptions = errors.New("invalid graphio options")
	// ErrDuplicateVertex graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrDuplicateVertex = errors.New("duplicate graph vertex")
	// ErrMissingEndpoint graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrMissingEndpoint = errors.New("missing graph edge endpoint")
	// ErrMalformedInput graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrMalformedInput = errors.New("malformed graph input")
	// ErrStreamClosed graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
	ErrStreamClosed = errors.New("graphio stream closed")
)

// Phase graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Phase string

const (
	// PhaseReadVertex graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	PhaseReadVertex Phase = "read_vertex"
	// PhaseReadEdge graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	PhaseReadEdge Phase = "read_edge"
	// PhaseWriteVertex graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	PhaseWriteVertex Phase = "write_vertex"
	// PhaseWriteEdge graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	PhaseWriteEdge Phase = "write_edge"
	// PhaseValidate graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	PhaseValidate Phase = "validate"
)

// Severity graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type Severity string

const (
	// SeverityError graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	SeverityError Severity = "error"
	// SeverityWarning graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	SeverityWarning Severity = "warning"
)

// FileRole graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type FileRole string

const (
	// FileRoleVertices graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	FileRoleVertices FileRole = "vertices"
	// FileRoleEdges graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	FileRoleEdges FileRole = "edges"
	// FileRoleStream graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	FileRoleStream FileRole = "stream"
)

// Location graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Location struct {
	Line     int64
	Row      int64
	Column   string
	FileRole FileRole
}

// Error graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type Error struct {
	Kind     error
	Format   Format
	Phase    Phase
	Location Location
	Field    string
	RecordID string
	Summary  string
	Cause    error
}

// NewError graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewError(kind error, format Format, phase Phase, location Location, field string, recordID string, summary string, cause error) *Error {
	return &Error{
		Kind:     kind,
		Format:   format,
		Phase:    phase,
		Location: location,
		Field:    field,
		RecordID: redactID(recordID),
		Summary:  summary,
		Cause:    cause,
	}
}

// Error graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	kind := "graphio error"
	if e.Kind != nil {
		kind = e.Kind.Error()
	}
	parts := []string{kind}
	if e.Format != "" {
		parts = append(parts, "format "+string(e.Format))
	}
	if e.Phase != "" {
		parts = append(parts, "phase "+string(e.Phase))
	}
	if e.Field != "" {
		parts = append(parts, "field "+e.Field)
	}
	if e.Summary != "" {
		parts = append(parts, e.Summary)
	}
	return strings.Join(parts, ": ")
}

// Unwrap graph IO Neo4j backend에서 제공하는 기능과 사용 경계를 설명한다.
func (e *Error) Unwrap() []error {
	if e == nil {
		return nil
	}
	errs := make([]error, 0, 2)
	if e.Kind != nil {
		errs = append(errs, e.Kind)
	}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

func optionError(summary string) error {
	return NewError(ErrInvalidOptions, "", PhaseValidate, Location{}, "", "", summary, nil)
}

func redactID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "key") {
		return "<redacted>"
	}
	if len(value) > 64 {
		sum := sha256.Sum256([]byte(value))
		return "sha256:" + hex.EncodeToString(sum[:8])
	}
	return value
}

func wrap(kind error, format Format, phase Phase, loc Location, field string, recordID string, summary string, cause error) error {
	return NewError(kind, format, phase, loc, field, recordID, summary, cause)
}
