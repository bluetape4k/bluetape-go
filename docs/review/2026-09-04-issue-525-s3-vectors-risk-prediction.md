# #525 S3 Vectors 위험 예측

## SPW-01 — 범위와 근거

대상은 `s3vectors` thin adapter와 fake-first 테스트·README다. 근거는 issue
#525, 설계/계획 artifact, AWS SDK for Go v2 `service/s3vectors@v1.13.0` source다.

## 예상 위험과 예방

| 위험 | 심각도 | 예방/검증 |
|---|---|---|
| SDK client를 넓게 감싸 vector DB abstraction으로 변형 | P1 | eight-method interface와 raw SDK input/output 유지, API review |
| typed-nil client panic | P1 | `reflect.IsNil` 기반 constructor test |
| caller cancellation보다 SDK 오류를 우선 반환 | P1 | pre-call/post-response cancellation fake test |
| metadata/filter 또는 vector 값이 오류 문자열에 노출 | P1 | redaction 및 `%+v` test |
| caller input slice/map을 fake가 보유해 data race/변조 | P1 | request snapshot deep-copy test와 `go test -race` |
| nil/malformed output을 성공으로 반환 | P1 | operation별 required output validation test |
| 기본 CI가 실제 AWS/emulator에 의존 | P1 | fake-only default test와 README explicit limitation |
| SDK의 future union/type 확장을 임의 해석 | P2 | `VectorData` opaque forwarding, finite float32 validation만 수행 |

## N/A 근거

이 package는 goroutine, timer, retry loop, cache, credential, logger, DB pool,
provisioning을 만들지 않으므로 lifecycle/leak/cache/worker 관련 stress와
operational logger 검사는 범위 밖이다. live AWS test는 검증된 credentials와
환경이 없으므로 기본 DoD에서 N/A이며 성공으로 세지 않는다.

## SPW-02 — 위험 계약

각 P1 위험에 조기 검증, context ownership, fake behavior와 명시적 문서
대응을 연결했다. P0 silent data loss는 raw SDK forwarding과 output/error
계약을 보존해 방지한다.

## SPW-03 — 한국어 기술 문체

위험, 영향, 예방을 구분해 한국어 엔지니어링 문체로 작성하고 API/SDK token은
그대로 보존했다.

## SPW-04 — 검증 추적성

각 위험은 계획의 구현 단계와 최종 명령(`go test`, `go test -race`, `go vet`,
`git diff --check`)으로 추적된다. package test, example test, race test와
`go vet ./s3vectors`는 모두 통과했다. 추가로 `go test -p 1 -count=1 ./...`,
`make fmt-check`, `make vet`, `make lint`, `go mod tidy` idempotence와
`git diff --check`도 통과했다. `make tidy-check`는 아직 dependency diff가
커밋되지 않은 작업 tree에서 baseline diff를 감지해 종료하므로, 그 명령의
실패는 tidy 불안정성이 아니라 이 branch의 미커밋 변경을 반영한다.

## SPW-05 — read-back

구현 후 이 문서를 다시 읽어 예상 위험이 실제 diff와 테스트에 반영됐는지
확인했다. 새 assumption은 검증된 S3 Vectors live/emulator 환경이 없다는 점이며,
이를 lesson에 기록하고 지원 주장으로 확대하지 않았다. 현재 P0/P1 미해결 finding은
없다.
