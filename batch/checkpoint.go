package batch

import (
	"context"
	"fmt"
	"sync"
)

// CheckpointReader는 interface 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CheckpointReader interface {
	Restore(context.Context, any) error
	Checkpoint(context.Context) (any, bool, error)
}

// CheckpointStore는 interface 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CheckpointStore interface {
	Load(context.Context, string) (any, bool, error)
	Save(context.Context, string, any) error
}

// MemoryCheckpointStore는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type MemoryCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewMemoryCheckpointStore는 NewMemoryCheckpointStore 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{values: make(map[string]any)}
}

// Load는 Load 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Load가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (s *MemoryCheckpointStore) Load(ctx context.Context, key string) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok, nil
}

// Save는 Save 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Save가 식별자, 상태, 이름, 또는 입력으로 해석하는 문자열 값이다. 빈 문자열 처리는 함수 계약을 따른다.
//   - checkpoint: Save 동작에 필요한 checkpoint 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (s *MemoryCheckpointStore) Save(ctx context.Context, key string, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = checkpoint
	return nil
}
