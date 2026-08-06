# Issue #485 leader/mongo 단일 elector 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


Issue: [#485](https://github.com/bluetape4k/bluetape-go/issues/485)  
Spec: `docs/superpowers/specs/2026-07-09-issue-485-leader-mongo-elector-design.md`  
Date: 2026-07-09

## 작업

| 작업 | 복잡도 | 파일 | 검증 |
|---|---:|---|---|
| T1 생성 패키지 API 및 document model | 높음 | `leader/mongo/*.go` | `go test -count=1 ./leader/mongo` |
| T2 구현 acquire, renew, release, leader read, 및 local lifecycle | 높음 | `leader/mongo/elector.go` | `go test -count=1 ./leader/mongo` |
| T3 추가 integration 및 lifecycle 테스트 | 높음 | `leader/mongo/*_test.go` | `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb` |
| T4 추가 README pair, root 패키지 index, 및 leader backend notes | 보통 | `leader/mongo/README*.md`, root/leader README pair | `rg -n "leader/mongo|MongoDB" README*.md leader` |
| T5 추가 lesson 및 code-review artifact | 보통 | `docs/lessons`, `docs/review` | `git diff --check` |
| T6 실행 verification 및 prepare PR | 보통 | 모든 changed files | targeted 테스트, race, `make fmt-check`, `make tidy-check`, `make vet`, PR CI |

## 구현 메모

- Apply `bluetape-go-patterns` for context cancellation, race/stress coverage,
  owner-token semantics, 및 공개 API docs.
- 유지 Testcontainers-backed verification serial.
- 다음을 하지 않는다: add dependencies; MongoDB driver 및 Testcontainers MongoDB are already
  present in `go.mod`.
- 사용 `_id` as the unique leader key 및 a TTL index on `lease_until` 만 for
  cleanup.
- 사용 client-clock `lease_until` in this first slice 및 document bounded
  clock-skew assumptions.

## 위험 점검

| 위험 | 완화책 |
|---|---|
| Duplicate upsert races under contention | Catch `mongo.IsDuplicateKeyError` 및 retry until context cancellation. |
| Renewal resurrects expired local ownership | Renewal filter includes `lease_until > now`. |
| Resign deletes a new owner | Delete filter includes local `token`. |
| Goroutine leak | `Resign` cancels renewal 및 waits on `done`; failed renewal closes `done`. |
| TTL overclaim | README states TTL is cleanup 만; 테스트 prove expired takeover without TTL deletion. |

## 필수 검증

- `go test -count=1 ./leader ./leader/mongo`
- `go test -race -count=1 ./leader ./leader/mongo`
- `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `git diff --check`

