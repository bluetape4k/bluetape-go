# #534 Cuckoo 구현 검토

## 검토 범위와 근거

기준은 develop `51be48337427323db96d3d3ee944e6331e313b28`이다. 승인 spec `1218112`,
계획 `2639d19` 이후의 `probabilistic/redis/cuckoo.go`, 세 테스트 파일, README EN/KO,
설계·계획 정정과 교훈을 검토했다. 기존 Bloom/HLL 코드와 key, Go 의존성은 변경하지 않는다.

독립 reviewer는 테스트·Docker·외부 네트워크·쓰기를 수행하지 않았다.
실행 증거와 최종 통합은 main 소유다. 노출된 code-reviewer/verifier 역할 설정은
`gpt-5.6-luna` / `max`이며 실제 추론량을 별도로 측정했다는 뜻은 아니다.

| 관점 | 실행 주체 | 근거와 결과 |
|---|---|---|
| 성능 | native code534_performance | Add 64 KiB 상한, 단일 Do, no-retry, bounded64. P0–P3=0, 성능 우위 주장 없음 |
| 안정성 | native code534_stability, 재검토 | execute 취소·unknown 우선순위와 caller-owned 수명 확인. 코드 P0/P1=0, 최종 실서버 재실행은 main 책임 |
| 보안 | main inline fallback review | NewCuckoo 입력·key 격리, 별도 argv, OpError redaction과 원인 보존, typed-nil 전 종류 확인 |
| 운영 | main inline fallback review | opt-in module/plain 미지원 분리, image pin, startup/IO/deadline/cleanup, quota와 replay 책임 명시 |
| 개발자/API | main inline fallback review | 다섯 메서드와 Go doc, RESP2/3 거부, strict count/bool 결과, 기존 API 무변경, 새 의존성 없음 |
| 사용자/호출자 | main inline fallback review | 예제 성공 장부, unknown 뒤 자동 삭제 금지, COUNT 과대 추정, README EN/KO 계약 일치 |
| 통합 | main | 원본·실패 회귀·리뷰 지적·최종 검증을 아래와 같이 구분 |

네 native lane은 실제로 `agent thread limit reached`로 생성 실패했다.
실패 기록은 보존하며 독립 6개 검토 완료로 계산하지 않는다.

## 지적과 수렴

| 단계·수준 | 지적 | 처리와 재검증 |
|---|---|---|
| TDD P1 | RESP3 false를 malformed로 판정 | 고정 원본·실서버로 반증. -1/false 모두 삽입 거부, 공간·확장 자원 부족을 구별하지 않도록 수정 |
| TDD P2 | 동일 item COUNT를 정확한 삽입 횟수로 가정 | 작은 fixture에서 2→4 관측. 포화 전후 불변과 과대 계수를 별도 검증 |
| 검증 P2 | testify 직접 import로 tidy 변경 | 표준 testing으로 교체, go.mod/go.sum 무변경과 lint 확인 |
| 안정성 P1 증거 공백 | 기본 CI만으로 module/race 성공을 주장할 위험 | opt-in 일반·race를 별도로 실행. 최종 수정본 재실행 결과는 아래 기록 |
| 안정성 P2 | fake 동시 기록·비협조적 client 수명 증거 부족 | 기존 stress helper16 workers/64 tasks, synctest 다섯 메서드 추가. targeted race PASS |
| 안정성 P2 | 유실 주입이 무관한 응답도 가로챔 | PONG 오주입 RED 재현. 완전히 전송된 단일 CF.INSERT로 한정, 회귀 race PASS |
| 안정성 P2 검토 | fake argv가 얕은 복사 | 값이 string/int64/uint8/uint16뿐임을 대조. 외부 mutable 객체가 없으므로 깊은 복사 요구는 철회 |
| 안정성 P3 | wrapped 미지원 오류의 보수적 분류가 불명확 | Go doc·README EN/KO에 명시, wrapped case를 다섯 메서드 오류 행렬에 추가. targeted race PASS |

제품 코드의 `context.Background`, `context.TODO`, `go func`, `time.Tick`, `panic` 검색은
0건이다. 테스트의 goroutine은 고정 개수·완료 대기 또는 synctest bubble 안에 있다.
예제의 panic은 실행 실패를 숨기지 않기 위한 fake-only example 경로다.
프로덕션 decoder의 타입 단언은 같은 함수의 strict decoder 성공 뒤에만 실행된다.

## 검증 증거와 공백

- 최초 constructor RED(미정의 API), 메서드 RED(고정 실패 stub), RESP3 false RED,
  PONG 유실 주입 RED를 확인한 뒤 각각 GREEN으로 수렴했다.
- 초기 opt-in 일반 2.214초, race 2.563초 PASS. RESP2/RESP3·64개 동시 삽입·응답 유실
  no-retry 1회/default retry 2회와 컨테이너 종료를 확인했다.
- 최종 fake/synctest/유실 장치/wrapped 오류/example targeted race: PASS 1.471초.
- 최종 `make fmt-check tidy-check vet lint`: PASS, lint 0 issues, go.mod/go.sum 변경 없음.
- `make ci` 첫 실행: 전체 일반 테스트 PASS. 실행 도중 추가한 synctest import와
  빌드 의존성 목록이 불일치해 race의 probabilistic/redis 빌드 FAIL. 최종 소스를
  동결한 뒤 전체 실행을 다시 수행했다. 이 최초 실패를 성공으로 계산하지 않는다.
- 최종 소스의 opt-in 일반 2.173초, race 2.564초 PASS. 컨테이너 종료도 확인했다.
- 이후 소스를 변경하지 않고 다시 실행한 `make ci`: 종료 코드 0, 전체 일반·race 및
  benchmark 도구의 실패 주입 회귀 PASS. probabilistic/redis race는 3.531초다.
- `git diff --check`: PASS. 한국어 문서 용어 감사: 6개 파일 0건.

확장 자원 부족의 실제 주입, 임의 TCP 분할·pipeline·Cluster redirect, 다른 Redis 버전은
검증 범위 밖이다. no-replay와 deadline은 생성자가 강제하지 못하는 호출자 계약이다.
wiki 조사 파일은 별도 미머지 작업 트리에 보존했고 GNO 기본 collection 검색은 0건이다.
CHANGELOG·태그·release는 배포 작업이 아니므로 변경하지 않는다.

최종 구현 검토: PASS — 알려진 미해결 P0–P3=0. 실제 서버 검증과 전체 로컬 CI를
확인했다. PR 생성 후 검토·hosted CI는 별도 PENDING이며 머지 승인을 대신하지 않는다.
