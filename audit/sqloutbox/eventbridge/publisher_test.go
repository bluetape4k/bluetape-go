package eventbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	awseventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	awstypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
)

var (
	testClock          = time.Date(2026, 9, 3, 2, 0, 0, 0, time.UTC)
	testEventID        = audit.EventID("event-7")
	testIdempotencyKey = "invoice:42:7"
)

type fakeClient struct {
	mu      sync.Mutex
	calls   int
	last    *awseventbridge.PutEventsInput
	lastCtx context.Context
	output  *awseventbridge.PutEventsOutput
	err     error
	entered chan struct{}
	release chan struct{}
	after   func()
}

func (f *fakeClient) PutEvents(ctx context.Context, input *awseventbridge.PutEventsInput, _ ...func(*awseventbridge.Options)) (*awseventbridge.PutEventsOutput, error) {
	f.mu.Lock()
	f.calls++
	f.last = cloneInput(input)
	f.lastCtx = ctx
	output := cloneOutput(f.output)
	err := f.err
	entered := f.entered
	release := f.release
	after := f.after
	f.mu.Unlock()

	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return output, ctx.Err()
		}
	}
	if after != nil {
		after()
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func (f *fakeClient) snapshot() (int, *awseventbridge.PutEventsInput) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, cloneInput(f.last)
}

func (f *fakeClient) context() context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastCtx
}

func cloneInput(input *awseventbridge.PutEventsInput) *awseventbridge.PutEventsInput {
	if input == nil {
		return nil
	}
	clone := &awseventbridge.PutEventsInput{EndpointId: cloneString(input.EndpointId)}
	clone.Entries = make([]awstypes.PutEventsRequestEntry, len(input.Entries))
	for i, entry := range input.Entries {
		clone.Entries[i] = entry
		clone.Entries[i].Detail = cloneString(entry.Detail)
		clone.Entries[i].DetailType = cloneString(entry.DetailType)
		clone.Entries[i].EventBusName = cloneString(entry.EventBusName)
		clone.Entries[i].Source = cloneString(entry.Source)
		if entry.Time != nil {
			value := *entry.Time
			clone.Entries[i].Time = &value
		}
		clone.Entries[i].Resources = append([]string(nil), entry.Resources...)
		clone.Entries[i].TraceHeader = cloneString(entry.TraceHeader)
	}
	return clone
}

func cloneOutput(output *awseventbridge.PutEventsOutput) *awseventbridge.PutEventsOutput {
	if output == nil {
		return nil
	}
	clone := &awseventbridge.PutEventsOutput{FailedEntryCount: output.FailedEntryCount}
	clone.Entries = make([]awstypes.PutEventsResultEntry, len(output.Entries))
	for i, entry := range output.Entries {
		clone.Entries[i] = entry
		clone.Entries[i].ErrorCode = cloneString(entry.ErrorCode)
		clone.Entries[i].ErrorMessage = cloneString(entry.ErrorMessage)
		clone.Entries[i].EventId = cloneString(entry.EventId)
	}
	return clone
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func successOutput() *awseventbridge.PutEventsOutput {
	eventID := "aws-event-id"
	return &awseventbridge.PutEventsOutput{
		Entries: []awstypes.PutEventsResultEntry{{EventId: &eventID}},
	}
}

func TestNewRejectsInvalidOptions(t *testing.T) {
	fake := &fakeClient{}
	tests := []struct {
		name    string
		options Options
	}{
		{name: "nil client", options: Options{}},
		{name: "typed nil client", options: Options{Client: (*fakeClient)(nil), Source: "app", DetailType: "audit"}},
		{name: "blank source", options: Options{Client: fake, Source: "  ", DetailType: "audit"}},
		{name: "blank detail type", options: Options{Client: fake, Source: "app", DetailType: "\t"}},
		{name: "invalid source utf8", options: Options{Client: fake, Source: string([]byte{0xff}), DetailType: "audit"}},
		{name: "source too long", options: Options{Client: fake, Source: strings.Repeat("x", 257), DetailType: "audit"}},
		{name: "detail type too long", options: Options{Client: fake, Source: "app", DetailType: strings.Repeat("x", 129)}},
		{name: "event bus too long", options: Options{Client: fake, EventBusName: strings.Repeat("x", 257), Source: "app", DetailType: "audit"}},
		{name: "negative detail size", options: Options{Client: fake, Source: "app", DetailType: "audit", MaxDetailSize: -1}},
		{name: "detail size over aws limit", options: Options{Client: fake, Source: "app", DetailType: "audit", MaxDetailSize: (256 << 10) + 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.options); !errors.Is(err, ErrInvalidOptions) && !errors.Is(err, ErrNilClient) {
				t.Fatalf("New error = %v, want invalid options or nil client", err)
			}
		})
	}
}

