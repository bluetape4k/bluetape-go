# MongoDB Testcontainers fixture 교훈

## L1: Package boundary가 생긴 뒤에만 실제 private fixture를 승격한다

Issue #430은 JWT test에 묻혀 있던 private MongoDB Testcontainers setup을
`testcontainers/mongodb`로 승격했다. 이전 database fixture research는 MongoDB package
boundary가 생길 때까지 MongoDB를 보류했으며, 이제 JWT Mongo repository가 그 consumer가 됐다.

예방:

- Fixture package는 active package consumer의 수요로 뒷받침한다.
- Connection detail과 cleanup contract만 노출한다. Client, database, collection,
  credential, index, test data는 caller-owned로 둔다.
- Shared fixture가 생기면 package-private launcher를 refactor해 test startup behavior가
  하나의 public implementation을 갖게 한다.

## L2: Testcontainers cleanup과 client cleanup의 owner는 다르다

Fixture package는 `t.Cleanup`과 bounded `internal/testcleanup`을 통해 container termination을
소유한다. MongoDB client disconnect는 caller test에 남긴다.

예방:

- Testcontainers helper 안에 driver client lifecycle을 숨기지 않는다.
- Test가 setup context를 cancel할 수 있을 때 client disconnect에는 bounded
  `context.WithoutCancel` cleanup context를 사용한다.
