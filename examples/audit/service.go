package auditexample

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

var (
	// ErrInvalidCommand reports an invalid example service command.
	ErrInvalidCommand = errors.New("invalid audit example command")
	// ErrOrderExists reports duplicate order creation.
	ErrOrderExists = errors.New("order already exists")
	// ErrOrderNotFound reports a command for a missing order.
	ErrOrderNotFound = errors.New("order not found")
	// ErrOrderCompleted reports a mutation attempt after completion.
	ErrOrderCompleted = errors.New("order already completed")
)

// OrderServiceOptions configures the example order service.
type OrderServiceOptions struct {
	Author string
	Now    func() time.Time
}

// OrderService records order changes through an audit.Repository.
type OrderService struct {
	mu     sync.Mutex
	repo   audit.Repository
	author string
	now    func() time.Time
	orders map[string]Order
}

// Order is the source-of-truth state in the example service.
type Order struct {
	ID         string
	CustomerID string
	Status     string
	Items      []LineItem
	UpdatedAt  time.Time
}

// LineItem is one order line in the example source state.
type LineItem struct {
	SKU      string
	Quantity int
}

// CreateOrderCommand creates an order aggregate.
type CreateOrderCommand struct {
	OrderID    string
	CustomerID string
	CommandID  string
}

// AddItemCommand adds an item to an open order.
type AddItemCommand struct {
	OrderID   string
	SKU       string
	Quantity  int
	CommandID string
}

// CompleteOrderCommand marks an order complete.
type CompleteOrderCommand struct {
	OrderID   string
	CommandID string
}

// EntrySink is the minimal outbox-like boundary used by the example replay
// helper. Production callers can adapt this to audit/sqloutbox.Store.Enqueue.
type EntrySink interface {
	Enqueue(context.Context, ...audit.Entry) error
}

// NewOrderService creates an audit-backed in-memory order service.
func NewOrderService(repo audit.Repository, options OrderServiceOptions) (*OrderService, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository must not be nil", ErrInvalidCommand)
	}
	author := strings.TrimSpace(options.Author)
	if author == "" {
		author = "audit-example"
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &OrderService{
		repo:   repo,
		author: author,
		now:    now,
		orders: make(map[string]Order),
	}, nil
}

// NewOrderAggregateID creates the aggregate identity used by the example.
func NewOrderAggregateID(orderID string) (audit.AggregateID, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return audit.AggregateID{}, fmt.Errorf("%w: order id is required", ErrInvalidCommand)
	}
	return audit.NewAggregateID("order", orderID)
}

// CreateOrder creates a new order and writes an audit entry before mutating the
// source state.
func (s *OrderService) CreateOrder(ctx context.Context, command CreateOrderCommand) (Order, error) {
	if err := checkContext(ctx); err != nil {
		return Order{}, err
	}
	orderID := strings.TrimSpace(command.OrderID)
	customerID := strings.TrimSpace(command.CustomerID)
	commandID := strings.TrimSpace(command.CommandID)
	if orderID == "" || customerID == "" || commandID == "" {
		return Order{}, fmt.Errorf("%w: order id, customer id, and command id are required", ErrInvalidCommand)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orders[orderID]; ok {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderExists, orderID)
	}

	now := s.now()
	order := Order{
		ID:         orderID,
		CustomerID: customerID,
		Status:     "open",
		UpdatedAt:  now,
	}
	entry, err := s.nextEntry(ctx, orderID, audit.EventType("OrderCreated"), commandID, now, map[string]any{
		"customer_id": customerID,
		"status":      order.Status,
	}, []string{"customer_id", "status"})
	if err != nil {
		return Order{}, err
	}
	if err := s.repo.Append(ctx, entry); err != nil {
		return Order{}, err
	}
	s.orders[orderID] = cloneOrder(order)
	return order, nil
}

// AddItem adds one line item and records the change in audit history.
func (s *OrderService) AddItem(ctx context.Context, command AddItemCommand) (Order, error) {
	if err := checkContext(ctx); err != nil {
		return Order{}, err
	}
	orderID := strings.TrimSpace(command.OrderID)
	sku := strings.TrimSpace(command.SKU)
	commandID := strings.TrimSpace(command.CommandID)
	if orderID == "" || sku == "" || commandID == "" || command.Quantity <= 0 {
		return Order{}, fmt.Errorf("%w: order id, sku, positive quantity, and command id are required", ErrInvalidCommand)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, orderID)
	}
	if order.Status == "completed" {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderCompleted, orderID)
	}

	now := s.now()
	updated := cloneOrder(order)
	updated.Items = append(updated.Items, LineItem{SKU: sku, Quantity: command.Quantity})
	updated.UpdatedAt = now
	entry, err := s.nextEntry(ctx, orderID, audit.EventType("OrderItemAdded"), commandID, now, map[string]any{
		"sku":      sku,
		"quantity": command.Quantity,
	}, []string{"items"})
	if err != nil {
		return Order{}, err
	}
	if err := s.repo.Append(ctx, entry); err != nil {
		return Order{}, err
	}
	s.orders[orderID] = cloneOrder(updated)
	return updated, nil
}

