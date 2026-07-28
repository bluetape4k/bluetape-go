package sqlkit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Mapper rows의 현재 row를 domain 값으로 mapping한다. 기대하는 column에 맞춰 rows.Scan을 정확히 호출해야 한다.
type Mapper[T any] func(rows *sql.Rows) (T, error)

// QueryAll query를 실행하고 반환된 모든 row를 mapping한다.
func QueryAll[T any](ctx context.Context, db Queryer, query string, mapper Mapper[T], args ...any) (values []T, err error) {
	return queryMapped(ctx, db, query, mapper, 0, args...)
}

// QueryOptional query를 실행하고 0개 또는 1개의 mapped row를 반환한다.
func QueryOptional[T any](ctx context.Context, db Queryer, query string, mapper Mapper[T], args ...any) (T, bool, error) {
	var zero T

	values, err := queryMapped(ctx, db, query, mapper, 2, args...)
	if err != nil {
		return zero, false, err
	}
	switch len(values) {
	case 0:
		return zero, false, nil
	case 1:
		return values[0], true, nil
	default:
		return zero, false, fmt.Errorf("%w: got at least %d rows", ErrTooManyRows, len(values))
	}
}

// QueryOne query를 실행하고 정확히 1개의 mapped row를 반환한다.
func QueryOne[T any](ctx context.Context, db Queryer, query string, mapper Mapper[T], args ...any) (T, error) {
	value, ok, err := QueryOptional(ctx, db, query, mapper, args...)
	if err != nil {
		return value, err
	}
	if !ok {
		return value, fmt.Errorf("%w", ErrNoRows)
	}
	return value, nil
}

func queryMapped[T any](ctx context.Context, db Queryer, query string, mapper Mapper[T], limit int, args ...any) (values []T, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		return nil, fmt.Errorf("%w: queryer is nil", ErrInvalidArgument)
	}
	if mapper == nil {
		return nil, fmt.Errorf("%w: mapper is nil", ErrInvalidArgument)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rows: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrapped := fmt.Errorf("close rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, wrapped)
				return
			}
			err = wrapped
		}
	}()

	for rows.Next() {
		value, err := mapper(rows)
		if err != nil {
			return nil, fmt.Errorf("map row: %w", err)
		}
		values = append(values, value)
		if limit > 0 && len(values) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return values, nil
}

// ScanOne 현재 row를 dest에 scan하고 dereference한 값을 반환한다.
func ScanOne[T any](dest *T) Mapper[T] {
	return func(rows *sql.Rows) (T, error) {
		var zero T
		if dest == nil {
			return zero, fmt.Errorf("%w: destination is nil", ErrInvalidArgument)
		}
		if err := rows.Scan(dest); err != nil {
			return zero, err
		}
		return *dest, nil
	}
}
