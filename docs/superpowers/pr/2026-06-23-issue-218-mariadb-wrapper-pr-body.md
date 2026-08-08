Part of #218

## 요약

- #218 database/storage roadmap matrix를 추가하고 광범위한 후보를 해당
  consumer issue로 명시적으로 연결한다.
- 첫 번째 좁은 database/storage wrapper 범위인 `testcontainers/mariadb`를
  추가한다.
- #217 server contract를 사용하여 `Start(ctx, testing.TB)`,
  `StartServer(ctx, testing.TB)`, `mariadb.dsn` connection detail을
  노출한다.
- dynamic port, env export, cleanup, fixed-port collision을 설명하는
  English/Korean MariaDB README 문서를 추가한다.

## 보류 범위

- MongoDB는 MongoDB backend/package 경계 작업이 시작될 때까지 #198에 묶어
  둔다.
- MinIO, DynamoDB Local, 광범위한 AWS emulator 작업은 #220 및 #61-#64에
  계속 연결한다.
- CockroachDB, ClickHouse, Trino는 #100/#101 SQL dialect 결정에 계속
  종속된다.
- AGE와 graph database는 #220/#50에 계속 연결한다.

## 검토 증거

- Step 2-R spec 검토: P0=0 P1=0.
- Step 6-R 코드 검토: P0=0 P1=0.

## 검증

- `go test -p 1 -count=1 ./testcontainers/mariadb`
- `go test -race -p 1 -count=1 ./testcontainers/mariadb`
- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## DoD Status

- [x] Server matrix가 database/storage 후보를 현재 roadmap issue에 매핑한다.
- [x] 첫 번째 좁은 구현 범위가 #217의 shared lifecycle/property contract를
      사용한다.
- [x] MariaDB wrapper에 README, example smoke test, connection detail
      contract, Testcontainers module을 통한 readiness, cleanup이 있다.
- [x] 보류된 database/storage server가 구체적인 후속 issue 또는 roadmap
      epic에 연결되어 있다.
- [x] Docker-heavy test가 로컬에서 직렬로 통과한다.
