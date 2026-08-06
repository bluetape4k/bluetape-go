package batch

import (
	"context"
	"fmt"
	"sync"
)

// CheckpointReader batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 인터페이스이다.
type CheckpointReader interface {
	Restore(context.Context, any) error
	Checkpoint(context.Context) (any, bool, error)
}

// CheckpointStore batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 인터페이스이다.
type CheckpointStore interface {
	Load(context.Context, string) (any, bool, error)
	Save(context.Context, string, any) error
}

// MemoryCheckpointStore batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type MemoryCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewMemoryCheckpointStore batch 단계, checkpoint, writer 안전성, 재시작에 사용할 값을 생성한다.
func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{values: make(map[string]any)}
}

// Load batch 단계, checkpoint, writer 안전성, 재시작에서 필요한 값을 조회한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Load가 해석할 문자열이다. 빈 문자열은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// Save batch 단계, checkpoint, writer 안전성, 재시작의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: Save가 해석할 문자열이다. 빈 문자열은 구현 검증을 따른다.
//   - checkpoint: 저장하거나 commit할 checkpoint 값이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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
