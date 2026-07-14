package batch

import (
	"context"
	"errors"
	"fmt"
)

func (s *Step[I, O]) runAtomic(ctx context.Context) (report Report) {
	report = newReport(s.name)
	readerOpened := false
	defer func() {
		if !readerOpened {
			return
		}
		closeErr := s.reader.Close(context.WithoutCancel(ctx))
		if closeErr == nil {
			return
		}
		if report.Err == nil && report.Status == StatusCompleted {
			report.finish(StatusFailed, closeErr)
			return
		}
		report.Err = errors.Join(report.Err, closeErr)
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

	reader, ok := s.reader.(CheckpointReader)
	if !ok {
		err := fmt.Errorf("reader does not support checkpoints")
		report.finish(StatusFailed, err)
		return report
	}

	checkpoint, exists, err := s.atomic.Load(ctx, s.key)
	if err != nil {
		report.finish(statusForError(err), err)
		return report
	}
	expected := uint64(0)
	if exists {
		if err := reader.Restore(ctx, checkpoint.Value); err != nil {
			report.finish(statusForError(err), err)
			return report
		}
		expected = checkpoint.Version
	}

	pending := make([]O, 0, s.chunkSize)
	progressCount := 0
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
			if progressCount > 0 {
				expected, err = s.commitAtomic(ctx, &report, reader, expected, pending)
				if err != nil {
					report.finish(statusForError(err), err)
					return report
				}
			}
			report.finish(StatusCompleted, nil)
			return report
		}

		report.ReadCount++
		progressCount++
		processed, keep, err := s.process(ctx, &report, item)
		if err != nil {
			if s.skip.shouldSkip(err, report.SkipCount, 1) {
				report.SkipCount++
			} else {
				report.finish(statusForError(err), err)
				return report
			}
		} else if !keep {
			report.FilterCount++
		} else {
			pending = append(pending, processed)
		}

		if progressCount == s.chunkSize {
			expected, err = s.commitAtomic(ctx, &report, reader, expected, pending)
			if err != nil {
				report.finish(statusForError(err), err)
				return report
			}
			pending = pending[:0]
			progressCount = 0
		}
	}
}

func (s *Step[I, O]) commitAtomic(
	ctx context.Context,
	report *Report,
	reader CheckpointReader,
	expected uint64,
	pending []O,
) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	checkpoint, exists, err := reader.Checkpoint(ctx)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("reader checkpoint is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	items := append([]O(nil), pending...)
	version, err := s.atomic.Commit(ctx, s.key, expected, items, checkpoint)
	if err != nil {
		return 0, err
	}
	report.WriteCount += len(pending)
	return version, nil
}
