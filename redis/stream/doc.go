// Package redisstream provides direct, caller-controlled Redis Streams commands.
//
// The package does not own a Redis client, consumer loop, retry policy, payload
// encoding, stream retention, or consumer-group topology. Callers must use
// idempotent effects because Redis Streams consumer groups are at-least-once.
package redisstream
