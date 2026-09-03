package eventbridge

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	awseventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	awstypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
)

const (
	defaultMaxDetailSize = 256 << 10
	maxEventEntrySize    = 256 << 10
	maxSourceBytes       = 256
	maxDetailTypeBytes   = 128
	maxEventBusNameBytes = 256
)

// Client - Publisher가 사용하는 좁은 EventBridge SDK surface다.
type Client interface {
	PutEvents(context.Context, *awseventbridge.PutEventsInput, ...func(*awseventbridge.Options)) (*awseventbridge.PutEventsOutput, error)
}

// Options - EventBridge publisher 생성에 필요한 caller-owned 값이다.
type Options struct {
	// Client는 호출자가 생성·수명·retry 정책을 소유하는 EventBridge client다.
	Client Client
	// EventBusName은 비어 있으면 AWS default event bus를 사용한다.
	EventBusName string
	// Source는 EventBridge event source이며 빈 값일 수 없다.
	Source string
	// DetailType은 EventBridge event detail type이며 빈 값일 수 없다.
	DetailType string
	// MaxDetailSize는 encoded JSON detail의 최대 bytes다. 0이면 256 KiB다.
	MaxDetailSize int
}

// Publisher - sqloutbox record를 단일 EventBridge entry로 전달한다.
type Publisher struct {
	client        Client
	eventBusName  string
	source        string
	detailType    string
	maxDetailSize int
}

var _ sqloutbox.Publisher = (*Publisher)(nil)
var _ Client = (*awseventbridge.Client)(nil)

// New - caller-owned EventBridge client와 immutable publish 설정을 검증한다.
func New(options Options) (*Publisher, error) {
	if isNilClient(options.Client) {
		return nil, ErrNilClient
	}
	if err := validateString(options.Source, maxSourceBytes, true); err != nil {
		return nil, err
	}
	if err := validateString(options.DetailType, maxDetailTypeBytes, true); err != nil {
		return nil, err
	}
	if options.EventBusName != "" {
		if err := validateString(options.EventBusName, maxEventBusNameBytes, true); err != nil {
			return nil, err
		}
	}
	maxDetailSize := options.MaxDetailSize
	if maxDetailSize == 0 {
		maxDetailSize = defaultMaxDetailSize
	}
	if maxDetailSize < 0 || maxDetailSize > maxEventEntrySize {
		return nil, ErrInvalidOptions
	}
	return &Publisher{
		client:        options.Client,
		eventBusName:  options.EventBusName,
		source:        options.Source,
		detailType:    options.DetailType,
		maxDetailSize: maxDetailSize,
	}, nil
}

// EventBusName - publisher가 사용할 custom bus 이름 또는 빈 default bus 값을 반환한다.
func (p *Publisher) EventBusName() string {
	if p == nil {
		return ""
	}
	return p.eventBusName
}

// Source - publisher가 EventBridge request에 전달할 source를 반환한다.
func (p *Publisher) Source() string {
	if p == nil {
		return ""
	}
	return p.source
}

// DetailType - publisher가 EventBridge request에 전달할 detail type을 반환한다.
func (p *Publisher) DetailType() string {
	if p == nil {
		return ""
	}
	return p.detailType
}

// Publish - record를 검증된 JSON detail의 단일 EventBridge entry로 전송한다.
func (p *Publisher) Publish(ctx context.Context, record sqloutbox.Record) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || isNilClient(p.client) || p.source == "" || p.detailType == "" || p.maxDetailSize <= 0 {
		return newError(ErrInvalidOptions, "validate options", nil, 0, "")
	}
	detail, err := p.detail(record)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	requestEntry := awstypes.PutEventsRequestEntry{
		Detail:     &detail,
		DetailType: &p.detailType,
		Source:     &p.source,
		Time:       timePointer(record.OccurredAt),
	}
	if p.eventBusName != "" {
		requestEntry.EventBusName = &p.eventBusName
	}
	input := &awseventbridge.PutEventsInput{Entries: []awstypes.PutEventsRequestEntry{requestEntry}}
	if err := ctx.Err(); err != nil {
		return err
	}
	output, callErr := p.client.PutEvents(ctx, input)
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return newError(ErrPublishFailed, "publish", callErr, 0, "")
	}
	if output == nil || len(output.Entries) != 1 || output.FailedEntryCount < 0 || output.FailedEntryCount > 1 {
		return newError(ErrMalformedOutput, "publish", nil, 0, "")
	}
	result := output.Entries[0]
	if output.FailedEntryCount > 0 ||
		result.ErrorCode != nil && *result.ErrorCode != "" ||
		result.ErrorMessage != nil && *result.ErrorMessage != "" {
		failureCount := output.FailedEntryCount
		if failureCount == 0 {
			failureCount = 1
		}
		code := ""
		if result.ErrorCode != nil {
			code = safeErrorCode(*result.ErrorCode)
		}
		return newError(ErrPartialFailure, "publish", nil, failureCount, code)
	}
	if result.EventId == nil || strings.TrimSpace(*result.EventId) == "" {
		return newError(ErrMalformedOutput, "publish", nil, 0, "")
	}
	return nil
}

type detailEnvelope struct {
	RecordID       int64           `json:"record_id"`
	Status         string          `json:"status"`
	AggregateType  string          `json:"aggregate_type"`
	AggregateID    string          `json:"aggregate_id"`
	Revision       uint64          `json:"revision"`
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	EventType      string          `json:"event_type"`
	OccurredAt     string          `json:"occurred_at"`
	RecordedAt     string          `json:"recorded_at"`
	SchemaVersion  int             `json:"schema_version"`
	Attempts       int             `json:"attempts"`
	EntryJSON      json.RawMessage `json:"entry_json"`
}

