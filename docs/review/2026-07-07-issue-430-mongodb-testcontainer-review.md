# Issue #430 MongoDB Testcontainers Review

## Scope

- Issue: #430 `feat: Add MongoDB Testcontainers fixture package`
- Parent: #423
- Implementation bucket: #427
- Worktree: `.worktrees/issue-430-mongodb-testcontainer`
- Branch: `issue-430-mongodb-testcontainer`

## Changes Reviewed

- Added `testcontainers/mongodb` with `Start`, `StartServer`, `URIKey`, package
  docs, README, and README.ko.
- Added MongoDB fixture tests for connection details, env export mapping, URI
  startup, driver ping, insert, and find.
- Refactored `jwt` Mongo repository tests to use `testcontainers/mongodb`
  instead of a private `jwtMongoFixture` launcher.
- Removed Mongo fixture cleanup from JWT `TestMain`; Redis fixture cleanup is
  unchanged and outside this issue.
- Added root README and README.ko package-table entries for the new fixture.

## Acceptance Mapping

| Requirement | Status | Evidence |
|---|---|---|
| Add `testcontainers/mongodb` using official Testcontainers-Go MongoDB module | PASS | `testcontainers/mongodb/mongodb.go` calls `github.com/testcontainers/testcontainers-go/modules/mongodb.Run`. |
| Expose caller-useful connection details | PASS | `URIKey` is `mongodb.uri`; `Start` returns the URI; `StartServer` exposes `ConnectionDetails`. |
| Keep MongoDB client lifecycle caller-owned | PASS | Fixture starts containers only; README and tests show caller-owned `mongo.Connect` and `Disconnect`. |
| Refactor JWT tests to shared fixture | PASS | `jwt/mongo_repository_test.go` imports `testcontainers/mongodb`; private `jwtMongoFixture` and `jwtMongoURI` were removed. |
| README pair documents runtime caveats and serial tests | PASS | `testcontainers/mongodb/README.md` and `README.ko.md` document Docker, dynamic ports, caller-owned clients, cleanup, and `-p 1`. |

## 7-Tier Review

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | MongoDB startup remains Docker-bound test work. The shared fixture avoids production hot paths and adds no runtime dependency path beyond existing test deps. P0=0 P1=0. |
| Stability | PASS | Container cleanup uses the existing bounded Testcontainers cleanup path; client disconnect uses bounded `context.WithoutCancel`. P0=0 P1=0. |
| Security | PASS | No production credentials or auth wrapper added; docs keep credentials and clients caller-owned. P0=0 P1=0. |
| Operator/Ops | PASS | README pair documents Docker requirement, dynamic host ports, serial package execution, and env export. P0=0 P1=0. |
| Developer/API | PASS | API matches existing fixture packages: `Start`, `StartServer`, and a single connection-detail key. P0=0 P1=0. |
| User/Caller | PASS | Tests prove the returned URI works with the official MongoDB driver for ping/insert/find. P0=0 P1=0. |
| Integration | PASS | JWT Mongo repository tests consume the shared fixture; no package-local private MongoDB launcher remains. P0=0 P1=0. |

## Validation

- TDD RED: `go test -count=1 ./testcontainers/mongodb` failed with no non-test Go files before implementation.
- `go test -p 1 -count=1 ./testcontainers/mongodb ./jwt ./jwt/mongo`: PASS.
- `go test -race -p 1 -count=1 ./testcontainers/mongodb ./jwt ./jwt/mongo`: PASS.
- `git diff --check`: PASS.
- `golangci-lint cache clean && make ci`: PASS. The cache clean was required
  because the first `make ci` lint pass reported stale diagnostics from the
  removed `issue-429-cumulative-hardening` worktree.

P0=0 P1=0.
