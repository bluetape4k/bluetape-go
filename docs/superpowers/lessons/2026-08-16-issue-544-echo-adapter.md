# Issue #544 Echo adapter 구현 교훈

## 재사용 가능한 규칙

1. **framework 경계를 package 단위로 고정한다.** Echo 의존성은
   `web/echo`에만 두고 `web`, `ratelimit`, `jwt`, `resilience`, `web/gin`은
   Echo를 import하지 않는다. `import_boundary_test.go`를 초기 단계에 두면
   공통 계약이 framework 타입으로 새는 것을 빠르게 발견할 수 있다.
2. **HTTP bridge에는 요청별 carrier를 사용한다.** 공통 `net/http` middleware를
   Echo chain에 연결할 때 request context에 adapter-owned state를 넣고, 다음
   `echo.HandlerFunc`의 오류를 보존한다. 전역 mutable 상태나 동시 요청 간 공유
   포인터는 사용하지 않는다.
3. **Echo 오류 경계는 반환값과 commit 상태를 함께 다룬다.** 허용된 middleware만
   다음 handler를 호출하고, 거부·인증 실패는 safe Problem을 기록한 뒤 chain을
   중단한다. 이미 commit된 응답에는 두 번째 status/body를 쓰지 않으며, nil
   downstream은 404로 fail-closed한다.
4. **retry snapshot은 Echo가 실제로 노출하는 상태만 약속한다.** request/context,
   replayable body, path, params, response header와 adapter-owned store key는
   attempt마다 복원한다. Echo context store의 key 열거 API가 없으므로 임의
   `Set` 전체 복원을 약속하지 않고, 이 한계를 README와 테스트에 명시한다.
5. **보안 callback은 기본 redaction을 유지한다.** JWT 실패 callback에는 설정한
   인증 header와 `Authorization`을 제거한 request 복사본과 분류된 오류만
   전달한다. rate-limit/resilience custom callback은 caller-owned 경계이므로
   raw 원인을 로그에 남길 때 callback 구현자가 redaction해야 한다.
6. **엄격한 입력 검증은 생성 시점과 요청 시점으로 나눈다.** parser와 limiter의
   typed-nil, header·scheme·context key 문법은 constructor에서 거부하고,
   duplicate/comma/whitespace/control/oversized Bearer 값은 요청마다 거부한다.
   context-aware parser가 있으면 request context를 전달하고, 반환 전 취소도
   다시 확인한다.
7. **문서 예제와 lint 계약을 동시에 고정한다.** Go example 함수 이름은 실제
   package symbol 규칙에 맞춰 `Example`과 `Example_migration`으로 두고, `noctx`,
   `staticcheck`, `gofmt`를 테스트 코드까지 적용한다. 영문/한국어 README는
   install, migration, redaction, retry limitation, 검증 명령을 같은 범위로
   유지한다.

## 이번 작업에서 확인한 실패 원인

- Echo `HandlerFunc`는 `error`를 반환하므로 Gin의 `Abort` 상태를 그대로 복제할
  수 없다. HTTP bridge carrier에 `nextErr`와 `handled`를 저장하고, response가
  commit되지 않은 next 오류만 바깥 Echo 오류 경계로 반환했다.
- Go vet은 `ExampleBootstrap`처럼 존재하지 않는 식별자 접미사를 허용하지
  않았다. compile-checked examples는 실제 package example 규칙에 맞춰 이름을
  정하고 README와 계획 문서의 명령도 함께 갱신해야 한다.
- `make lint`의 `noctx`는 테스트 fixture의 `httptest.NewRequest`와
  `http.NewRequest`에도 context 전달을 요구했고, `staticcheck`는 embedded
  context selector를 단순화하라고 지적했다. production/test code를 같은
  context contract로 정리한 뒤 lint가 통과했다.

## 다음 수정자가 피해야 할 선택

- Echo 타입을 framework-neutral core나 Gin adapter로 역수입하지 않는다.
- callback 편의를 위해 Authorization, raw token, provider/backend 오류를 기본
  response나 JWT failure callback에 그대로 전달하지 않는다.
- Echo context store에 대한 전체 key snapshot을 구현한 것처럼 문서화하거나,
  non-replayable body와 commit된 response를 retry하지 않는다.
- `go mod tidy`가 정리한 direct/transitive dependency 구분을 수동으로 되돌리지
  않는다. 변경된 `go.mod`/`go.sum`은 clean-tree `tidy-check`로 재확인한다.

## 근거와 재현 명령

- `go test -count=1 ./web/echo`
- `go test -race -count=1 ./web/echo ./web/gin ./web ./resilience ./ratelimit ./jwt`
- `go vet ./web/echo`
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make ci`
- `git diff --check`
