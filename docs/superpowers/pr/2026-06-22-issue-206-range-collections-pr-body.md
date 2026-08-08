## 요약

- 닫힌/열린 경계 생성자, containment, overlap 검사, NaN-safe membership,
  zero-value empty 동작을 제공하는 `core.Range`를 추가한다.
- Go-native generic API를 사용하는 `collections.BoundedStack`, `RingBuffer`,
  `Page`, lazy `Permutations` 및 `iter.Seq` API를 추가한다.
- English/Korean README, compile-test한 example, verifier/review 산출물,
  lesson을 갱신한다.

Closes #206.

## 검토

- Step 2-R spec 검토: `P0=0 P1=0`
- Step 3-R plan 검토: `P0=0 P1=0`
- Step 5 verifier: PASS
- Step 6-R 코드 검토: `P0=0 P1=0`

## DoD Status

| 게이트 | 상태 | 증거 |
|---|---|---|
| Worktree | PASS | `issue-206-range-collections`가 `origin/develop`을 기반으로 함. |
| TDD | PASS | RED undefined-symbol test가 구현보다 먼저 실행되었고, 구현 후 대상 GREEN 통과. |
| 대상 test | PASS | `go test -count=1 ./core ./collections` |
| Race 게이트 | PASS | `go test -race -count=1 ./core ./collections` |
| 전체 test | PASS | `go test ./...` |
| Whitespace | PASS | `git diff --check` |
| CI | PASS | `make ci` |
| Lessons | PASS | `docs/lessons/2026-06-22-issue-206-range-collections.md`를 PR 전에 commit. |
