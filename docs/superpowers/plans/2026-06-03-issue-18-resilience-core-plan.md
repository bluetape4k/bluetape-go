# Issue 18 Resilience Core Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 공개 패키지, 조합 가능한 API 표면, docs/spec/plan 산출물, 테스트, README 갱신을 포함한다.
- 진행 방식: 전용 worktree에서 직접 구현하고, local review-delta와 검증 게이트를 통과한다.

## 순서

1. Epic과 하위 이슈 본문을 1차 구현은 first-party로 진행하고 외부 라이브러리는 참고용으로만 둔다고 갱신한다.
2. 기존 코드를 더 확장하기 전에 superpowers research inventory, implementation spec, plan을 먼저 추가한다.
3. `resilience` 패키지 문서와 핵심 조합 타입을 추가한다.
4. fake-sleeper 테스트 지원을 포함한 retry/backoff 구현을 추가한다.
5. cooperative `context.Context` 의미를 지키는 timeout 구현을 추가한다.
6. #18 계약을 고정하는 테스트와 예제를 추가한다.
7. focused tests, repo-wide tests, `go vet`, formatting, diff checks, local review-delta를 실행한다.
8. PR을 게시하기 전에 review finding을 수정한다.

## 리뷰 게이트

다음 항목을 확인한다.

- first-party 구현이 외부 라이브러리 형태나 의존성을 실수로 노출하지 않는지 확인한다.
- composition order가 호출자에게 명확한지 확인한다.
- error unwrapping과 sentinel 동작을 확인한다.
- context cancellation과 timeout 분류를 확인한다.
- #21 event skeleton에 필요한 확장 여지가 충분한지 확인한다.
- 테스트가 deterministic 또는 bounded인지 확인한다.
- README와 research 내용이 서로 일치하는지 확인한다.

## 검증 게이트

완료 전에 다음을 실행한다.

- `go test -count=1 ./resilience`
- `go test -count=1 ./...` 또는 infrastructure failure를 명시적으로 기록한다.
- `go vet ./...`
- `gofmt`
- `git diff --check`
- 구체적 finding이 포함된 local diff review
