package eventbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
)

func ExampleNew() {
	client := &fakeClient{output: successOutput()}
	publisher, err := New(Options{
		Client:     client,
		Source:     "com.example.billing",
		DetailType: "InvoicePaid",
	})
	if err != nil {
		panic(err)
	}

	var outboxPublisher sqloutbox.Publisher = publisher
	if err := outboxPublisher.Publish(context.Background(), exampleRecord()); err != nil {
		panic(err)
	}
	fmt.Println("published")
	// Output: published
}

func exampleRecord() sqloutbox.Record {
	aggregate, err := audit.NewAggregateID("invoice", "42")
	if err != nil {
		panic(err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        "event-example",
		EventType:      "invoice.paid",
		AggregateID:    aggregate,
		Revision:       7,
		OccurredAt:     testClock,
		RecordedAt:     testClock.Add(time.Second),
		IdempotencyKey: "invoice:42:7",
		Metadata:       audit.Metadata{"source": "example"},
		Payload:        json.RawMessage(`{"amount":100}`),
	})
	if err != nil {
		panic(err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: "example", Event: event})
	if err != nil {
		panic(err)
	}
	return sqloutbox.Record{
		ID:             1,
		Status:         sqloutbox.StatusClaimed,
		Aggregate:      entry.Aggregate,
		Revision:       entry.Revision,
		EventID:        entry.Event.EventID,
		IdempotencyKey: entry.Event.IdempotencyKey,
		EventType:      entry.Event.EventType,
		OccurredAt:     entry.Event.OccurredAt,
		RecordedAt:     entry.Event.RecordedAt,
		SchemaVersion:  entry.SchemaVersion,
		Attempts:       1,
		Entry:          entry,
	}
}
