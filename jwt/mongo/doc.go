// Package mongo provides a MongoDB facade for distributed JWT KeyChain repositories.
//
// MongoDB stores JWT signing authority for distributed providers. Use a private,
// trusted MongoDB deployment and keep the MongoDB client lifecycle caller-owned.
package mongo
