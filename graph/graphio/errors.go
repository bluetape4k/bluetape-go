package graphio

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	// ErrInvalidRecord reports an invalid graphio record.
	ErrInvalidRecord = errors.New("invalid graphio record")
	// ErrInvalidOptions reports invalid graphio options.
	ErrInvalidOptions = errors.New("invalid graphio options")
	// ErrDuplicateVertex reports a duplicate vertex ID.
	ErrDuplicateVertex = errors.New("duplicate graph vertex")
	// ErrMissingEndpoint reports an edge whose endpoints were not seen.
	ErrMissingEndpoint = errors.New("missing graph edge endpoint")
	// ErrMalformedInput reports malformed graph input.
	ErrMalformedInput = errors.New("malformed graph input")
	// ErrStreamClosed reports use of a reader or writer after Close.
	ErrStreamClosed = errors.New("graphio stream closed")
)

// Phase describes where a graphio failure occurred.
type Phase string

const (
	// PhaseReadVertex identifies vertex read failures.
	PhaseReadVertex Phase = "read_vertex"
	// PhaseReadEdge identifies edge read failures.
	PhaseReadEdge Phase = "read_edge"
	// PhaseWriteVertex identifies vertex write failures.
	PhaseWriteVertex Phase = "write_vertex"
	// PhaseWriteEdge identifies edge write failures.
	PhaseWriteEdge Phase = "write_edge"
	// PhaseValidate identifies validation failures.
	PhaseValidate Phase = "validate"
)

// Severity describes whether a report entry is fatal or informational.
type Severity string

const (
	// SeverityError identifies a fatal failure.
	SeverityError Severity = "error"
	// SeverityWarning identifies a non-fatal report entry.
	SeverityWarning Severity = "warning"
)

// FileRole identifies the stream where an input or output failure occurred.
type FileRole string

const (
	// FileRoleVertices identifies a vertex CSV stream.
	FileRoleVertices FileRole = "vertices"
	// FileRoleEdges identifies an edge CSV stream.
	FileRoleEdges FileRole = "edges"
	// FileRoleStream identifies a single-stream format.
	FileRoleStream FileRole = "stream"
)

// Location describes a redacted stream position.
type Location struct {
	Line     int64
	Row      int64
	Column   string
	FileRole FileRole
}

// Error describes a redacted graphio failure.
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

// NewError creates a redacted graphio error.
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

// Error returns a redacted failure message.
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

// Unwrap exposes the sentinel kind and optional cause to errors.Is/As.
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
