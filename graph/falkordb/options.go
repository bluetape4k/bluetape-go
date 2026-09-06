package falkordb

import (
	"fmt"
	"time"
)

const (
	defaultMaxRows = 1024
	maxRowsLimit   = 1 << 16
	maxQueryBytes  = 1 << 20
)

// Option은 FalkorDB client 설정을 변경한다.
type Option func(*Client) error

// WithMaxRows는 한 query가 materialize할 최대 row 수를 설정한다.
func WithMaxRows(limit int) Option {
	return func(client *Client) error {
		if limit <= 0 || limit > maxRowsLimit {
			return fmt.Errorf("%w: max rows must be between 1 and %d", ErrInvalidOptions, maxRowsLimit)
		}
		client.maxRows = limit
		return nil
	}
}

// WithTimeout은 FalkorDB server timeout을 위한 query option을 설정한다.
func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) error {
		if timeout <= 0 {
			return fmt.Errorf("%w: timeout must be positive", ErrInvalidOptions)
		}
		client.timeout = timeout
		return nil
	}
}
