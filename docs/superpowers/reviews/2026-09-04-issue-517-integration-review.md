# Issue #517 통합 검토

## 판정

다섯 child issue의 PR은 모두 exact-head CI를 통과한 뒤 `develop`에 squash
merge되었다. 병합 후 기준점은 `c59dfab2e84bae19ba8192db186480d536274d54`이며,
변경된 여섯 Go package의 normal/race 검증과 정적 게이트는 PASS다. 최종
통합 판정은 `P0=0, P1=0`이다.

저장소 전체 Testcontainers suite는 macOS Colima의 host-port forwarding과
기존 Redis/Postgres/Mongo/etcd/Kafka fixture가 일괄 실행 중 간헐적으로
`HTTP/1.1 400 Bad Request`, `EOF`, connection reset/refused 또는 2초
concurrency timeout을 내어 `make test`/`make race`가 종료되었다. 같은
실패 패키지를 단독 실행하면 통과하고 이번 변경 package에는 재현되지 않아,
이번 변경의 회귀로 판정하지 않는다. 이 환경 한계는 아래 증적에 남기며
전체 suite green으로 표현하지 않는다.

## 병합 증적

| PR | merged source head | merge commit | exact-head CI |
|---|---|---|---|
| #718 | `79ac508f25c4aefba99287395b1b0c5e5f09fa89` | `aea9d34541ada48659a7e50c1f192dda43a86029` | run `33791942928` PASS |
| #719 | `86257fecfbf6705e40fdfbea13eb4383ae409f7e` | `b7f4b93ecb0cd252021be432aaef056536c94c0c` | run `33803515189` PASS |
| #720 | `aa0188b60298ab6c6f672d431f4f99eff5b5d885` | `f14e6458410887d7c51b8fa3b716790160b66a24` | run `33806441639` PASS |
| #721 | `2701009d8cccfe570f0e6ebb77683fc7af082c7f` | `7db5c189a7a6466ad70d85b6a537e89f681795d0` | run `33834742104` PASS |
| #722 | `294e8cd946d1b52345d966b4c3e521471cdf6d8c` | `c59dfab2e84bae19ba8192db186480d536274d54` | run `33836029023` PASS |

모든 PR의 live state는 `MERGED`, base는 `develop`, merge commit은 위 표와
일치하고, merge 직전 review thread는 0개였다. 병합 전 rebase 충돌은 root
README/package index를 보존하여 해결했으며, #721의 `go.mod`는 `go mod tidy`
정렬을 다시 적용한 뒤 CI run `33834742104`에서 확인했다.

## 병합 후 검증

| 검증 | 결과 |
|---|---|
| `git fetch origin develop; git pull --ff-only` | PASS; `HEAD == origin/develop == c59dfab2e84bae19ba8192db186480d536274d54` |
| `make fmt-check tidy-check vet lint` | PASS; `golangci-lint`: `0 issues` |
| `go test -race -p 1 -count=1 ./messaging/sqsextended ./examples/cloudwatch ./s3vectors ./secretsmanager ./ssm ./rds/auth` | PASS |
| `go test -race -count=1 ./leader/sql -run '^TestPostgresFaultRecovery$'` 및 3회 반복 | PASS; 변경 외 fixture의 일시 실패를 재현하지 않음 |
| `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false go test -p 1 -count=1 ./cache/redisnear ./redis ./redis/stream ./ratelimit/sql ./sqlkit` | PASS; 단독 fixture 경계 확인 |
| 첫 `make ci`의 `make test` 단계 | PASS; 이후 동일 명령 재실행은 비변경 fixture의 간헐적 오류로 실패 |
| `make test` / `make race` 전체 일괄 재실행 | PENDING 환경 증적; 비변경 Testcontainers fixture의 간헐적 오류로 실패 |

전체 실패의 대표 오류는 `cache/redisnear` RESP3 `HTTP/1.1 400 Bad Request`/
`EOF`, `ratelimit/sql`·`sqlkit`·`leader/sql` Postgres connection
refused/reset, `leader/etcd` readiness timeout, `leader/mongo` connection
reset, `testcontainers/kafka` connection reset이다. Colima는 실행 중이고
Docker `29.2.1`에 연결되었으며, 사용자 소유로 보이는 기존 컨테이너와
worktree는 정리 대상에서 제외했다.

## 변경 계약 검토

- #523 SQS/S3 envelope는 512 MiB aggregate와 digest 재사용, preflight,
  cancellation 이후 orphan/receipt 상태를 명시하고 body close를 보장한다.
- #524 CloudWatch 예시는 metric/log/entity의 범위·finite 값·timestamp span을
  preflight하고 partial rejection을 거부 index만 재시도하는 caller 계약으로
  남긴다.
- #525 S3 Vectors adapter는 AWS 서비스 한도(Put 500, Get 100, dimension
  4096, metadata 40 KiB, request 20 MiB)를 clone 전에 검사하고, key를 valid
  UTF-8 최대 1,024 characters로 검증한다. 자세한 근거는 [Amazon S3
  Vectors](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-vectors-vectors.html)와
  [S3 Vectors limitations](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-vectors-limitations.html)에
  둔다.
- #538 Secrets Manager/SSM은 caller-supplied bounded TTL cache만 사용하고
  typed-nil, expiry, `%+v`/`%#v` redaction을 검증한다.
- #539 RDS IAM helper는 strict DNS/IP host grammar, port/range 검증과 token
  redaction을 제공하며 credentials, IAM policy, pool, retry, refresh는
  caller/operator 소유다.

여섯 독립 review lane과 main-session integration review에서 P0/P1은 0건이다.
partial retry, SQS visibility/reconciliation, live AWS/IAM/provisioning은
문서화된 P2 또는 N/A이며 구현 범위를 확장하지 않는다.

## 운영 표면 변경

관찰성 SDK 경계의 service limit, batching/order/cardinality/deprecation,
preflight와 fake-first 요구를 `$bluetape-go-patterns`에 추가했다. managed
source/live parity는 clean이며 self-audit는 `PASS=6 WARN=1 FAIL=0`(기존
dotfiles warning), contracts validator는 `Contract issues: 0`이다. 변경은
chezmoi commit `6974520b005857ae1eead3afcfbd80f07bf0eed7`로 push했다.

## 잔여 범위

실제 AWS SQS/S3/CloudWatch/S3 Vectors/Secrets/SSM/RDS endpoint와 IAM
provisioning은 credentials·외부 상태를 요구하므로 이 통합 lane의 N/A다.
CI에서 각 PR의 exact head가 PASS한 사실은 live AWS proof나 macOS 전체
Testcontainers green을 대신하지 않는다.
