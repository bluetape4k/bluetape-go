# Provider 전환에서 독립 상태와 quota 안전성을 구분하기

## 배경과 결정

#611은 local, Redis, SQL provider 전환 시 상태 격리와 Redis 응답 유실을
검증하는 작업이다. public API나 자동 전환 controller를 추가하지 않고 실제
provider를 같은 테스트에 연결했다. milestone 0.23.0의 다른 구현과 섞지 않고 이슈별 PR로
진행하라는 사용자 지시에 따라 테스트·runbook 변경만 분리했다.

## 검증에서 드러난 경계

독립 namespace는 상태 충돌을 막지만 같은 사용자에게 두 bucket의 quota가
동시에 생기는 문제를 막지는 않는다. 9개 전환 조합에서 이전 bucket을 소진한
뒤 신규 bucket이 full burst를 허용하고, 이전 bucket은 소진 상태로 유지되는
것을 확인했다. 두 경로를 켜 놓으면 초기 burst뿐 아니라 refill도 계속 더해진다.
따라서 중복 운영 예산은 추가 burst 숫자만이 아니라 종료 시각과 refill 허용량을
포함해야 한다.

Redis 연결 fixture는 실제 Lua 응답을 읽은 뒤 `io.EOF`를 반환한다. 최초 `Allow`의
`ErrCommitUnknown`과 호출 횟수 1, 후속 요청의 잔여 quota를 함께 확인해야
"오류가 났으니 차감되지 않았다"는 잘못된 판단과 내부 replay를 검출할 수 있다.
검증용 후속 호출은 실패 요청의 재실행이 아니라 별도의 quota 관찰 시나리오다.

첫 운영 리뷰는 `ProcessHook` 바깥에서 일반 오류를 반환하는 테스트가 client
내부의 transport retry를 검증하지 못한다는 P1을 발견했다. `DialHook`의 연결을
감싸 실제 응답을 유실시키는 방식으로 변경했다. 기본 재시도 설정 대조군은 2회
차감됐고, `MaxRetries: -1`은 단일 차감과 `ErrCommitUnknown`을 반환했다.
Cluster의 `MaxRedirects`도 독립적인 재시도 반복이므로 문서에서 따로 제한했다.

처음에는 EOF를 한 번만 주입했으나 RESP3 push 사전 읽기가 이를 소비하고 다음
읽기는 timeout으로 끝났다. go-redis `push/processor.go`의 `PeekReplyType` 오류
처리와 `redis.go`의 후속 읽기를 확인한 뒤 해당 연결의 읽기에 계속 EOF를
반환하도록 수정했다. 연결 종료는 pool과 호출자의 cleanup에 맡긴다.

## 구현·검증 기록

- 생산 코드·dependency 변경 없이 테스트를 추가했으므로 production RED/GREEN은
  해당하지 않는다. 기존 동작의 보강 테스트이며 신규 알고리즘 구현으로 보고하지 않는다.
- `go test -p 1 -count=1 -timeout=5m ./ratelimit/...`: 4개 패키지 통과.
- `go test -race -p 1 -count=1 -timeout=5m ./ratelimit/...`:
  transport fixture 보강 후 4개 패키지 통과.
- 첫 lint는 helper의 `context.Context`가 첫 번째 인자가 아니라는 오류를 검출했다.
  인자 순서를 수정한 뒤 `golangci-lint run ./ratelimit/...`에서 0건을 확인했다.
- 라우팅 배포, 실제 진행 중인 애플리케이션 작업의 drain, 운영 환경의 전환은
  검증하지 않는다. README는 이를 호출자 책임으로 명시한다.
- 운영 관점 재리뷰는 재시도 검증 P1과 수동 수용 기준 P2의 해소를 확인했다.
  로컬 `make ci`는 이 보강을 반영하기 위해 진행 도중 중단했으므로 전체 통과로
  보고하지 않는다. 최종 commit의 전체 검증은 GitHub CI에서 별도로 확인한다.

## 다음 작업의 예방 규칙

1. 전환 테스트는 신규 provider의 성공만 보지 말고 이전 provider의 남은 상태와
   같은 cohort의 총 허용량을 확인한다.
2. 응답 유실은 실제 mutation 이후에 주입하고 호출 횟수와 backend 상태를 함께 본다.
3. retry 설정은 라이브러리와 caller-owned client 양쪽에서 확인하고 실제 transport
   경계에서 재현한다. hook 바깥에서 만든 일반 오류를 내부 재시도 증거로 쓰지 않는다.
4. 테스트 helper도 `context.Context`를 첫 번째 인자로 두고 처음 추가한 시점에 lint한다.
5. 이슈별 PR 요청에서는 Epic 전체를 하나의 브랜치에 합치지 않고, 모든 하위 이슈의
   구현·PR·CI 상태를 별도로 보고한다.
