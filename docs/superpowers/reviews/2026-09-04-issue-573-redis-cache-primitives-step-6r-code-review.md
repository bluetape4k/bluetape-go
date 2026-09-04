# 이슈 #573 Redis bucket 및 map cache primitive Step 6-R 코드 리뷰

> 리뷰 경계: 판정과 설명은 한국어로 작성하고, severity 토큰, 상태 토큰, 파일 경로, 라인 번호, 명령, API 식별자는 검증 가능한 증거 앵커로 보존한다.

## 범위와 정확한 기준

- Base: `origin/develop` = `352c0bdbbef7ef41362027e3ecb591ed38be1c32`
- Reviewed implementation head: `811935e45b2e9fb31e58f228d337d22a9f86ac40`
- Branch: `feat/issue-573-redis-cache-primitives`
- Scope: caller-owned Redis client 위의 typed `Bucket[V]`와 `MapCache[V]`,
  serialization/TTL/CAS/get-and-delete, namespace/hash-tag, redacted errors,
  bounded payload contract, Testcontainers integration 및 EN/KO README.
- Initial code-review lane은 P1(payload size/bounded read)과 mutation-path
  bypass를 발견했다. 최종 head는 write preflight와 `GETRANGE`/`EXISTS` bounded
  Lua를 `Get`, `GetAndDelete`, CAS 모두에 적용해 그 finding을 닫았다.
- Architect 독립 lane은 현재 native slot 부족으로 생성할 수 없었다. 따라서
  architect 관점은 main-session architectural fallback으로 수행했으며,
  독립 architect 승인으로 간주하지 않는다.

## 최종 판정 요약

`P0=0`, `P1=0`, `P2=3`, `P3=0`이다. P2는 직접 Redis cancellation race,
두 sibling package 경계의 의도적 중복, 그리고 live 운영 설정 검증 공백이다.
구현·fake·Testcontainers의 PR 준비를 막는 P0/P1은 없다. 최종 verdict는
`PASS for PR / PENDING conditional gates`다.

## 6개 관점과 메인 통합

| 관점 | 판정 | 증거와 경계 |
|---|---|---|
| 성능 | PASS after fix | `redis/bucket/bucket.go:43-67,126-160,220-325` 및 MapCache 동등 경로는 GET 전체 materialization 대신 최대 `MaxPayloadBytes+1`만 관찰한다. write/CAS preflight는 dispatch 전에 끝난다. Cluster throughput/메모리 수치는 측정하지 않았다. |
| 안정성 | PASS after fix | status `{0}`, `{1,payload}`, `{2}` parser와 malformed/empty/oversized legacy value preservation을 fake와 real Redis에서 검증했다. `GetAndDelete`/CAS는 oversized 기존 값을 삭제·교체하지 않는다. real Redis concurrent CAS에서 16-way exact winner가 통과했다. 직접 Redis cancellation race는 `PENDING`이다. |
| 보안 | PASS | caller-owned codec/client/ACL/TLS 정책을 유지하고, 오류·로그에 raw payload나 full key를 넣지 않는다. bounded script가 `GETRANGE`, `EXISTS`, `EVAL`, `SET`, `SETNX`, `DEL` surface만 요구한다. `MaxPayloadBytes`는 1 MiB 기본, 64 MiB 최대이며 oversized legacy 값은 보존된다. |
| 운영/Ops | PASS with PENDING | namespace/hash-tag/TTL, persistence/eviction/maxmemory, retry/timeout/client lifecycle, commit-unknown 및 rollback 경계가 양쪽 README에 있다(`redis/README.ko.md:17-112`). Redis ACL/TLS와 production memory policy는 caller/operator 소유이며 live rollout은 실행하지 않았다. |
| 개발자/API | PASS after fix | `Client` 최소 surface는 `Set`, `SetNX`, `Del`, `Eval`이고 unused `Get` 의존을 제거했다(`redis/bucket/bucket.go:20-23`). exported options/errors/doc comments, generic typed API, examples, `errors.Is` 계약과 양쪽 package README가 일치한다. |
| 사용자/호출자 | PASS | codec 소유권, empty value와 miss 구분, atomic single-key CAS/get-and-delete, oversized error 후 key 보존, cancellation 뒤 no replay가 문서와 테스트에 있다. near-cache invalidation, multi-key transaction, cache stampede는 의도적으로 범위 밖이다. |
| 메인 통합 | PASS | 정확한 base/head에서 P0/P1을 재확인했고 bucket/map sibling 동작·문서·tests가 같은 payload/error 계약을 따른다. architect는 위와 같이 unavailable/fallback으로 기록한다. |

