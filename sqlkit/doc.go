// Package sqlkit provides small database/sql helpers for context-aware
// transactions and explicit row mapping.
//
// sqlkit keeps SQL caller-owned and visible. It does not build SQL, manage
// schema migrations, own database pools, or hide driver behavior behind an ORM.
package sqlkit
