# Issue 85 LeaderGroupElector Plan

Spec: `docs/superpowers/specs/2026-06-04-issue-85-leader-group-elector-spec.md`
Issue: #85

## Tasks

1. Add core API.
   - Add `leader/group.go` with `GroupOptions`, `GroupElector`, and
     `GroupOptions.Normalize`.
   - Add focused option validation tests.

2. Add Redis group elector implementation.
   - Add `leader/redis/group.go`.
   - Reuse existing token generation style.
   - Add acquire, renew, release, and status Lua scripts using Redis server
     `TIME`.
   - Keep lifecycle state under mutex and run a renewal loop while owned.
   - Make `Campaign` context-bounded and contention-aware.

3. Add Redis Testcontainers coverage.
   - Cover max slot acquisition, contention timeout, duplicate campaign,
     repeated resign, status counts, expiry reclamation, renewal loss, and
     foreign slot protection.
   - Keep tests serial inside one `go test` process.

4. Add examples.
   - Add a compile-checked `ExampleNewGroup` for copy-paste API usage.
   - Add a Testcontainers-backed N-worker batch coordination smoke test.

5. Update docs.
   - Update `leader/doc.go` and `leader/redis/doc.go`.
   - Update `README.md` and `README.ko.md` together.

6. Review and validation.
   - Run `gofmt`.
   - Run `go test -count=1 ./leader ./leader/redis`.
   - Run targeted example test for the new example.
   - Run `make ci`.
   - Run `git diff --check`.
   - Run local 7-Tier implementation review and record P0/P1=0.

7. Lessons, commit, and PR.
   - Add `docs/lessons/2026-06-04-leader-group-elector.md`.
   - Commit with Lore trailers.
   - Create PR assigned to `debop`, milestone `0.2.0`, linking #85.
   - Check PR status rollup and update PR body DoD.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Spec requirements mapped | Done | Every issue acceptance criterion has a task. |
| Task order implementable | Done | API first, backend second, tests/docs after behavior exists. |
| Testcontainers coverage assigned | Done | Redis backend scenarios are explicit. |
| Docs locale pair assigned | Done | README English/Korean updates included. |
| Verification commands concrete | Done | Targeted tests, `make ci`, and diff check listed. |
