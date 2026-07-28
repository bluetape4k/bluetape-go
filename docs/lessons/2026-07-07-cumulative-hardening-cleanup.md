# 누적 hardening cleanup 교훈

## L1: README example이 약한 cleanup contract를 다시 들여오면 안 된다

Issue #429는 retrospective P1 fix 이후 0.12.0까지의 교훈을 다시 적용했다. Toxiproxy
README pair에는 여전히 upstream Redis container를 `testcontainers.TerminateContainer`로
종료하고 Docker network를 test start context로 제거하는 예제가 있었다. 이 예제는 #201
Testcontainers rule에서 벗어나 있었다. Cleanup은 `context.WithoutCancel`에서 파생한 bounded
context를 사용해야 한다.

예방:

- Docker cleanup example은 helper implementation과 맞춘다.
- 각 external resource cleanup에는 fresh bounded cleanup context를 사용한다.
- README snippet이 설명하는 package test보다 약한 계약을 가르치지 않게 한다.

## L2: Docs-only example에도 errcheck 형태의 cleanup이 필요하다

Redis near-cache, Redis coordinator, JWT README example에는 bare `defer Close()` 호출이
있었다. Compile-checked example은 아니었지만, 이전 작업에서 repository errcheck gate를
이미 실패시킨 pattern을 다시 가르치고 있었다.

예방:

- Cleanup failure가 example result를 바꾸지 못하는 README snippet에서는
  `defer func() { _ = value.Close() }()`를 선호한다.
- Example contract를 강화할 때 README와 README.ko pair를 동기화한다.