func TestNewDefaultsAndPreservesOptions(t *testing.T) {
	const bus = " arn:aws:events:ap-northeast-2:123456789012:event-bus/audit "
	publisher, err := New(Options{
		Client:       &fakeClient{},
		EventBusName: bus,
		Source:       " com.example.audit ",
		DetailType:   " AuditRecorded ",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if publisher.EventBusName() != bus || publisher.Source() != " com.example.audit " || publisher.DetailType() != " AuditRecorded " {
		t.Fatalf("publisher did not preserve exact caller values: bus=%q source=%q detail_type=%q", publisher.EventBusName(), publisher.Source(), publisher.DetailType())
	}
	if publisher.maxDetailSize != 256<<10 {
		t.Fatalf("max detail size = %d, want %d", publisher.maxDetailSize, 256<<10)
	}

	defaultBus, err := New(Options{Client: &fakeClient{}, Source: "app", DetailType: "audit"})
	if err != nil {
		t.Fatalf("New default bus: %v", err)
	}
	if defaultBus.EventBusName() != "" {
		t.Fatalf("default EventBusName = %q, want empty", defaultBus.EventBusName())
	}
}

func TestPublishSuccessMapsSingleEntryAndStableDetail(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{EventBusName: "audit", Source: "com.example.audit", DetailType: "AuditRecorded"})
	record := testRecord(t, 42, 7, testEventID, testIdempotencyKey)

	if err := publisher.Publish(context.Background(), record); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	calls, input := client.snapshot()
	if calls != 1 || input == nil || len(input.Entries) != 1 {
		t.Fatalf("PutEvents calls/input/entries = %d/%#v/%d, want 1/non-nil/1", calls, input, len(input.Entries))
	}
	entry := input.Entries[0]
	if entry.EventBusName == nil || *entry.EventBusName != "audit" {
		t.Fatalf("event bus = %#v, want audit", entry.EventBusName)
	}
	if entry.Source == nil || *entry.Source != "com.example.audit" || entry.DetailType == nil || *entry.DetailType != "AuditRecorded" {
		t.Fatalf("source/detail type = %#v/%#v", entry.Source, entry.DetailType)
	}
	if entry.Time == nil || !entry.Time.Equal(record.OccurredAt) {
		t.Fatalf("time = %#v, want %v", entry.Time, record.OccurredAt)
	}
	if entry.Detail == nil || len(*entry.Detail) == 0 || !json.Valid([]byte(*entry.Detail)) {
		t.Fatalf("detail = %q, want valid JSON", stringValue(entry.Detail))
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(*entry.Detail), &detail); err != nil {
		t.Fatalf("detail decode: %v", err)
	}
	if detail["event_id"] != string(record.EventID) || detail["idempotency_key"] != record.IdempotencyKey {
		t.Fatalf("stable identity = %#v, want event/idempotency %q/%q", detail, record.EventID, record.IdempotencyKey)
	}
	if detail["record_id"] != float64(record.ID) || detail["aggregate_type"] != record.Aggregate.Type || detail["aggregate_id"] != record.Aggregate.ID {
		t.Fatalf("record metadata = %#v", detail)
	}
	var nested map[string]any
	if err := json.Unmarshal(detailJSON(t, detail, "entry_json"), &nested); err != nil {
		t.Fatalf("entry_json decode: %v", err)
	}
	if nested["event"].(map[string]any)["event_id"] != string(record.EventID) {
		t.Fatalf("entry_json lost event identity: %#v", nested)
	}
}

func TestPublishPropagatesCallerContext(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	ctx := context.WithValue(context.Background(), contextKey("request-id"), "req-520")
	if err := publisher.Publish(ctx, testRecord(t, 43, 1, "event-context", "idem-context")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := client.context().Value(contextKey("request-id")); got != "req-520" {
		t.Fatalf("context value = %v, want req-520", got)
	}
}

func TestPublishOmitsEventBusForDefaultBus(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	if err := publisher.Publish(context.Background(), testRecord(t, 1, 1, "event-default", "idem-default")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	_, input := client.snapshot()
	if input.Entries[0].EventBusName != nil {
		t.Fatalf("default bus pointer = %#v, want nil", input.Entries[0].EventBusName)
	}
}

func TestPublishRejectsInvalidRecordBeforeSDK(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sqloutbox.Record)
	}{
		{name: "zero id", mutate: func(record *sqloutbox.Record) { record.ID = 0 }},
		{name: "zero attempts", mutate: func(record *sqloutbox.Record) { record.Attempts = 0 }},
		{name: "entry invalid", mutate: func(record *sqloutbox.Record) { record.Entry.Author = "" }},
		{name: "aggregate mismatch", mutate: func(record *sqloutbox.Record) { record.Aggregate.ID = "different" }},
		{name: "event id mismatch", mutate: func(record *sqloutbox.Record) { record.EventID = "different" }},
		{name: "idempotency mismatch", mutate: func(record *sqloutbox.Record) { record.IdempotencyKey = "different" }},
		{name: "event type mismatch", mutate: func(record *sqloutbox.Record) { record.EventType = "different" }},
		{name: "schema mismatch", mutate: func(record *sqloutbox.Record) { record.SchemaVersion++ }},
		{name: "occurred at mismatch", mutate: func(record *sqloutbox.Record) { record.OccurredAt = record.OccurredAt.Add(time.Second) }},
		{name: "recorded at mismatch", mutate: func(record *sqloutbox.Record) { record.RecordedAt = record.RecordedAt.Add(time.Second) }},
		{name: "entry invalid utf8", mutate: func(record *sqloutbox.Record) {
			record.EventID = audit.EventID(string([]byte{0xff}))
			record.Entry.Event.EventID = record.EventID
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{output: successOutput()}
			publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
			record := testRecord(t, 2, 1, "event-invalid", "idem-invalid")
			tt.mutate(&record)
			if err := publisher.Publish(context.Background(), record); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("Publish error = %v, want ErrInvalidRecord", err)
			}
			calls, _ := client.snapshot()
			if calls != 0 {
				t.Fatalf("PutEvents calls = %d, want 0", calls)
			}
		})
	}
}

