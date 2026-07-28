package server

import (
	"errors"
	"fmt"
)

// ErrMissingDetail는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
var ErrMissingDetail = errors.New("missing connection detail")

// ConnectionDetails는 Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
type ConnectionDetails map[string]string

// Clone는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (d ConnectionDetails) Clone() ConnectionDetails {
	clone := make(ConnectionDetails, len(d))
	for key, value := range d {
		clone[key] = value
	}
	return clone
}

// Merge는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (d ConnectionDetails) Merge(other ConnectionDetails) ConnectionDetails {
	merged := d.Clone()
	for key, value := range other {
		merged[key] = value
	}
	return merged
}

// Get는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (d ConnectionDetails) Get(key string) (string, bool) {
	value, ok := d[key]
	return value, ok
}

// Require는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (d ConnectionDetails) Require(key string) (string, error) {
	value, ok := d.Get(key)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrMissingDetail, key)
	}
	return value, nil
}