func (p *Publisher) detail(record sqloutbox.Record) (string, error) {
	if err := validateRecord(record); err != nil {
		return "", err
	}
	if !entryInputWithinLimit(record.Entry, p.maxDetailSize) {
		return "", ErrDetailTooLarge
	}
	entryJSON, err := json.Marshal(record.Entry)
	if err != nil {
		return "", newError(ErrInvalidRecord, "marshal detail", nil, 0, "")
	}
	envelope := detailEnvelope{
		RecordID:       int64(record.ID),
		Status:         string(record.Status),
		AggregateType:  record.Aggregate.Type,
		AggregateID:    record.Aggregate.ID,
		Revision:       uint64(record.Revision),
		EventID:        string(record.EventID),
		IdempotencyKey: record.IdempotencyKey,
		EventType:      string(record.EventType),
		OccurredAt:     record.OccurredAt.UTC().Format(timeFormatRFC3339Nano),
		RecordedAt:     record.RecordedAt.UTC().Format(timeFormatRFC3339Nano),
		SchemaVersion:  record.SchemaVersion,
		Attempts:       record.Attempts,
		EntryJSON:      json.RawMessage(entryJSON),
	}
	detailJSON, err := json.Marshal(envelope)
	if err != nil {
		return "", newError(ErrInvalidRecord, "marshal detail", nil, 0, "")
	}
	detailSize := len(detailJSON)
	if detailSize > p.maxDetailSize || detailSize+len(p.source)+len(p.detailType)+len(p.eventBusName) >= maxEventEntrySize {
		return "", ErrDetailTooLarge
	}
	return string(detailJSON), nil
}

func validateRecord(record sqloutbox.Record) error {
	if record.ID <= 0 || record.Attempts <= 0 || !utf8.ValidString(string(record.Status)) {
		return ErrInvalidRecord
	}
	if !validEntryUTF8(record.Entry) {
		return ErrInvalidRecord
	}
	if err := record.Entry.Validate(); err != nil {
		return ErrInvalidRecord
	}
	entry := record.Entry
	if record.Aggregate != entry.Aggregate ||
		record.Revision != entry.Revision ||
		record.EventID != entry.Event.EventID ||
		record.IdempotencyKey != entry.Event.IdempotencyKey ||
		record.EventType != entry.Event.EventType ||
		record.SchemaVersion != entry.SchemaVersion ||
		!record.OccurredAt.Equal(entry.Event.OccurredAt) ||
		!record.RecordedAt.Equal(entry.Event.RecordedAt) {
		return ErrInvalidRecord
	}
	return nil
}

func entryInputWithinLimit(entry audit.Entry, limit int) bool {
	if limit <= 0 {
		return false
	}
	total := 0
	add := func(size int) bool {
		if size < 0 || total > limit-size {
			return false
		}
		total += size
		return true
	}
	if !add(len(entry.Event.Payload)) || !add(len(entry.Event.EventID)) || !add(len(entry.Event.EventType)) ||
		!add(len(entry.Event.IdempotencyKey)) || !add(len(entry.Event.Aggregate.Type)) || !add(len(entry.Event.Aggregate.ID)) ||
		!add(len(entry.Aggregate.Type)) || !add(len(entry.Aggregate.ID)) || !add(len(entry.Author)) {
		return false
	}
	for key, value := range entry.Event.Metadata {
		if !add(len(key)) || !add(len(value)) {
			return false
		}
	}
	if entry.Snapshot != nil {
		if !add(len(entry.Snapshot.Format)) || !add(len(entry.Snapshot.SchemaVersion)) || !add(len(entry.Snapshot.Payload)) {
			return false
		}
	}
	if entry.Change != nil {
		if !add(len(entry.Change.Summary)) {
			return false
		}
		for _, field := range entry.Change.ChangedFields {
			if !add(len(field)) {
				return false
			}
		}
		for key, value := range entry.Change.Attributes {
			if !add(len(key)) || !add(len(value)) {
				return false
			}
		}
	}
	return true
}

func validEntryUTF8(entry audit.Entry) bool {
	values := []string{
		entry.Aggregate.Type,
		entry.Aggregate.ID,
		entry.Author,
		string(entry.Event.EventID),
		string(entry.Event.EventType),
		entry.Event.Aggregate.Type,
		entry.Event.Aggregate.ID,
		entry.Event.IdempotencyKey,
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	for key, value := range entry.Event.Metadata {
		if !utf8.ValidString(key) || !utf8.ValidString(value) {
			return false
		}
	}
	if entry.Snapshot != nil {
		if !utf8.ValidString(entry.Snapshot.Format) || !utf8.ValidString(entry.Snapshot.SchemaVersion) {
			return false
		}
	}
	if entry.Change != nil {
		if !utf8.ValidString(entry.Change.Summary) {
			return false
		}
		for _, field := range entry.Change.ChangedFields {
			if !utf8.ValidString(field) {
				return false
			}
		}
		for key, value := range entry.Change.Attributes {
			if !utf8.ValidString(key) || !utf8.ValidString(value) {
				return false
			}
		}
	}
	return true
}

func validateString(value string, maxBytes int, rejectBlank bool) error {
	if !utf8.ValidString(value) || len(value) > maxBytes || rejectBlank && strings.TrimSpace(value) == "" {
		return ErrInvalidOptions
	}
	return nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isNilClient(client Client) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func timePointer(value time.Time) *time.Time {
	clone := value
	return &clone
}

const timeFormatRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