func TestPublishRejectsOversizedDetailBeforeSDK(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	record := testRecord(t, 3, 1, "event-large", "idem-large")
	record.Entry.Event.Payload = json.RawMessage(`"` + strings.Repeat("x", 300<<10) + `"`)
	if err := publisher.Publish(context.Background(), record); !errors.Is(err, ErrDetailTooLarge) {
		t.Fatalf("Publish error = %v, want ErrDetailTooLarge", err)
	}
	calls, _ := client.snapshot()
	if calls != 0 {
		t.Fatalf("PutEvents calls = %d, want 0", calls)
	}
}

func TestPublishPreservesPreDispatchCancellation(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publisher.Publish(ctx, testRecord(t, 4, 1, "event-cancel", "idem-cancel")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish error = %v, want context.Canceled", err)
	}
	calls, _ := client.snapshot()
	if calls != 0 {
		t.Fatalf("PutEvents calls = %d, want 0", calls)
	}
}

func TestPublishReturnsCancellationAfterSDKResponse(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	ctx, cancel := context.WithCancel(context.Background())
	client.after = cancel
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	if err := publisher.Publish(ctx, testRecord(t, 5, 1, "event-after-cancel", "idem-after-cancel")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish error = %v, want context.Canceled", err)
	}
}

func TestPublishRedactsTransportError(t *testing.T) {
	const secret = "customer-42-credential"
	injected := errors.New("AWS secret detail: " + secret)
	client := &fakeClient{output: successOutput(), err: injected}
	publisher := mustPublisher(t, client, Options{EventBusName: "arn:secret-bus", Source: "secret-source", DetailType: "secret-type"})
	err := publisher.Publish(context.Background(), testRecord(t, 6, 1, "event-secret", "idem-secret"))
	if !errors.Is(err, ErrPublishFailed) || !errors.Is(err, injected) {
		t.Fatalf("Publish error = %v, want publish sentinel and injected cause", err)
	}
	var operationErr *Error
	if !errors.As(err, &operationErr) {
		t.Fatalf("Publish error type = %T, want *Error", err)
	}
	formatted := err.Error() + fmt.Sprintf(" %+v", err)
	for _, forbidden := range []string{secret, "secret-source", "secret-type", "arn:secret-bus"} {
		if strings.Contains(formatted, forbidden) {
			t.Fatalf("transport error leaked %q: %q", forbidden, formatted)
		}
	}
}

