package auditexample

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func ExampleOrderService() {
	ctx := context.Background()
	repo := audit.NewMemoryRepository()
	service, err := NewOrderService(repo, OrderServiceOptions{
		Author: "orders-api",
		Now:    fixedClock,
	})
	if err != nil {
		panic(err)
	}

	if _, err := service.CreateOrder(ctx, CreateOrderCommand{
		OrderID:    "order-100",
		CustomerID: "customer-42",
		CommandID:  "command-create-100",
	}); err != nil {
		panic(err)
	}
	if _, err := service.AddItem(ctx, AddItemCommand{
		OrderID:   "order-100",
		SKU:       "sku-blue",
		Quantity:  2,
		CommandID: "command-add-100",
	}); err != nil {
		panic(err)
	}
	order, err := service.CompleteOrder(ctx, CompleteOrderCommand{
		OrderID:   "order-100",
		CommandID: "command-complete-100",
	})
	if err != nil {
		panic(err)
	}

	history, ok, err := service.History(ctx, "order-100")
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("missing history")
	}

	fmt.Println(order.Status, len(order.Items))
	fmt.Println(history.AggregateID(), history.HeadRevision())
	for _, entry := range history.Entries() {
		fmt.Println(entry.Revision, entry.Event.EventType)
	}

	// Output:
	// completed 1
	// order:order-100 3
	// 1 OrderCreated
	// 2 OrderItemAdded
	// 3 OrderCompleted
}

func TestOrderServiceRecordsHistoryQueriesAndOutboxReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	repo := audit.NewMemoryRepository()
	service := newTestService(t, repo)

	if _, err := service.CreateOrder(ctx, CreateOrderCommand{
		OrderID:    "order-200",
		CustomerID: "customer-42",
		CommandID:  "command-create-200",
	}); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if _, err := service.AddItem(ctx, AddItemCommand{
		OrderID:   "order-200",
		SKU:       "sku-red",
		Quantity:  3,
		CommandID: "command-add-200",
	}); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	history, ok, err := service.History(ctx, "order-200")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if !ok {
		t.Fatal("History returned ok=false")
	}
	if history.HeadRevision() != audit.Revision(2) {
		t.Fatalf("HeadRevision = %d, want 2", history.HeadRevision())
	}
	if got := eventTypes(history.Entries()); !reflect.DeepEqual(got, []audit.EventType{"OrderCreated", "OrderItemAdded"}) {
		t.Fatalf("event types = %v", got)
	}

	found, err := repo.Find(ctx, audit.Query{AggregateType: "order", FromRevision: audit.Revision(2)})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 1 || found[0].Event.EventID != "command-add-200:OrderItemAdded" {
		t.Fatalf("Find returned %#v", found)
	}

	outbox := NewMemoryOutbox()
	aggregate, err := NewOrderAggregateID("order-200")
	if err != nil {
		t.Fatalf("NewOrderAggregateID: %v", err)
	}
	if err := ReplayHistoryToOutbox(ctx, repo, aggregate, outbox); err != nil {
		t.Fatalf("ReplayHistoryToOutbox: %v", err)
	}
	if got := eventTypes(outbox.Entries()); !reflect.DeepEqual(got, []audit.EventType{"OrderCreated", "OrderItemAdded"}) {
		t.Fatalf("outbox event types = %v", got)
	}
}

func TestOrderServiceUsesRepositoryBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	expected := errors.New("repository unavailable")
	service := newTestService(t, failingRepository{err: expected})
	_, err := service.CreateOrder(ctx, CreateOrderCommand{
		OrderID:    "order-rejected",
		CustomerID: "customer-42",
		CommandID:  "command-rejected",
	})
	if !errors.Is(err, expected) {
		t.Fatalf("CreateOrder error = %v, want %v", err, expected)
	}
	if _, ok := service.Lookup("order-rejected"); ok {
		t.Fatal("source order was mutated after repository failure")
	}
}

func TestOrderServiceConcurrentCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	repo := audit.NewMemoryRepository()
	service := newTestService(t, repo)
	var sequence atomic.Int64

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 16,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		id := sequence.Add(1)
		orderID := fmt.Sprintf("order-%02d", id)
		if _, err := service.CreateOrder(ctx, CreateOrderCommand{
			OrderID:    orderID,
			CustomerID: "customer-42",
			CommandID:  fmt.Sprintf("command-create-%02d", id),
		}); err != nil {
			return err
		}
		if _, err := service.AddItem(ctx, AddItemCommand{
			OrderID:   orderID,
			SKU:       "sku-blue",
			Quantity:  1,
			CommandID: fmt.Sprintf("command-add-%02d", id),
		}); err != nil {
			return err
		}
		return nil
	})

	entries, err := repo.Find(ctx, audit.Query{AggregateType: "order"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(entries) != 32 {
		t.Fatalf("entry count = %d, want 32", len(entries))
	}
}

func TestReplayHistoryToOutboxHonorsCancellation(t *testing.T) {
	repo := audit.NewMemoryRepository()
	service := newTestService(t, repo)
	ctx := context.Background()
	if _, err := service.CreateOrder(ctx, CreateOrderCommand{
		OrderID:    "order-cancel",
		CustomerID: "customer-42",
		CommandID:  "command-cancel",
	}); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	aggregate, err := NewOrderAggregateID("order-cancel")
	if err != nil {
		t.Fatalf("NewOrderAggregateID: %v", err)
	}
	outbox := NewMemoryOutbox()
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 3,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := ReplayHistoryToOutbox(cancelled, repo, aggregate, outbox)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err == nil {
			return errors.New("ReplayHistoryToOutbox returned nil, want context.Canceled")
		}
		return fmt.Errorf("ReplayHistoryToOutbox returned unexpected error, want context.Canceled: %w", err)
	})
}

func newTestService(t *testing.T, repo audit.Repository) *OrderService {
	t.Helper()

	service, err := NewOrderService(repo, OrderServiceOptions{
		Author: "orders-api",
		Now:    fixedClock,
	})
	if err != nil {
		t.Fatalf("NewOrderService: %v", err)
	}
	return service
}

func fixedClock() time.Time {
	return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
}

func eventTypes(entries []audit.Entry) []audit.EventType {
	types := make([]audit.EventType, len(entries))
	for i, entry := range entries {
		types[i] = entry.Event.EventType
	}
	return types
}

type failingRepository struct {
	err error
}

func (r failingRepository) Append(context.Context, ...audit.Entry) error {
	return r.err
}

func (r failingRepository) Find(context.Context, audit.Query) ([]audit.Entry, error) {
	return nil, r.err
}

func (r failingRepository) LoadHistory(context.Context, audit.AggregateID) (audit.History, bool, error) {
	return audit.History{}, false, r.err
}

func (r failingRepository) Latest(context.Context, audit.AggregateID) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}

func (r failingRepository) LatestSnapshot(context.Context, audit.AggregateID) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}

func (r failingRepository) PreviousSnapshot(context.Context, audit.AggregateID, audit.Revision) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}
