package audit

import (
	"encoding/json"
	"math"
	"strings"
)

// AggregateID identifies one aggregate root instance.
type AggregateID struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// NewAggregateID creates a validated aggregate ID.
func NewAggregateID(typ string, id string) (AggregateID, error) {
	aggregate := AggregateID{
		Type: strings.TrimSpace(typ),
		ID:   strings.TrimSpace(id),
	}
	if err := aggregate.Validate(); err != nil {
		return AggregateID{}, err
	}
	return aggregate, nil
}

// Validate checks that both aggregate type and instance ID are present.
func (id AggregateID) Validate() error {
	if strings.TrimSpace(id.Type) == "" {
		return validationError(ErrInvalidAggregateID, "type", id.Type)
	}
	if strings.TrimSpace(id.ID) == "" {
		return validationError(ErrInvalidAggregateID, "id", id.ID)
	}
	return nil
}

// String returns the aggregate ID in type:id form.
func (id AggregateID) String() string {
	return id.Type + ":" + id.ID
}

// UnmarshalJSON decodes and validates an aggregate ID.
func (id *AggregateID) UnmarshalJSON(data []byte) error {
	type aggregateID AggregateID
	var decoded aggregateID
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := NewAggregateID(decoded.Type, decoded.ID)
	if err != nil {
		return err
	}
	*id = value
	return nil
}

// Revision is a positive aggregate event sequence number.
type Revision uint64

// InitialRevision returns the first durable aggregate revision.
func InitialRevision() Revision {
	return Revision(1)
}

// Validate checks that the revision is positive.
func (r Revision) Validate() error {
	if r == 0 {
		return validationError(ErrInvalidRevision, "revision", r)
	}
	return nil
}

// Next returns the next revision or an error when the revision overflows.
func (r Revision) Next() (Revision, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	if uint64(r) == math.MaxUint64 {
		return 0, validationError(ErrInvalidRevision, "revision", r)
	}
	return r + 1, nil
}

// Metadata is string key/value metadata copied by constructors.
type Metadata map[string]string

// Clone returns a defensive copy of metadata.
func (m Metadata) Clone() Metadata {
	if len(m) == 0 {
		return nil
	}
	clone := make(Metadata, len(m))
	for key, value := range m {
		clone[key] = value
	}
	return clone
}

func validateMetadata(kind error, field string, metadata Metadata) error {
	for key := range metadata {
		if strings.TrimSpace(key) == "" {
			return validationError(kind, field, key)
		}
	}
	return nil
}

func cloneRawMessage(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return nil
	}
	clone := make(json.RawMessage, len(payload))
	copy(clone, payload)
	return clone
}

func normalizePayload(field string, payload json.RawMessage, kind error) (json.RawMessage, error) {
	clone := payload
	if len(clone) == 0 {
		clone = json.RawMessage(`{}`)
	}
	clone = cloneRawMessage(clone)
	if !json.Valid(clone) {
		return nil, validationError(kind, field, string(clone))
	}
	return clone, nil
}
