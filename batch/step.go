package batch

import (
	"context"
	"errors"
	"fmt"
)

// DefaultChunkSize는 상수 공개 값이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
const DefaultChunkSize = 100

// StepOptions는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type StepOptions[I any, O any] struct {
	Name            string
	ChunkSize       int
	Reader          Reader[I]
	Processor       Processor[I, O]
	Writer          Writer[O]
	RetryPolicy     RetryPolicy
	SkipPolicy      SkipPolicy
	CheckpointStore CheckpointStore
	CheckpointKey   string
}

// Step는 struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Step[I any, O any] struct {
	name      string
	chunkSize int
	reader    Reader[I]
	processor Processor[I, O]
	writer    Writer[O]
	atomic    AtomicCheckpointWriter[O]
	retry     RetryPolicy
	skip      SkipPolicy
	store     CheckpointStore
	key       string
}

// NewStep는 NewStep 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - options: NewStep 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewStep[I any, O any](options StepOptions[I, O]) (*Step[I, O], error) {
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
	if options.Writer == nil {
		return nil, fmt.Errorf("writer must not be nil")
	}
	retry, err := options.RetryPolicy.normalize()
	if err != nil {
		return nil, err
	}
	skip, err := options.SkipPolicy.normalize()
	if err != nil {
		return nil, err
	}
	if options.CheckpointStore != nil && options.CheckpointKey == "" {
		options.CheckpointKey = options.Name
	}
	return &Step[I, O]{
		name:      options.Name,
		chunkSize: options.ChunkSize,
		reader:    options.Reader,
		processor: options.Processor,
		writer:    options.Writer,
		retry:     retry,
		skip:      skip,
		store:     options.CheckpointStore,
		key:       options.CheckpointKey,
	}, nil
}

// Name는 Name 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (s *Step[I, O]) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run는 Run 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
func (s *Step[I, O]) Run(ctx context.Context) (report Report) {
	ctx = normalizeContext(ctx)
	if s == nil {
		report = newReport("")
		report.finish(StatusFailed, fmt.Errorf("step must not be nil"))
		return report
	}
	if s.atomic != nil {
		return s.runAtomic(ctx)
	}

	report = newReport(s.name)
	closeCtx := context.WithoutCancel(ctx)
	readerOpened := false
	writerOpened := false
	defer func() {
		s.closeResources(closeCtx, &report, readerOpened, writerOpened)
	}()

	if err := ctx.Err(); err != nil {
		report.finish(StatusCancelled, err)
		return report
	}
	if err := s.reader.Open(ctx); err != nil {
		report.finish(statusForError(err), err)
		return report
	}
	readerOpened = true
	if err := s.writer.Open(ctx); err != nil {
		report.finish(statusForError(err), err)
		return report
	}
	writerOpened = true
	if err := s.restoreCheckpoint(ctx); err != nil {
		report.finish(statusForError(err), err)
		return report
	}

	chunk := make([]O, 0, s.chunkSize)
	for {
		if err := ctx.Err(); err != nil {
			report.finish(StatusCancelled, err)
			return report
		}

		item, ok, err := s.reader.Read(ctx)
		if err != nil {
			report.finish(statusForError(err), err)
			return report
		}
		if !ok {
			if err := s.flush(ctx, &report, chunk); err != nil {
				report.finish(statusForError(err), err)
				return report
			}
			report.finish(StatusCompleted, nil)
			return report
		}
		report.ReadCount++

		processed, keep, err := s.process(ctx, &report, item)
		if err != nil {
			if s.skip.shouldSkip(err, report.SkipCount, 1) {
				report.SkipCount++
				if err := s.saveCheckpoint(ctx); err != nil {
					report.finish(statusForError(err), err)
					return report
				}
				continue
			}
			report.finish(statusForError(err), err)
			return report
		}
		if !keep {
			report.FilterCount++
			if err := s.saveCheckpoint(ctx); err != nil {
				report.finish(statusForError(err), err)
				return report
			}
			continue
		}

		chunk = append(chunk, processed)
		if len(chunk) == s.chunkSize {
			if err := s.flush(ctx, &report, chunk); err != nil {
				report.finish(statusForError(err), err)
				return report
			}
			chunk = chunk[:0]
		}
	}
}

func (s *Step[I, O]) flush(ctx context.Context, report *Report, chunk []O) error {
	if len(chunk) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.write(ctx, report, chunk); err != nil {
		if s.skip.shouldSkip(err, report.SkipCount, len(chunk)) {
			if s.store != nil {
				return fmt.Errorf("%w: %w", ErrUnsafeWriterSkipCheckpoint, err)
			}
			report.SkipCount += len(chunk)
			return nil
		}
		return err
	}
	report.WriteCount += len(chunk)
	return s.saveCheckpoint(ctx)
}

func (s *Step[I, O]) process(ctx context.Context, report *Report, item I) (O, bool, error) {
	var zero O
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, false, err
		}
		value, keep, err := s.processor.Process(ctx, item)
		if err == nil {
			return value, keep, nil
		}
		if !s.retry.shouldRetry(err, attempt) {
			return zero, false, err
		}
		report.RetryCount++
	}
}

func (s *Step[I, O]) write(ctx context.Context, report *Report, chunk []O) error {
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.writer.Write(ctx, chunk); err != nil {
			if !s.retry.shouldRetry(err, attempt) {
				return err
			}
			report.RetryCount++
			continue
		}
		return nil
	}
}

func (s *Step[I, O]) restoreCheckpoint(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	reader, ok := s.reader.(CheckpointReader)
	if !ok {
		return fmt.Errorf("reader does not support checkpoints")
	}
	checkpoint, exists, err := s.store.Load(ctx, s.key)
	if err != nil || !exists {
		return err
	}
	return reader.Restore(ctx, checkpoint)
}

func (s *Step[I, O]) saveCheckpoint(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	reader, ok := s.reader.(CheckpointReader)
	if !ok {
		return fmt.Errorf("reader does not support checkpoints")
	}
	checkpoint, exists, err := reader.Checkpoint(ctx)
	if err != nil || !exists {
		return err
	}
	return s.store.Save(ctx, s.key, checkpoint)
}

func (s *Step[I, O]) closeResources(ctx context.Context, report *Report, readerOpened bool, writerOpened bool) {
	var closeErr error
	if writerOpened && s != nil && s.writer != nil {
		closeErr = errors.Join(closeErr, s.writer.Close(ctx))
	}
	if readerOpened && s != nil && s.reader != nil {
		closeErr = errors.Join(closeErr, s.reader.Close(ctx))
	}
	if closeErr == nil {
		return
	}
	if report.Err == nil && report.Status == StatusCompleted {
		report.finish(StatusFailed, closeErr)
		return
	}
	report.Err = errors.Join(report.Err, closeErr)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func statusForError(err error) Status {
	if errors.Is(err, ErrCommitUnknown) || errors.Is(err, ErrAtomicityUnknown) {
		return StatusFailed
	}
	if isContextError(err) {
		return StatusCancelled
	}
	return StatusFailed
}
