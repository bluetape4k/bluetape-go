// Package falkordb 는 caller-owned Redis client를 통한 제한된 FalkorDB
// OpenCypher 경계를 제공한다.
//
// 공식 falkordb-go/v2의 고수준 API가 package-global background context를
// 사용하므로, 이 adapter의 외부 명령은 redis.UniversalClient.Do(ctx, ...)로
// 발행한다. ORM, transaction facade, TinkerPop abstraction은 제공하지 않는다.
package falkordb
