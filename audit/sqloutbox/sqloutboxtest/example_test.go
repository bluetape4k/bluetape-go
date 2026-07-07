package sqloutboxtest_test

import (
	"context"
	"fmt"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
)

func ExampleDiscardPublisher() {
	publisher := sqloutboxtest.DiscardPublisher{}

	err := publisher.Publish(context.Background(), sqloutbox.Record{
		EventID:        audit.EventID("evt-001"),
		IdempotencyKey: "command-001",
	})

	fmt.Println(err == nil)
	// Output: true
}

func ExamplePublisherFunc() {
	publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, record sqloutbox.Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Println(record.EventID)
		return nil
	})

	_ = publisher.Publish(context.Background(), sqloutbox.Record{
		EventID: audit.EventID("evt-002"),
	})

	// Output: evt-002
}

func ExampleRecordingPublisher() {
	publisher := sqloutboxtest.NewRecordingPublisher()

	_ = publisher.Publish(context.Background(), sqloutbox.Record{EventID: audit.EventID("evt-003")})
	_ = publisher.Publish(context.Background(), sqloutbox.Record{EventID: audit.EventID("evt-004")})

	fmt.Println(publisher.EventIDs())
	// Output: [evt-003 evt-004]
}
