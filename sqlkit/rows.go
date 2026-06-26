package sqlkit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Mapper maps the current row from rows. It should call rows.Scan exactly for
// the columns it expects.
type Mapper[T any] func(rows *sql.Rows) (T, error)

// QueryAll runs query and maps every returned row.
func QueryAll[T any](ctx context.Context, db Queryer, query string, mapper Mapper[T], args ...any) (values []T, err error) {
	return queryMapped(ctx, db, query, mapper, 0, args...)
}

// QueryOptional runs query and returns zero or one mapped row.
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

// QueryOne runs query and returns exactly one mapped row.
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

// ScanOne scans the current row into dest and returns the dereferenced value.
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
