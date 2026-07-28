// Package sqlkit은 context-aware transaction과 명시적인 row mapping을 위한 작은
// database/sql helper를 제공한다.
//
// sqlkit은 SQL을 호출자 소유이자 가시적인 값으로 유지한다. schema migration 관리, database pool 소유,
// ORM 뒤에 driver 동작 숨기기를 수행하지 않는다.
package sqlkit