// CompleteOrder marks an order complete and records the change in audit
// history.
func (s *OrderService) CompleteOrder(ctx context.Context, command CompleteOrderCommand) (Order, error) {
	if err := checkContext(ctx); err != nil {
		return Order{}, err
	}
	orderID := strings.TrimSpace(command.OrderID)
	commandID := strings.TrimSpace(command.CommandID)
	if orderID == "" || commandID == "" {
		return Order{}, fmt.Errorf("%w: order id and command id are required", ErrInvalidCommand)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, orderID)
	}
	if order.Status == "completed" {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderCompleted, orderID)
	}

	now := s.now()
	updated := cloneOrder(order)
	updated.Status = "completed"
	updated.UpdatedAt = now
	entry, err := s.nextEntry(ctx, orderID, audit.EventType("OrderCompleted"), commandID, now, map[string]any{
		"status": updated.Status,
	}, []string{"status"})
	if err != nil {
		return Order{}, err
	}
	if err := s.repo.Append(ctx, entry); err != nil {
		return Order{}, err
	}
	s.orders[orderID] = cloneOrder(updated)
	return updated, nil
}

// History returns reconstructed audit history for one order.
func (s *OrderService) History(ctx context.Context, orderID string) (audit.History, bool, error) {
	aggregate, err := NewOrderAggregateID(orderID)
	if err != nil {
		return audit.History{}, false, err
	}
	return s.repo.LoadHistory(ctx, aggregate)
}

// Lookup returns a defensive copy of source order state.
func (s *OrderService) Lookup(orderID string) (Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[strings.TrimSpace(orderID)]
	if !ok {
		return Order{}, false
	}
	return cloneOrder(order), true
}

func (s *OrderService) nextEntry(
	ctx context.Context,
	orderID string,
	eventType audit.EventType,
	commandID string,
	now time.Time,
	payload map[string]any,
	changedFields []string,
) (audit.Entry, error) {
	aggregate, err := NewOrderAggregateID(orderID)
	if err != nil {
		return audit.Entry{}, err
	}
	latest, ok, err := s.repo.Latest(ctx, aggregate)
	if err != nil {
		return audit.Entry{}, err
	}
	var head audit.Revision
	if ok {
		head = latest.Revision
	}
	recorder, err := audit.NewAggregateRecorderFromHead(aggregate, head)
	if err != nil {
		return audit.Entry{}, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return audit.Entry{}, err
	}
	event, err := recorder.Record(audit.EventRecord{
		EventID:        audit.EventID(commandID + ":" + string(eventType)),
		EventType:      eventType,
		OccurredAt:     now,
		IdempotencyKey: commandID,
		Metadata:       audit.Metadata{"example": "orders"},
		Payload:        encoded,
	})
	if err != nil {
		return audit.Entry{}, err
	}
	change, err := audit.NewChangeMetadata(changedFields, string(eventType), nil)
	if err != nil {
		return audit.Entry{}, err
	}
	return audit.NewEntry(audit.EntryOptions{
		Author: s.author,
		Event:  event,
		Change: &change,
	})
}

// ReplayHistoryToOutbox replays one aggregate history into an outbox-like sink.
func ReplayHistoryToOutbox(ctx context.Context, reader audit.HistoryReader, aggregate audit.AggregateID, sink EntrySink) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if reader == nil {
		return fmt.Errorf("%w: history reader must not be nil", ErrInvalidCommand)
	}
	if sink == nil {
		return fmt.Errorf("%w: entry sink must not be nil", ErrInvalidCommand)
	}
	history, ok, err := reader.LoadHistory(ctx, aggregate)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrOrderNotFound, aggregate)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	return sink.Enqueue(ctx, history.Entries()...)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func cloneOrder(order Order) Order {
	clone := order
	if len(order.Items) > 0 {
		clone.Items = append([]LineItem(nil), order.Items...)
	}
	return clone
}
