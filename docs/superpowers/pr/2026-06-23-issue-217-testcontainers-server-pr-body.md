Closes #217

## 요약

- 좁은 started-container contract, 복제된 connection detail, `testcontainers/server`의
  명시적 `ExportEnv`, bounded cleanup/termination을 추가한다.
- PostgreSQL, MySQL, Redis, Kafka, NATS wrapper에
  `StartServer(ctx, testing.TB)`를 추가하면서 기존 typed `Start` helper를 보존한다.
- English/Korean README pair에 shared server 사용법, dynamic mapped port,
  `testing.TB.Setenv` 제한, fixed host-port collision 위험을 문서화한다.

## 검토 증거

- Step 2-R spec review: API 수정 후 P0=0 P1=0.
- Step 3-R plan review: construction-failure cleanup guard 추가 후 P0=0 P1=0.
- Step 6-R 코드 검토: P0=0 P1=0.

## 검증

- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## DoD Status

- [x] 공통 server interface가 global state 없이 host, mapped port, endpoint,
      connection detail, cleanup, manual termination을 노출한다.
- [x] Env export는 명시적이며 `testing.TB.Setenv`로 되돌릴 수 있고
      validation-first로 동작한다. parallel test에서 안전하지 않다는 점도
      문서화했다.
- [x] 기존 wrapper는 현재 `Start` 반환 type을 유지하고 opt-in
      `StartServer`를 추가한다.
- [x] Contract test가 connection detail cloning, missing key, env export
      validation, delegation, termination을 검증한다.
- [x] Wrapper smoke test가 Docker에서 직렬로 통과한다.
- [x] English 및 Korean README가 dynamic mapped port와 fixed-port collision
      위험을 설명한다.
