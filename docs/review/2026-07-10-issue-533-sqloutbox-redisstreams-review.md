# Issue 533 - Redis Streams sqloutbox publisher review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

Reviewed diff for `audit/sqloutbox/redisstreams`, README/index updates,
`CHANGELOG.md`, and issue lesson evidence on branch
`feat/issue-533-sqloutbox-redisstreams`.

## 증거

- `go test -count=1 ./audit/sqloutbox/redisstreams` passed after adding the
  provider and tests.
- `go test -count=1 ./audit/sqloutbox ./audit/sqloutbox/redisstreams` passed
  after review fixes.
- `go test -run Example -count=1 ./audit/sqloutbox/redisstreams` passed after
  adding the compile-checked constructor example.
- `go test -race -count=1 ./audit/sqloutbox/redisstreams` passed after review
  fixes.
- `git diff --check` passed after review fixes.
- Context7 confirmed go-redis `XAdd(ctx, &redis.XAddArgs{Stream, Values})`
  returns the appended stream entry ID and exposes retention/idempotent
  production fields that this provider intentionally does not set.
- Existing repo Redis packages use caller-owned `redis.Cmdable` style clients
  and Testcontainers-backed Redis integration tests.
- Existing sqloutbox relay tests define the retry/cancellation contract that
  this provider preserves.
- Step 6-R reviewer lanes found two P1s before fixes: silent stream key
  trimming and typed-nil `Client` acceptance. The implementation now preserves
  exact caller-owned stream keys while rejecting blank values, rejects typed-nil
  clients in `New` and `Publish`, and covers both cases with unit tests.
- P2/P3 documentation polish from reviewer lanes was addressed by documenting
  `entry_json` sensitivity, trusted stream-key ownership, Redis error-text
  persistence, ambiguous Redis write outcomes, and stream field encodings in
  both English and Korean READMEs.

## 발견 사항

P0=0 P1=0

- No P0 findings.
- No P1 findings.

## 메모

- The package has no diagram asset because this provider has no independent
  lifecycle beyond one `XADD` call; the sqloutbox relay sequence diagrams remain
  the owning workflow visualization.
- The provider does not set `MaxLen`, `MinID`, consumer group, or go-redis
  idempotent-production fields to avoid taking ownership of caller topology and
  retention policy.
