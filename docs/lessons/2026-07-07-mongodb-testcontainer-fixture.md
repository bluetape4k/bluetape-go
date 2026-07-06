# MongoDB Testcontainers Fixture Lessons

## L1: Promote real private fixtures only after a package boundary exists

Issue #430 promoted the private MongoDB Testcontainers setup embedded in JWT
tests into `testcontainers/mongodb`. Earlier database fixture research deferred
MongoDB until a MongoDB package boundary existed; the JWT Mongo repository now
provides that consumer.

Prevention:

- Keep fixture packages demand-backed by an active package consumer.
- Expose only connection details and cleanup contracts; keep clients,
  databases, collections, credentials, indexes, and test data caller-owned.
- Refactor package-private launchers once a shared fixture exists so test
  startup behavior has one public implementation.

## L2: Testcontainers cleanup and client cleanup have different owners

The fixture package owns container termination through `t.Cleanup` and bounded
`internal/testcleanup`. MongoDB client disconnect remains in caller tests.

Prevention:

- Do not hide driver client lifecycle in Testcontainers helpers.
- Use bounded `context.WithoutCancel` cleanup contexts for client disconnects
  when tests may cancel their setup context.
