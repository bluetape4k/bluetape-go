package batchwrite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// MaxItemsPerBatch is the DynamoDB BatchWriteItem request item limit.
const MaxItemsPerBatch = 25

// DefaultMaxAttempts is the default attempt budget for each request chunk.
const DefaultMaxAttempts = 3

var (
	// ErrNilClient reports a missing DynamoDB batch write client.
	ErrNilClient = errors.New("batchwrite: client must not be nil")
	// ErrEmptyRequestItems reports an empty batch write request set.
	ErrEmptyRequestItems = errors.New("batchwrite: request items must not be empty")
	// ErrInvalidMaxAttempts reports a non-positive retry budget.
	ErrInvalidMaxAttempts = errors.New("batchwrite: max attempts must be positive")
	// ErrUnprocessedItems reports exhausted retry attempts with items still pending.
	ErrUnprocessedItems = errors.New("batchwrite: unprocessed items remain")
)

// Client is the AWS SDK for Go v2 BatchWriteItem subset used by WriteAll.
type Client interface {
	BatchWriteItem(context.Context, *dynamodb.BatchWriteItemInput, ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

// Backoff returns the delay after a failed one-based attempt.
type Backoff func(attempt int) time.Duration

// Options configures WriteAll.
type Options struct {
	MaxAttempts                 int
	Backoff                     Backoff
	ReturnConsumedCapacity      types.ReturnConsumedCapacity
	ReturnItemCollectionMetrics types.ReturnItemCollectionMetrics
}

// Option configures WriteAll.
type Option func(*Options)

// Result summarizes completed BatchWriteItem calls.
type Result struct {
	Attempts              int
	Processed             int
	ConsumedCapacity      []types.ConsumedCapacity
	ItemCollectionMetrics map[string][]types.ItemCollectionMetrics
}

// UnprocessedItemsError reports items that remained after retry exhaustion.
type UnprocessedItemsError struct {
	Attempts         int
	UnprocessedItems map[string][]types.WriteRequest
}

// Error returns a human-readable retry exhaustion message.
func (e UnprocessedItemsError) Error() string {
	return fmt.Sprintf("%v after %d attempts (%d item(s) remain)", ErrUnprocessedItems, e.Attempts, countRequestItems(e.UnprocessedItems))
}

// Is supports errors.Is(err, ErrUnprocessedItems).
func (e UnprocessedItemsError) Is(target error) bool {
	return target == ErrUnprocessedItems
}

// WithMaxAttempts sets the per-chunk attempt budget.
func WithMaxAttempts(maxAttempts int) Option {
	return func(options *Options) {
		options.MaxAttempts = maxAttempts
	}
}

// WithBackoff sets the retry delay policy used before resubmitting unprocessed items.
func WithBackoff(backoff Backoff) Option {
	return func(options *Options) {
		options.Backoff = backoff
	}
}

// WithReturnConsumedCapacity requests DynamoDB consumed-capacity details.
func WithReturnConsumedCapacity(value types.ReturnConsumedCapacity) Option {
	return func(options *Options) {
		options.ReturnConsumedCapacity = value
	}
}

// WithReturnItemCollectionMetrics requests DynamoDB item-collection metrics.
func WithReturnItemCollectionMetrics(value types.ReturnItemCollectionMetrics) Option {
	return func(options *Options) {
		options.ReturnItemCollectionMetrics = value
	}
}

// WriteAll writes all request items with DynamoDB BatchWriteItem.
//
// requestItems uses the same table-to-WriteRequest map accepted by the AWS SDK.
// WriteAll splits the map into requests containing at most MaxItemsPerBatch
// items, then retries only the UnprocessedItems returned by DynamoDB. Service
// errors are returned immediately and wrapped with their batch attempt context.
func WriteAll(ctx context.Context, client Client, requestItems map[string][]types.WriteRequest, options ...Option) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return Result{}, ErrNilClient
	}

	cfg, err := newOptions(options)
	if err != nil {
		return Result{}, err
	}

	items := flattenRequestItems(requestItems)
	if len(items) == 0 {
		return Result{}, ErrEmptyRequestItems
	}

	var result Result
	for _, chunk := range chunkItems(items, MaxItemsPerBatch) {
		chunkTotal := len(chunk)
		pending := itemsToRequestMap(chunk)

		for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return result, err
			}

			result.Attempts++
			out, err := client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems:                pending,
				ReturnConsumedCapacity:      cfg.ReturnConsumedCapacity,
				ReturnItemCollectionMetrics: cfg.ReturnItemCollectionMetrics,
			})
			if err != nil {
				return result, fmt.Errorf("batch write items attempt %d: %w", attempt, err)
			}
			result.merge(out)

			unprocessed := cloneRequestItems(nil)
			if out != nil {
				unprocessed = cloneRequestItems(out.UnprocessedItems)
			}
			remaining := countRequestItems(unprocessed)
			if remaining == 0 {
				result.Processed += chunkTotal
				break
			}
			if attempt == cfg.MaxAttempts {
				result.Processed += chunkTotal - remaining
				return result, UnprocessedItemsError{
					Attempts:         attempt,
					UnprocessedItems: unprocessed,
				}
			}

			pending = unprocessed
			delay := cfg.Backoff(attempt)
			if err := sleep(ctx, delay); err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

