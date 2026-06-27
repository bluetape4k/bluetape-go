package audit

import (
	"encoding/json"
	"sort"
	"strings"
)

// SchemaVersion is the supported audit entry JSON schema version.
const SchemaVersion = 1

// SnapshotMetadata describes an optional serialized aggregate snapshot.
type SnapshotMetadata struct {
	Format        string          `json:"format"`
	SchemaVersion string          `json:"schema_version"`
	Payload       json.RawMessage `json:"payload"`
}

// Validate checks required snapshot metadata fields.
func (m SnapshotMetadata) Validate() error {
	if strings.TrimSpace(m.Format) == "" {
		return validationError(ErrInvalidEntry, "snapshot.format", m.Format)
	}
	if strings.TrimSpace(m.SchemaVersion) == "" {
		return validationError(ErrInvalidEntry, "snapshot.schema_version", m.SchemaVersion)
	}
	if len(m.Payload) == 0 || !json.Valid(m.Payload) {
		return validationError(ErrInvalidEntry, "snapshot.payload", string(m.Payload))
	}
	return nil
}

// Clone returns a defensive copy of snapshot metadata.
func (m SnapshotMetadata) Clone() SnapshotMetadata {
	clone := m
	clone.Payload = cloneRawMessage(m.Payload)
	return clone
}

// UnmarshalJSON decodes and validates snapshot metadata.
func (m *SnapshotMetadata) UnmarshalJSON(data []byte) error {
	type snapshotMetadata SnapshotMetadata
	var decoded snapshotMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value := SnapshotMetadata(decoded).Clone()
	value.Format = strings.TrimSpace(value.Format)
	value.SchemaVersion = strings.TrimSpace(value.SchemaVersion)
	if err := value.Validate(); err != nil {
		return err
	}
	*m = value
	return nil
}

// ChangeMetadata describes optional changed-field metadata for an audit entry.
type ChangeMetadata struct {
	ChangedFields []string `json:"changed_fields"`
	Summary       string   `json:"summary,omitempty"`
	Attributes    Metadata `json:"attributes,omitempty"`
}

// NewChangeMetadata creates normalized change metadata.
func NewChangeMetadata(fields []string, summary string, attributes Metadata) (ChangeMetadata, error) {
	seen := make(map[string]struct{}, len(fields))
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return ChangeMetadata{}, validationError(ErrInvalidEntry, "change.changed_fields", field)
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		normalized = append(normalized, field)
	}
	sort.Strings(normalized)
	change := ChangeMetadata{
		ChangedFields: normalized,
		Summary:       strings.TrimSpace(summary),
		Attributes:    attributes.Clone(),
	}
	if err := change.Validate(); err != nil {
		return ChangeMetadata{}, err
	}
	return change, nil
}

// Validate checks required change metadata fields.
func (m ChangeMetadata) Validate() error {
	if len(m.ChangedFields) == 0 {
		return validationError(ErrInvalidEntry, "change.changed_fields", m.ChangedFields)
	}
	seen := make(map[string]struct{}, len(m.ChangedFields))
	for _, field := range m.ChangedFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return validationError(ErrInvalidEntry, "change.changed_fields", field)
		}
		if _, ok := seen[field]; ok {
			return validationError(ErrRevisionConflict, "change.changed_fields", field)
		}
		seen[field] = struct{}{}
	}
	if err := validateMetadata(ErrInvalidEntry, "change.attributes", m.Attributes); err != nil {
		return err
	}
	return nil
}

// Clone returns a defensive copy of change metadata.
func (m ChangeMetadata) Clone() ChangeMetadata {
	clone := ChangeMetadata{
		Summary:    m.Summary,
		Attributes: m.Attributes.Clone(),
	}
	if len(m.ChangedFields) > 0 {
		clone.ChangedFields = append([]string(nil), m.ChangedFields...)
	}
	return clone
}

// UnmarshalJSON decodes and validates change metadata.
func (m *ChangeMetadata) UnmarshalJSON(data []byte) error {
	type changeMetadata ChangeMetadata
	var decoded changeMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := NewChangeMetadata(decoded.ChangedFields, decoded.Summary, decoded.Attributes)
	if err != nil {
		return err
	}
	*m = value
	return nil
}

// EntryOptions contains validated inputs for NewEntry.
type EntryOptions struct {
	Author   string
	Event    DomainEvent
	Snapshot *SnapshotMetadata
	Change   *ChangeMetadata
}

// Entry is one immutable audit log record.
type Entry struct {
	SchemaVersion int               `json:"schema_version"`
	Aggregate     AggregateID       `json:"aggregate"`
	Revision      Revision          `json:"revision"`
	Author        string            `json:"author"`
	Event         DomainEvent       `json:"event"`
	Snapshot      *SnapshotMetadata `json:"snapshot,omitempty"`
	Change        *ChangeMetadata   `json:"change,omitempty"`
}

// NewEntry creates a validated audit entry from a domain event.
func NewEntry(options EntryOptions) (Entry, error) {
	entry := Entry{
		SchemaVersion: SchemaVersion,
		Aggregate:     options.Event.Aggregate,
		Revision:      options.Event.Revision,
		Author:        strings.TrimSpace(options.Author),
		Event:         options.Event.Clone(),
	}
	if options.Snapshot != nil {
		snapshot := options.Snapshot.Clone()
		entry.Snapshot = &snapshot
	}
	if options.Change != nil {
		change := options.Change.Clone()
		entry.Change = &change
	}
	if err := entry.Validate(); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Validate checks the audit entry schema, author, event, and consistency.
func (e Entry) Validate() error {
	if e.SchemaVersion != SchemaVersion {
		return validationError(ErrInvalidEntry, "schema_version", e.SchemaVersion)
	}
	if err := e.Aggregate.Validate(); err != nil {
		return validationCause(ErrInvalidEntry, "aggregate", e.Aggregate, err)
	}
	if err := e.Revision.Validate(); err != nil {
		return validationCause(ErrInvalidEntry, "revision", e.Revision, err)
	}
	if strings.TrimSpace(e.Author) == "" {
		return validationError(ErrInvalidEntry, "author", e.Author)
	}
	if err := e.Event.Validate(); err != nil {
		return validationCause(ErrInvalidEntry, "event", e.Event, err)
	}
	if e.Event.Aggregate != e.Aggregate {
		return validationError(ErrInvalidEntry, "event.aggregate", e.Event.Aggregate)
	}
	if e.Event.Revision != e.Revision {
		return validationError(ErrInvalidEntry, "event.revision", e.Event.Revision)
	}
	if e.Snapshot != nil {
		if err := e.Snapshot.Validate(); err != nil {
			return err
		}
	}
	if e.Change != nil {
		if err := e.Change.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Clone returns a defensive copy of the audit entry.
func (e Entry) Clone() Entry {
	clone := e
	clone.Event = e.Event.Clone()
	if e.Snapshot != nil {
		snapshot := e.Snapshot.Clone()
		clone.Snapshot = &snapshot
	}
	if e.Change != nil {
		change := e.Change.Clone()
		clone.Change = &change
	}
	return clone
}

// UnmarshalJSON decodes and validates an audit entry.
func (e *Entry) UnmarshalJSON(data []byte) error {
	type auditEntry Entry
	var decoded auditEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value := Entry(decoded).Clone()
	value.Author = strings.TrimSpace(value.Author)
	if err := value.Validate(); err != nil {
		return err
	}
	*e = value
	return nil
}

// DecodeEntryJSON decodes already-bounded JSON bytes into an audit entry.
func DecodeEntryJSON(data []byte) (Entry, error) {
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}
