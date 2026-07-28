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

// MaxItemsPerBatch DynamoDB BatchWriteItem request의 item 한도다.
const MaxItemsPerBatch = 25

// DefaultMaxAttempts 각 request chunk에 적용하는 기본 attempt 예산이다.
const DefaultMaxAttempts = 3

var (
	// ErrNilClient DynamoDB batch write client가 없을 때 반환된다.
	ErrNilClient = errors.New("batchwrite: client must not be nil")
	// ErrEmptyRequestItems batch write request set이 비어 있을 때 반환된다.
	ErrEmptyRequestItems = errors.New("batchwrite: request items must not be empty")
	// ErrInvalidMaxAttempts retry budget이 양수가 아닐 때 반환된다.
	ErrInvalidMaxAttempts = errors.New("batchwrite: max attempts must be positive")
	// ErrUnprocessedItems retry attempt를 소진한 뒤에도 pending item이 남았을 때 반환된다.
	ErrUnprocessedItems = errors.New("batchwrite: unprocessed items remain")
)

// Client WriteAll이 사용하는 AWS SDK for Go v2 BatchWriteItem subset이다.
type Client interface {
	BatchWriteItem(context.Context, *dynamodb.BatchWriteItemInput, ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

// Backoff 1-base attempt 실패 뒤 대기할 delay를 반환한다.
type Backoff func(attempt int) time.Duration

// Options WriteAll 실행 방식을 설정한다.
type Options struct {
	MaxAttempts                 int
	Backoff                     Backoff
	ReturnConsumedCapacity      types.ReturnConsumedCapacity
	ReturnItemCollectionMetrics types.ReturnItemCollectionMetrics
}

// Option은 WriteAll 실행 설정을 변경한다.
type Option func(*Options)

// Result 완료된 BatchWriteItem 호출 결과를 요약한다.
type Result struct {
	Attempts              int
	Processed             int
	ConsumedCapacity      []types.ConsumedCapacity
	ItemCollectionMetrics map[string][]types.ItemCollectionMetrics
}

// UnprocessedItemsError retry를 소진한 뒤에도 남은 item을 보고한다.
type UnprocessedItemsError struct {
	Attempts         int
	UnprocessedItems map[string][]types.WriteRequest
}

// Error 사람이 읽을 수 있는 retry exhaustion message를 반환한다.
func (e UnprocessedItemsError) Error() string {
	return fmt.Sprintf("%v after %d attempts (%d item(s) remain)", ErrUnprocessedItems, e.Attempts, countRequestItems(e.UnprocessedItems))
}

// Is errors.Is(err, ErrUnprocessedItems)를 지원한다.
func (e UnprocessedItemsError) Is(target error) bool {
	return target == ErrUnprocessedItems
}

// WithMaxAttempts chunk별 attempt budget을 설정한다.
func WithMaxAttempts(maxAttempts int) Option {
	return func(options *Options) {
		options.MaxAttempts = maxAttempts
	}
}

// WithBackoff unprocessed item을 다시 제출하기 전에 사용할 retry delay policy를 설정한다.
func WithBackoff(backoff Backoff) Option {
	return func(options *Options) {
		options.Backoff = backoff
	}
}

// WithReturnConsumedCapacity DynamoDB consumed-capacity detail 반환을 요청한다.
func WithReturnConsumedCapacity(value types.ReturnConsumedCapacity) Option {
	return func(options *Options) {
		options.ReturnConsumedCapacity = value
	}
}

// WithReturnItemCollectionMetrics DynamoDB item-collection metric 반환을 요청한다.
func WithReturnItemCollectionMetrics(value types.ReturnItemCollectionMetrics) Option {
	return func(options *Options) {
		options.ReturnItemCollectionMetrics = value
	}
}

// WriteAll은 모든 request item을 DynamoDB BatchWriteItem으로 기록한다.
//
// requestItems는 AWS SDK가 받는 table-to-WriteRequest map과 같은 형식을 사용한다. WriteAll은 map을
// 최대 MaxItemsPerBatch item을 포함하는 request로 나눈 뒤, DynamoDB가 반환한 UnprocessedItems만
// 재시도한다. Service error는 즉시 반환하며 batch attempt context로 감싼다.
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
