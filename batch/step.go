package batch

import (
	"context"
	"errors"
	"fmt"
)

// DefaultChunkSize is used when StepOptions.ChunkSize is zero.
const DefaultChunkSize = 100

// StepOptions configures a batch step.
type StepOptions[I any, O any] struct {
	Name      string
	ChunkSize int
	Reader    Reader[I]
	Processor Processor[I, O]
	Writer    Writer[O]
}

// Step runs a reader, processor, and writer as one chunk-oriented batch unit.
type Step[I any, O any] struct {
	name      string
	chunkSize int
	reader    Reader[I]
	processor Processor[I, O]
	writer    Writer[O]
}

// NewStep creates a batch step.
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
	return &Step[I, O]{
		name:      options.Name,
		chunkSize: options.ChunkSize,
		reader:    options.Reader,
		processor: options.Processor,
		writer:    options.Writer,
	}, nil
}

// Name returns the step name.
func (s *Step[I, O]) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the step until the reader is exhausted, context is cancelled, or
// processing/writing fails.
func (s *Step[I, O]) Run(ctx context.Context) (report Report) {
	ctx = normalizeContext(ctx)
	if s == nil {
		report = newReport("")
		report.finish(StatusFailed, fmt.Errorf("step must not be nil"))
		return report
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

		processed, keep, err := s.processor.Process(ctx, item)
		if err != nil {
			report.finish(statusForError(err), err)
			return report
		}
		if !keep {
			report.FilterCount++
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
	if err := s.writer.Write(ctx, chunk); err != nil {
		return err
	}
	report.WriteCount += len(chunk)
	return nil
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
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StatusCancelled
	}
	return StatusFailed
}