func TestPublishMapsPartialFailureAndRedactsMessage(t *testing.T) {
	code := "InternalFailure"
	message := "raw customer-42 credentials"
	client := &fakeClient{
		output: &awseventbridge.PutEventsOutput{
			FailedEntryCount: 1,
			Entries:          []awstypes.PutEventsResultEntry{{ErrorCode: &code, ErrorMessage: &message}},
		},
	}
	publisher := mustPublisher(t, client, Options{Source: "secret-source", DetailType: "secret-type"})
	err := publisher.Publish(context.Background(), testRecord(t, 7, 1, "event-partial", "idem-partial"))
	if !errors.Is(err, ErrPartialFailure) {
		t.Fatalf("Publish error = %v, want ErrPartialFailure", err)
	}
	var operationErr *Error
	if !errors.As(err, &operationErr) {
		t.Fatalf("Publish error type = %T, want *Error", err)
	}
	if operationErr.FailureCount() != 1 || operationErr.ErrorCode() != code {
		t.Fatalf("failure metadata = %d/%q, want 1/%q", operationErr.FailureCount(), operationErr.ErrorCode(), code)
	}
	formatted := err.Error() + fmt.Sprintf(" %+v", err)
	for _, forbidden := range []string{message, "customer-42", "secret-source", "secret-type"} {
		if strings.Contains(formatted, forbidden) {
			t.Fatalf("partial error leaked %q: %q", forbidden, formatted)
		}
	}
}

func TestPublishMapsMessageOnlyFailure(t *testing.T) {
	message := "raw failure details"
	client := &fakeClient{output: &awseventbridge.PutEventsOutput{
		Entries: []awstypes.PutEventsResultEntry{{ErrorMessage: &message}},
	}}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	if err := publisher.Publish(context.Background(), testRecord(t, 11, 1, "event-message-only", "idem-message-only")); !errors.Is(err, ErrPartialFailure) {
		t.Fatalf("Publish error = %v, want ErrPartialFailure", err)
	}
}

func TestPublishReturnsOutputAndTransportErrorSafely(t *testing.T) {
	injected := errors.New("provider detail customer-42")
	client := &fakeClient{output: successOutput(), err: injected}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	err := publisher.Publish(context.Background(), testRecord(t, 12, 1, "event-output-error", "idem-output-error"))
	if !errors.Is(err, ErrPublishFailed) || !errors.Is(err, injected) {
		t.Fatalf("Publish error = %v, want publish and injected errors", err)
	}
	if strings.Contains(fmt.Sprintf("%+v", err), "customer-42") {
		t.Fatalf("output+transport error leaked provider text: %q", err)
	}
}

func TestPublishRejectsMalformedOutput(t *testing.T) {
	tests := []struct {
		name   string
		output *awseventbridge.PutEventsOutput
	}{
		{name: "nil output", output: nil},
		{name: "zero entries", output: &awseventbridge.PutEventsOutput{}},
		{name: "two entries", output: &awseventbridge.PutEventsOutput{Entries: make([]awstypes.PutEventsResultEntry, 2)}},
		{name: "invalid event id", output: &awseventbridge.PutEventsOutput{Entries: []awstypes.PutEventsResultEntry{{EventId: invalidStringPointer()}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{output: tt.output}
			publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
			if err := publisher.Publish(context.Background(), testRecord(t, 8, 1, "event-malformed", "idem-malformed")); !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("Publish error = %v, want ErrMalformedOutput", err)
			}
		})
	}
}

