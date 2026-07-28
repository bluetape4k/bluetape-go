# Issue #217 Testcontainers 서버 추상화 조사

Issue: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Milestone: `0.6.5`  
Date: 2026-06-23

## 현재 Go 증거

- 기존 래퍼는 다음 경로에 있다.
  - `testcontainers/postgres`
  - `testcontainers/mysql`
  - `testcontainers/redis`
  - `testcontainers/kafka`
  - `testcontainers/nats`
- #216은 이 래퍼들을 먼저 보강했다.
  - `internal/testcleanup.Terminate`는 제한 시간이 있는 정리를 제공한다.
  - `internal/testcleanup.FormatStartError`는 시작 실패를 분류한다.
  - 각 래퍼는 문서화된 연결 상세 키를 노출한다.
  - Docker 기반 패키지 테스트는 직렬로 실행된다.
- 현재 래퍼 API는 서비스별 값을 직접 반환한다.
  - PostgreSQL: connection string
  - MySQL: DSN
  - Redis: address
  - Kafka: broker list
  - NATS: URL
- 현재 래퍼는 `testing.TB`를 통해 실패하므로 Go 테스트 헬퍼 스타일과 맞다.
  아직 오류를 반환하는 공개 lifecycle API는 제공하지 않는다.

## Kotlin 패리티 증거

- GNO는 관련 bluetape4k-projects 설계 노트
  `docs/superpowers/specs/2026-04-03-testcontainers-design.md`를 찾았다.
- Kotlin `PropertyExportingServer`는 다음을 정의한다.
  - `propertyNamespace`
  - `propertyKeys()`
  - `properties()`
  - `registerSystemProperties()`
  - `writeToSystemProperties()`
- Kotlin `GenericServer`는 Testcontainers `ContainerState` 위에서 `port`와
  `url`을 노출한다.
- Kotlin 설계는 JVM 고유 문제를 해결했다. 숨은 전역
  `System.setProperty` 쓰기는 되돌리기 어렵기 때문에, 최종 Kotlin
  인터페이스는 되돌릴 수 있는 등록 방식을 사용한다.

## Go 패리티 결정

JVM system-property 동작을 직접 이식하지 않는다.

Go에서는 다음 방식을 사용한다.

- 반환된 연결 상세 맵을 기본 export 계약으로 둔다.
- 선택적 환경 변수 export는 `testing.TB.Setenv`를 사용해 테스트가 정리를
  소유하고 되돌릴 수 있게 한다.
- singleton/global 상태 대신 명시적 server 값을 사용한다.
- 패키지별 래퍼는 기존 `Start(ctx, testing.TB)` 함수의 소스 호환성을
  유지한다.

이는 `docs/research/2026-06-21-issue-202-source-parity-matrix.md`와 맞다.
해당 문서는 generic server/property export를 #215/#217로 라우팅하고,
JVM system property export는 Go 범위에서 제외한다고 명시한다.

## 공식 Testcontainers-Go 증거

Context7은 공식 라이브러리를 `/testcontainers/testcontainers-go`로 해석했다.
설치된 모듈은 `github.com/testcontainers/testcontainers-go v0.42.0`이다.

관련 API:

- `testcontainers.Container`
  - `Host(context.Context) (string, error)`
  - `MappedPort(context.Context, string) (network.Port, error)`
  - `PortEndpoint(context.Context, port string, proto string) (string, error)`
  - `Endpoint(context.Context, proto string) (string, error)`
  - `Terminate(context.Context, ...TerminateOption) error`
- `testcontainers.GenericContainer(ctx, GenericContainerRequest)`는
  `ContainerRequest`에서 generic container를 시작한다.
- `testcontainers.CleanupContainer(testing.TB, Container)`는 nil-safe이며,
  container 생성 직후 반환 오류를 검사하기 전에 호출하는 용도다.

이 저장소는 이미 `internal/testcleanup`으로 제한 시간이 있는 정리를
소유하므로, #217은 raw `CleanupContainer`로 대체하지 않고 해당 헬퍼를
재사용할 수 있다.

## 채택 / 차용 / 제외

| Source | Decision | Rationale |
|---|---|---|
| Kotlin `PropertyExportingServer` key contract | Borrow | 키 탐색과 value map은 래퍼 전반에서 유용하다. |
| Kotlin `registerSystemProperties()` | Adapt | Go 대응 방식은 전역 JVM property가 아니라 `testing.TB.Setenv`여야 한다. |
| Kotlin `GenericServer` inheritance shape | Skip | Go는 넓은 container framework보다 작은 interface와 composition이 맞다. |
| Testcontainers-Go `Container` host/port APIs | Adopt | 공식 API가 이미 host, mapped port, endpoint, terminate를 노출한다. |
| Testcontainers-Go `CleanupContainer` | Borrow concept | Nil-safe cleanup 개념은 유용하지만, repo의 bounded cleanup이 로컬 계약으로 남는다. |

## 설계 제약

- 기존 `Start(ctx, tb)` 함수는 소스 호환성을 유지해야 한다.
- 새 shared package는 새 dependency를 추가하지 않아야 한다.
- 환경 변수 export는 opt-in이고 되돌릴 수 있어야 한다.
- 연결 상세 키는 안정적이고 문서화되어야 한다.
- Docker 기반 테스트는 계속 직렬로 실행한다.
- 추상화 contract test는 Docker를 요구하지 않아야 한다.
- 실제 래퍼 smoke test는 Docker와 `-p 1`을 계속 사용한다.

## Spec용 조사 요약

가장 작은 안전 설계는 새 shared `testcontainers/server` package다.

- name, host, mapped ports, URLs/endpoints, connection details, cleanup,
  termination을 다루는 `Server` interface;
- `testcontainers.Container`를 감싸는 concrete `Started` adapter;
- clone/merge/string value를 위한 `ConnectionDetails` 헬퍼;
- `tb.Setenv`를 사용하는
  `ExportEnv(testing.TB, ConnectionDetails, map[string]string)`;
- fake container 기반의 재사용 가능한 contract test;
- 기존 래퍼에는 새 `StartServer(ctx, tb)` 함수를 추가하고, 기존
  `Start(ctx, tb)` 함수는 delegation하거나 현재 반환값을 보존한다.
