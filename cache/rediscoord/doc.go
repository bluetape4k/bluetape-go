// Package rediscoord coordinates cache loads through Redis.
//
// The package is not a durable Redis cache. It wraps an existing LoadingCache
// and briefly shares one process's loader result during a cold-miss burst.
// Options.MaxResultBytes can bound the transient encoded result envelope.
// Go-native Apache Fory codecs are available from the opt-in fory child package.
package rediscoord