func newOptions(optionFns []Option) (Options, error) {
	cfg := Options{
		MaxAttempts: DefaultMaxAttempts,
		Backoff: func(int) time.Duration {
			return 0
		},
	}
	for _, option := range optionFns {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.MaxAttempts <= 0 {
		return cfg, ErrInvalidMaxAttempts
	}
	if cfg.Backoff == nil {
		cfg.Backoff = func(int) time.Duration {
			return 0
		}
	}
	return cfg, nil
}

type tableWrite struct {
	table string
	item  types.WriteRequest
}

func flattenRequestItems(requestItems map[string][]types.WriteRequest) []tableWrite {
	tables := make([]string, 0, len(requestItems))
	for table := range requestItems {
		tables = append(tables, table)
	}
	sort.Strings(tables)

	var items []tableWrite
	for _, table := range tables {
		for _, item := range requestItems[table] {
			items = append(items, tableWrite{table: table, item: item})
		}
	}
	return items
}

func chunkItems(items []tableWrite, size int) [][]tableWrite {
	chunks := make([][]tableWrite, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func itemsToRequestMap(items []tableWrite) map[string][]types.WriteRequest {
	requestItems := make(map[string][]types.WriteRequest)
	for _, item := range items {
		requestItems[item.table] = append(requestItems[item.table], item.item)
	}
	return requestItems
}

func cloneRequestItems(requestItems map[string][]types.WriteRequest) map[string][]types.WriteRequest {
	if len(requestItems) == 0 {
		return nil
	}
	cloned := make(map[string][]types.WriteRequest, len(requestItems))
	for table, items := range requestItems {
		cloned[table] = append([]types.WriteRequest(nil), items...)
	}
	return cloned
}

func countRequestItems(requestItems map[string][]types.WriteRequest) int {
	var count int
	for _, items := range requestItems {
		count += len(items)
	}
	return count
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *Result) merge(out *dynamodb.BatchWriteItemOutput) {
	if out == nil {
		return
	}
	r.ConsumedCapacity = append(r.ConsumedCapacity, out.ConsumedCapacity...)
	if len(out.ItemCollectionMetrics) == 0 {
		return
	}
	if r.ItemCollectionMetrics == nil {
		r.ItemCollectionMetrics = make(map[string][]types.ItemCollectionMetrics, len(out.ItemCollectionMetrics))
	}
	for table, metrics := range out.ItemCollectionMetrics {
		r.ItemCollectionMetrics[table] = append(r.ItemCollectionMetrics[table], metrics...)
	}
}
