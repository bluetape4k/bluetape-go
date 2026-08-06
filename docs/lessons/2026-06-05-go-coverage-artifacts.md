# Go Coverage Artifacts Lessons

## Context

- Issue #125는 `bluetape-go` CI와 Nightly에 coverage reporting을 추가한다.
- bluetape4k JVM repository의 reference pattern은 Kover XML generation, artifact
  upload, GitHub Step Summary aggregation이다.

## Decision

- external coverage SaaS integration을 추가하기 전에 Go native coverage를 사용한다.
- 하나의 `make coverage` target으로 `coverage.out`, `coverage-packages.md`,
  `coverage.txt`, `coverage.html`을 생성한다.
- raw local report는 workflow artifact로 upload하고 text summary는
  `$GITHUB_STEP_SUMMARY`에 쓴다.
- Step Summary는 package-level subtotal에 집중하고, full function-level output은
  uploaded artifact에 둔다.

## Learnings

- 이 repository가 single Go module인 동안에는 Kover-style aggregation script가
  필요하지 않다.
- race test를 분리하면 coverage instrumentation과 race detector overhead가 섞이지
  않는다.
- workflow validation에는 `actionlint`와 escaped single quote search가 push 전에
  포함되어야 한다.
- `go tool cover -func` output은 file-path order라 `tail`이 representative coverage가
  아니라 arbitrary last-path function을 보여준다.
- package subtotal은 Go coverage profile에서 source package별 statement block을
  합산해 계산할 수 있다.

## Follow-up

- `0.3.0`의 stable baseline이 생긴 뒤 Coveralls/Codecov upload와 coverage threshold를
  다시 검토한다.