## 닫힌 주요 finding

1. `MaxPayloadBytes`와 `ErrPayloadTooLarge`를 두 primitive에 추가하고 constructor가
   `0`을 1 MiB로 정규화하며 `[1, 64 MiB]` 밖을 거부한다. `prepareWrite`와 CAS
   expected/replacement marshal 결과는 Redis dispatch 전에 거부한다.
2. `Get`은 bounded Lua에서 `GETRANGE(key, 0, max)`와 `EXISTS`를 함께 실행한다
   (`bucket.go:49-52`). `GetAndDelete`와 CAS도 동일하게 최대 `max+1`만 읽고
   status `2`를 반환한다(`bucket.go:53-67`). oversized legacy key는 codec,
   delete, replacement를 거치지 않고 typed error로 반환된다.
3. empty serialized payload는 hit로 보존하도록 `payloadOrEmpty`를 조정했다. 이로써
   zero-length value와 missing key를 구별하면서 기존 serializer의 nil 입력 문제를
   피한다.

## 수용된 P2 및 검증 공백

- fake cancellation/output-plus-error와 real Redis expiry/basic/concurrent CAS는
  검증했지만, 직접 Testcontainers Redis command가 취소되는 race를 별도 재현하지
  않았다(`PENDING`). mutation dispatch 이후에는 `ErrCommitUnknown` 경계를
  유지한다.
- `Bucket`과 `MapCache`의 sibling 구현 중복은 서로 다른 key structural
  namespace와 독립 public package를 유지하기 위한 의도적 경계다. 공통 추상화로
  합치지 않았다(`P2 accepted`).
- live Redis ACL/TLS/cluster/memory-pressure 및 production capacity는 caller와
  operator 환경이 필요해 실행하지 않았다(`PENDING`).
- `gopls`/LSP 증적은 수집하지 않았다(`PENDING`).

## 검증 증거

```text
go test -count=1 ./redis/bucket ./redis/mapcache                 PASS
go test -race -count=1 ./redis/bucket ./redis/mapcache           PASS
go test -run '^Example' -count=1 ./redis/bucket ./redis/mapcache  PASS
go vet ./redis/bucket ./redis/mapcache                           PASS
golangci-lint run ./redis/bucket ./redis/mapcache                PASS (0 issues)
go test -v -p 1 -count=1 -run 'RedisIntegration|IndependentEntryExpiry' ./redis/bucket ./redis/mapcache  PASS
go test -p 1 -count=1 ./redis/... ./cache/...                    PASS
make fmt-check tidy-check vet lint                               PASS (lint 0 issues)
git diff --check origin/develop...HEAD                            PASS
```

Testcontainers는 `colima status`, `docker context show`, `docker info`로 readiness를
확인한 뒤 `redis:7.4-alpine@sha256:6ab0b6e...`를 사용해 다른 Docker suite와
직렬(`-p 1`)로 실행했다. 전체 `make test`의 첫 실행은 변경하지 않은
`lock/redis`의 `TestMutexAcquiresAndUnlocksOwner`와
`TestMutexGeneratedTokenUsesSharedOwnerToken` timeout에서 중단됐으나,
`go test -count=1 ./lock/redis` isolated retry는 통과했다. 전체 make test를
green이라고 주장하지 않으며, 해당 baseline은 별도 리스크로 남긴다.

## PR 전 DoD

| 항목 | 상태 |
|---|---|
| live issue #573/milestone/assignee 확인 | 완료: 0.20.0, `debop`, task/cache/testing/serialization/p2 |
| 정확한 base/head 및 six lenses/main 통합 | 완료: 위 SHA, P0/P1 zero |
| Bucket/MapCache API, bounded payload, typed error, docs | 완료 |
| focused/race/examples/vet/lint/Redis integration/relevant suite | 완료 |
| 직접 Redis cancellation race 및 live ACL/cluster 운영 설정 | `PENDING` |
| PR 생성/원격 CI/Step 7-R/merge | `PENDING`: PR 생성 후 fresh exact-head gate 필요 |

## 최종 상태

`PASS for PR / PENDING conditional gates`. PR을 생성할 수 있으나, live 운영
환경과 직접 cancellation race를 실행했다고 주장하지 않으며, 원격 CI와 merge는
별도 게이트다.
