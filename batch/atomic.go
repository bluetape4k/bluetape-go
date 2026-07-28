package batch

import (
	"context"
	"fmt"
)

// VersionedCheckpoint struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type VersionedCheckpoint struct {
	Value   any
	Version uint64
}

// AtomicCheckpointWriter interface 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AtomicCheckpointWriter[T any] interface {
	// Load returns the checkpoint for key and whether it exists. When no checkpoint
	// exists, the consumer contract starts with expected revision zero.
	Load(ctx context.Context, key string) (checkpoint VersionedCheckpoint, exists bool, err error)
	// Commit atomically persists items and checkpoint only when expectedVersion
	// matches, and returns the new revision. Implementations must preserve applicable
	// ErrCheckpointConflict, ErrCommitUnknown, ErrAtomicityUnknown, and
	// ErrCheckpointVersionExhausted identities via %w or equivalent errors.Is behavior.
	Commit(ctx context.Context, key string, expectedVersion uint64, items []T, checkpoint any) (newVersion uint64, err error)
}

// AtomicStepOptions struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AtomicStepOptions[I any, O any] struct {
	Name          string
	ChunkSize     int
	Reader        Reader[I]
	Processor     Processor[I, O]
	AtomicWriter  AtomicCheckpointWriter[O]
	RetryPolicy   RetryPolicy
	SkipPolicy    SkipPolicy
	CheckpointKey string
}

// NewAtomicStep NewAtomicStep 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - options: NewAtomicStep 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewAtomicStep[I any, O any](options AtomicStepOptions[I, O]) (*Step[I, O], error) {
	if options.Name == "" {
		return nil, fmt.Errorf("step name must not be empty")
	}
	if options.ChunkSize == 0 {
		options.ChunkSize = DefaultChunkSize
	}
	if options.ChunkSize < 0 {
		return nil, fmt.Errorf("chunk size must be positive")
	}
	if options.Reader == nil {
		return nil, fmt.Errorf("reader must not be nil")
	}
	if options.Processor == nil {
		return nil, fmt.Errorf("processor must not be nil")
	}
	if options.AtomicWriter == nil {
		return nil, fmt.Errorf("atomic writer must not be nil")
	}
	if _, ok := options.Reader.(CheckpointReader); !ok {
		return nil, fmt.Errorf("reader does not support checkpoints")
	}
	retry, err := options.RetryPolicy.normalize()
	if err != nil {
		return nil, err
	}
	skip, err := options.SkipPolicy.normalize()
	if err != nil {
		return nil, err
	}
	if options.CheckpointKey == "" {
		options.CheckpointKey = options.Name
	}
	return &Step[I, O]{
		name:      options.Name,
		chunkSize: options.ChunkSize,
		reader:    options.Reader,
		processor: options.Processor,
		atomic:    options.AtomicWriter,
		retry:     retry,
		skip:      skip,
		key:       options.CheckpointKey,
	}, nil
}
