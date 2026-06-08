// Package id provides Go-native ID generators for service identifiers.
//
// The package focuses on local ID generation: UUID v4/v7 strings, random and
// monotonic ULID strings, standard seconds-precision KSUID strings, and
// Snowflake int64 identifiers. Generated IDs are identifiers, not authentication
// tokens, authorization secrets, or a standalone security boundary.
package id
