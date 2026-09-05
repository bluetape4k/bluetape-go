# 진행 상황

기준 시각: 2026-09-05 KST
범위: `v0.21.0` Go stable release 준비.

## 현재 대상 릴리스

`v0.21.0`은 framework-neutral web helper, middleware conformance, Gin/Echo
adapter, 선택적 JWKS provider와 Echo 후속 안전성 수정을 묶는 web API helper
릴리스입니다. Framework abstraction, global logger/MDC, JWE/OIDC discovery,
application route ownership은 범위에 포함하지 않습니다.

## 이력 경계

`v0.20.0`의 `main` projection tree에는 #541~#545, #688, #689의 구현이
이미 포함되어 있습니다. 그러나 `v0.20.0` 변경 기록은 해당 web API helper
범위를 별도 릴리스 항목으로 설명하지 않았습니다. `v0.21.0`은 이 기능군을
공식 릴리스 범위로 명시하고, `v0.20.0..develop`의 실제 source delta인
Echo 후속 #692~#694를 함께 배포합니다. `v0.20.0` 사용자는 Echo 후속 수정이
필요할 때 `v0.21.0`으로 올리면 됩니다.

## 현재 상태

- release-preparation 기준 `develop`은
  `aa2ffc78e7656194b47eb8aedff605acf3dc8df6`이며 `origin/develop`과
  일치합니다.
- release 기준 `origin/main`과 `v0.20.0^{}`는
  `eedd2d50aa4729840f1bb74fc83e8c4e35b07337`입니다.
- milestone #33 `0.21.0`은 `CLOSED`, `open_issues=0`,
  `closed_issues=22`입니다. Epic #540과 계획된 하위 이슈는 모두
  `COMPLETED`이며 #709는 `NOT_PLANNED`입니다.
- local/remote `v0.21.0` tag와 GitHub Release는 없습니다. 최신 외부
  release는 `v0.20.0`입니다.
- baseline `develop` GitHub CI
  [33928663952](https://github.com/bluetape4k/bluetape-go/actions/runs/33928663952)는
  exact head `aa2ffc78e7656194b47eb8aedff605acf3dc8df6`에서 성공했습니다.
- release-preparation tree에서 세 번의 `make ci` normal test와
  `make check-bench-web-gin`은 통과했습니다. monolithic race 단계는 서로 다른
  PostgreSQL Testcontainers test에서 Colima mapped-port 연결이 60초 후
  간헐적으로 reset/refused되어 종료 코드 0을 얻지 못했습니다.
- `go test -race ./leader/sql -count=1 -v` 전체와 `ratelimit/sql`의 모든
  top-level test를 한 테스트당 별도 프로세스로 직렬 실행한 검증은 통과했고,
  각 container의 create/start/ready/terminate lifecycle도 확인했습니다. 따라서
  로컬 gate는 엄격한 infrastructure 직렬화 증거로 충족하며, exact-head
  GitHub CI와 Testcontainers Nightly를 원격 필수 증거로 유지합니다.
- exact release-preparation head의 GitHub CI, 실제 Testcontainers Nightly,
  release-preparation PR, `main` promotion, tag 및 GitHub Release는
  아직 실행하지 않았습니다.

## 릴리스 순서

1. `CHANGELOG.md`, README locale pair, 이 WIP 및 release checklist의 의미와
   link를 검증합니다.
2. release-preparation 최종 tree에서 formatter, tidy, vet, lint, normal test,
   직렬 race test와 Gin benchmark contract를 검증합니다.
3. `chore/v0.21.0-release-prep -> develop` PR을 만들고 exact-head CI,
   review 및 Testcontainers Nightly를 확인합니다.
4. fresh 승인 뒤 release-preparation PR을 `develop`에 반영합니다.
5. 검증된 `develop` tree를 `main`으로 promotion합니다. 직접 PR이
   충돌하면 tree-equivalent protected-branch projection fallback을 사용합니다.
6. `main` exact commit에 서명된 annotated `v0.21.0` tag를 생성·push하고
   한국어 GitHub Release를 게시합니다. 각 side effect는 별도 fresh authority
   gate를 유지합니다.
7. tag와 Release live read-back, branch sync, task-owned worktree/branch 정리를
   끝낸 뒤 milestone `0.22.0` 작업을 시작합니다.

## 비범위

- downstream consumer의 `go.mod` 업데이트는 이번 요청에 포함하지 않습니다.
- `0.22.0` 구현은 `v0.21.0` release identity가 검증되기 전 시작하지
  않습니다.
