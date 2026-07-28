// Package redis JWT key provider repository 계약과 호출자 사용 경계를 설명한다.
//
// Redis keyspace는 trusted service boundary 안에서 사용하며, key material payload와
// current KID pointer의 저장, 조회, 회전 계약을 유지한다.
package redis
