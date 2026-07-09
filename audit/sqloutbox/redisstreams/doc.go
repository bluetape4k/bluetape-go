// Package redisstreams publishes sqloutbox records to Redis Streams.
//
// The package only appends one stream entry per sqloutbox publish attempt. The
// Redis client, stream retention, consumer groups, authentication, TLS, replay,
// and duplicate handling policies remain caller-owned.
package redisstreams
