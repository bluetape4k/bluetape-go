// Package redis provides a Redis facade for distributed JWT KeyChain repositories.
//
// Redis stores JWT signing authority for distributed providers. Use a private,
// trusted Redis deployment and keep the Redis client lifecycle caller-owned.
package redis
