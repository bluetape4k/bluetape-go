package sqlratelimit

import "errors"

// ErrConfigurationMismatch indicates that an existing bucket uses different options.
var ErrConfigurationMismatch = errors.New("sql rate limiter: configuration mismatch")
