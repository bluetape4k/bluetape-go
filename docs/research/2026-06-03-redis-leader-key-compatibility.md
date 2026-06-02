# Redis Leader Key 호환성 결정

## 결론

`bluetape-go` `0.1.0`에서는 Kotlin/JVM `bluetape4k-leader`와 Go Redis leader가
같은 Redis key를 공유하는 혼합 참가자 구성을 지원하지 않는다. 두 구현은 같은
“Redis 기반 leader election” 문제를 풀지만 저장 구조와 소유권 token 계약이 다르다.

## 비교

| 구현 | Redis key | value / 소유권 | TTL / 갱신 |
|---|---|---|---|
| Go `leader/redis` | `bluetape:leader:<group>` | `memberID:random` 문자열 | `SET NX PX`, token 일치 시 `PEXPIRE` |
| Kotlin Lettuce single leader | `lockName` 직접 사용 | Base58 random token | `SET key token NX PX`, token 일치 Lua unlock/extend |
| Kotlin Redisson single leader | Redisson `RLock` 내부 key 구조 | Redisson thread/lock id | 명시적 `leaseTime`, Redisson lock API와 `expire` |

## 근거

- Go 구현은 `leader.Options.KeyPrefix` 기본값 `bluetape:leader`와 `Group`을 조합해
  key를 만들고, `MemberID`를 앞에 붙인 token을 저장한다.
- Kotlin Lettuce `LettuceLock`은 호출자가 넘긴 `lockName`을 Redis key로 직접 쓰며,
  value는 Base58 random token이다.
- Kotlin Redisson `RedissonLeaderElector`는 `redissonClient.getLock(lockName)`의
  `RLock`을 사용하므로 Redisson 내부 lock 표현에 의존한다.

## 운영 지침

Kotlin과 Go 서비스를 동시에 같은 Redis에 붙일 수는 있지만, 같은 leader group을
공유하면 안 된다. 같은 업무를 언어별로 나누어 운영해야 한다면 group/key prefix를
분리하고, 한쪽을 authoritative leader로 정하거나 별도 interop adapter를 설계한다.
