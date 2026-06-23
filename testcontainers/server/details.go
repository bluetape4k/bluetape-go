package server

import (
	"errors"
	"fmt"
)

// ErrMissingDetail reports that a requested connection detail key is absent.
var ErrMissingDetail = errors.New("missing connection detail")

// ConnectionDetails contains string connection properties for a started server.
type ConnectionDetails map[string]string

// Clone returns an independent copy of the connection details.
func (d ConnectionDetails) Clone() ConnectionDetails {
	clone := make(ConnectionDetails, len(d))
	for key, value := range d {
		clone[key] = value
	}
	return clone
}

// Merge returns a new details map with other values overriding receiver values.
func (d ConnectionDetails) Merge(other ConnectionDetails) ConnectionDetails {
	merged := d.Clone()
	for key, value := range other {
		merged[key] = value
	}
	return merged
}

// Get returns the value for key.
func (d ConnectionDetails) Get(key string) (string, bool) {
	value, ok := d[key]
	return value, ok
}

// Require returns the value for key or an ErrMissingDetail-wrapped error.
func (d ConnectionDetails) Require(key string) (string, error) {
	value, ok := d.Get(key)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrMissingDetail, key)
	}
	return value, nil
}