func TestPublishHandlesFailedCountWithoutEntryCode(t *testing.T) {
	client := &fakeClient{output: &awseventbridge.PutEventsOutput{
		FailedEntryCount: 1,
		Entries:          []awstypes.PutEventsResultEntry{{}},
	}}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	err := publisher.Publish(context.Background(), testRecord(t, 9, 1, "event-failed-count", "idem-failed-count"))
	if !errors.Is(err, ErrPartialFailure) {
		t.Fatalf("Publish error = %v, want ErrPartialFailure", err)
	}
	var operationErr *Error
	if !errors.As(err, &operationErr) || operationErr.FailureCount() != 1 {
		t.Fatalf("error = %#v, want failure count 1", operationErr)
	}
}

func TestPublishRejectsInconsistentFailureCount(t *testing.T) {
	client := &fakeClient{output: &awseventbridge.PutEventsOutput{
		FailedEntryCount: 2,
		Entries:          []awstypes.PutEventsResultEntry{{}},
	}}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	err := publisher.Publish(context.Background(), testRecord(t, 10, 1, "event-inconsistent", "idem-inconsistent"))
	if !errors.Is(err, ErrMalformedOutput) {
		t.Fatalf("Publish error = %v, want ErrMalformedOutput", err)
	}
}

func TestPublishConcurrentCallsAreSafe(t *testing.T) {
	client := &fakeClient{output: successOutput()}
	publisher := mustPublisher(t, client, Options{Source: "app", DetailType: "audit"})
	const workers = 32
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errCh <- publisher.Publish(context.Background(), testRecord(t, sqloutbox.RecordID(i+1), i+1, audit.EventID(fmt.Sprintf("event-%d", i)), fmt.Sprintf("idem-%d", i)))
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Publish: %v", err)
		}
	}
	calls, _ := client.snapshot()
	if calls != workers {
		t.Fatalf("PutEvents calls = %d, want %d", calls, workers)
	}
}

func mustPublisher(t *testing.T, client Client, options Options) *Publisher {
	t.Helper()
	options.Client = client
	publisher, err := New(options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return publisher
}

func testRecord(t *testing.T, id sqloutbox.RecordID, attempts int, eventID audit.EventID, idempotencyKey string) sqloutbox.Record {
	t.Helper()
	aggregate, err := audit.NewAggregateID("invoice", "42")
	if err != nil {
		t.Fatalf("NewAggregateID: %v", err)
	}
	revision := audit.Revision(7)
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        eventID,
		EventType:      audit.EventType("invoice.paid"),
		AggregateID:    aggregate,
		Revision:       revision,
		OccurredAt:     testClock,
		RecordedAt:     testClock.Add(time.Second),
		IdempotencyKey: idempotencyKey,
		Metadata:       audit.Metadata{"source": "test"},
		Payload:        json.RawMessage(`{"amount":100}`),
	})
	if err != nil {
		t.Fatalf("NewDomainEvent: %v", err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: "tester", Event: event})
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	return sqloutbox.Record{
		ID:             id,
		Status:         sqloutbox.StatusClaimed,
		Aggregate:      entry.Aggregate,
		Revision:       entry.Revision,
		EventID:        entry.Event.EventID,
		IdempotencyKey: entry.Event.IdempotencyKey,
		EventType:      entry.Event.EventType,
		OccurredAt:     entry.Event.OccurredAt,
		RecordedAt:     entry.Event.RecordedAt,
		SchemaVersion:  entry.SchemaVersion,
		Attempts:       attempts,
		Entry:          entry,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

func invalidStringPointer() *string {
	value := string([]byte{0xff})
	return &value
}

func detailJSON(t *testing.T, detail map[string]any, key string) []byte {
	t.Helper()
	value, ok := detail[key]
	if !ok {
		t.Fatalf("detail missing %q", key)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal detail %q: %v", key, err)
	}
	return encoded
}

type contextKey string
