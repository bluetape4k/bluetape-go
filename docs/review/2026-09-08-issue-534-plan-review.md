# #534 구현 계획 검토

## 범위

승인된 spec `1218112`와 `docs/superpowers/plans/2026-09-08-issue-534-cuckoo-plan.md`를
대조했다. 코드·실제 테스트 성공을 판정하는 리뷰가 아니다. main은 작성과 통합을 소유한다.

| 관점 | 실행 근거 | 결과 |
|---|---|---|
| 성능 | native plan534_performance, code-reviewer | 지적 0. 생성자 key 재사용과 bounded stress 확인 |
| 안정성 | native plan534_stability, verifier | P1 fixture와 P2 취소/retry assertion 보완 요구 |
| 보안 | native 생성 실패, main inline fallback | 안전한 생성자 오류, typed Redis error만 미지원 판정, 별도 argv 검토 |
| 운영 | native 생성 실패, main inline fallback | startup/operation/cleanup 시간과 image pin, 미지원/양성 분리 확인 |
| 개발자/API | native 생성 실패, main inline fallback | 생성자 반환 인터페이스의 메서드 선언 순서와 sentinel 오류 계약 보완 |
| 사용자/호출자 | native 생성 실패, main inline fallback | README EN/KO, 장부·unknown·삭제 전제, no-replay 예제 확인 |
| 통합 | main | 수용 기준을 작업1..5에 대응, 아래 변경 반영 |

독립 역할의 노출된 설정은 code-reviewer/verifier 모두 `gpt-5.6-luna` / `max`다.
네 lane의 실제 실패는 `agent thread limit reached`이며 독립 완료로 계산하지 않는다.

## 지적과 처리

- P1 fixture: 공용 Redis StartServer는 7.4 이미지를 고정한다. 공용 API 확장 없이
  기존 package의 tcredis.Run과 testcleanup을 사용하도록 변경했다. 생성 직후 cleanup을
  등록하고 startup90초와 operation30초를 분리한다.
- P2 취소: 다섯 메서드 모두 전송 전/중/응답 직후 취소와 호출 수를 검증하도록 명시했다.
  mutation만 commit-unknown이며 읽기는 false/0 결과를 반환한다.
- P2 retry: namespace를 분리하고 no-retry의 실제 삽입1회+EOF+unknown과 기본 retry의
  성공+중복 삽입을 서로 다른 assertion으로 정했다. 후자는 잘못된 설정의 negative control이다.
- main 순서 보완: 생성자 단계에서 인터페이스 메서드를 선언하되 고정 실패로 시작하며,
  작업2의 RED가 실제 동작을 요구하도록 했다. 생성자 validation은 입력 포함 오류를 노출하지 않는다.
- 비차단 성능 권고: fake callback을 mutex 밖에서 호출하도록 변경했다. fake 설정은
  호출 시작 전에 고정하고 실행 중 변경하지 않는다.

## 문서·검증

SPW-01: 한국어 개발자용 계획 검토, 승인 spec·현재 helper 원본을 근거로 고정했다.
SPW-02: 관점·severity·처리·한계·수용 기준을 구분했다.
SPW-03: KO-01..06 사실·용어·문장·표·링크 검토 완료, KO-07 두 문서 용어 검사 지적0건.
SPW-04: spec의 API·오류·취소·fixture·문서·CI를 계획 작업1..5에 대응했다.
SPW-05: 계획·리뷰 최종 readback 완료. 독립 안정성 재검토에서 기존3건 해소를 확인했다.

재검토의 추가 P2는 #611 미머지 파일 참조였다. EOF 방식을 #534 테스트 내부에서
독립 구현하고 embedding한 net.Conn 및 client Close의 정리 소유권을 명시해 처리했다.
main이 수정 후 독립 PR 실행 가능성을 재검토했다.

최종 계획 판정: PASS, 미해결 P0/P1/P2/P3=0. SPW 5/5, KO 7/7.
구현·테스트 결과는 아직 PENDING이다.
