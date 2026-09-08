# #534 Cuckoo 서버 계약을 실제 fixture로 검증한 교훈

## 반증과 수정

- CF.INSERT 공식 문서의 RESP3 `-1` 설명을 설계에 옮겼지만 Redis 8.8.1은 `false`를 반환했다.
  단위 테스트와 실서버 테스트의 실패를 남기고 `ErrCuckooFull` 매핑을 고쳤다.
  공간 부족과 확장 자원 부족은 같은 응답이므로 sentinel 설명도 함께 수정했다.
- 같은 item만 넣으면 COUNT가 정확하다는 가정도 틀렸다. 작은 필터의 후보 bucket이
  겹치면 2회 삽입을 4로 센다. 포화 검증은 실패 전후 count 불변으로, 과대 추정은
  고정 fixture의 별도 회귀로 검증한다. 이 값을 삭제 권한이나 정확한 성공 횟수로 쓰지 않는다.
- 응답을 읽은 뒤 EOF로 바꾼 실제 연결에서 no-retry는 삽입 1회와 commit-unknown을,
  기본 retry는 삽입 2회를 보였다. fake의 호출 1회만으로 wire-level 재전송 방지를 주장하지 않는다.
- `testify`가 간접 의존성이라고 직접 import해도 되는 것은 아니다. tidy가 직접 의존성으로
  승격시킨 것을 검출해 표준 testing으로 바꿨다. go.mod/go.sum 변경 없음 조건을 유지한다.
- 안정성 리뷰에서 응답 유실 주입 범위를 재검토했다. 무관한 PONG까지 EOF로 바꾸는
  실패를 재현하고, 완전히 전송된 단일 CF.INSERT의 응답에만 주입하도록 수정했다.
  정확한 fixture 범위를 명시하고 서버의 실제 결과를 별도 client로 확인한다.
  임의 패킷 분할·pipeline·cluster 전체의 transport 검증으로 일반화하지 않는다.
- fake의 동시 기록은 기존 bounded stress helper로, 취소에 협조하지 않는 Do는
  synctest로 검증한다. adapter가 소유하지 않은 호출을 별도 goroutine으로 분리하거나
  client를 강제로 닫지 않는다는 경계를 다섯 메서드 모두에서 확인한다.
- main이 `make ci` 실행 중 새 `testing/synctest` import를 추가해 진행 중인 빌드의
  의존성 목록과 소스가 불일치했다. `could not import testing/synctest (open : no such file or directory)`를
  확인했다. 새 targeted 실행은 통과했지만 전체 실행을 성공으로 바꾸지 않는다.
  검증 중에는 Go 파일을 동결하고, 수정이 필요하면 종료 후 새 검증을 시작한다.

## 원본과 적용 경계

설계·계획 리뷰에서는 public 반환 인터페이스와 실제 구현 순서를 먼저 맞추고,
fixture helper의 실제 이미지 선택 기능을 읽어야 한다는 점을 확인했다.
미머지 #611 테스트 파일을 의존성처럼 참조하지 않고 필요한 EOF 주입을 독립 구현했다.
다음 계획에서도 다섯 메서드의 취소 행렬과 retry negative control을 수용 기준으로
먼저 고정한다. 상세 지적은 `../review/2026-09-08-issue-534-plan-review.md`에 연결한다.

설계 리뷰의 초기 “정상 삽입 횟수 이내의 삭제도 false negative를 만든다”는 주장은
근거 재검토 후 철회했다. 삭제 위험은 never-added, 초과 삭제, unknown을 성공으로
취급하는 경우로 좁힌다. 확률적 자료구조라는 이유만으로 정상 연산을 위험으로 분류하지 않는다.
CF helper가 Expansion 0을 생략하는지도 호출 전에 확인해 기본 서버 설정과 명시적 확장 금지를 구분한다.

review receipt에는 CLI `--help`의 필수 인자와 bounded evidence 배열을 먼저 확인한다.
실제 lane-create에서 `--evidence` 누락을 발견했다. 실패한 native lane은 완료로
덮어쓰지 않고 main 대체 검토의 근거와 연결한다. 문서 기록은 실제 독립 검토와 구분한다.

[Redis 8.8.1 내장 모듈](https://github.com/redis/redis/blob/77b6c308396c9700672390a210143a8496fb4b10/modules/redisbloom/Makefile),
[고정 응답 처리](https://github.com/RedisBloom/RedisBloom/blob/77dea3b88d579d7ec1e529f5fa2e58947ef7f767/src/rebloom.c),
[계수 및 실패 rollback](https://github.com/RedisBloom/RedisBloom/blob/77dea3b88d579d7ec1e529f5fa2e58947ef7f767/src/cuckoo.c)을 확인했다.
조회일은 2026-09-08이다. 원문을 복제하지 않았으며 외부 조사 요약은 wiki의
`research/2026-09-08-go023-cuckoo-provider-contracts.md`에도 보존했다.

서버 버전과 RESP 형식은 고정 fixture로 검증한다. source 설명은 실제 wire 응답과
대조하며 COUNT 같은 근사 API를 exact assertion으로 일반화하지 않는다.
이번 단일 관측을 사용자 범위 Go skill의 광범위 규칙으로 승격하지 않고 이 교훈과
  회귀 테스트에 우선 보존한다. 배포 버전 전체·메모리 할당 실패 주입·cluster redirect는
이 fixture의 검증 범위 밖이다.
