// Package encrypt provides a small AES-GCM facade for local service data.
//
// The package uses Go's standard-library AES-GCM random-nonce AEAD and wraps
// ciphertext in a versioned envelope. Callers own key generation, persistence,
// rotation, and storage. The package never generates durable singleton keys and
// never exposes nonce management.
//
// Use associated data to bind ciphertext to stable context such as tenant,
// entity, column, message type, or protocol version. The same associated data
// must be supplied for decryption.
//
// This package is for byte and UTF-8 string encryption. It is not a password
// hashing helper, JWT signer, MAC-only API, deterministic searchable encryption
// API, KMS envelope system, or file/stream encryption package.
package encrypt
