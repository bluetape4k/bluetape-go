# #534 Cuckoo 설계 검토

## 범위와 판정

기준 소스는 develop `51be48337427323db96d3d3ee944e6331e313b28`이며 대상은
`docs/superpowers/specs/2026-09-08-issue-534-cuckoo-design.md`다.
2026-09-08 작성된 설계만 검토했다. 구현·통합 테스트·CI 성공 판정이 아니다.

두 독립 관점과 네 main 대체 관점을 통합했다. 추가 native lane 생성은
`agent thread limit reached`로 실패했다. 여섯 독립 리뷰 완료로 계산하지 않는다.
두 독립 리뷰는 native `code-reviewer` 역할이며 노출된 역할 설정은
`gpt-5.6-luna` / `max`다. 실제 추론량을 별도로 측정했다는 뜻은 아니다.

| 관점 | 실행 주체 | 결과와 한계 |
|---|---|---|
| 성능 | spec534_performance, 독립 | P0–P3 0. 단일 명령, 입력 한계, IO 소유권 검토. benchmark 미실행 |
| 보안 | spec534_security, 독립 | 재시도·자원·오류·입력 경계 지적 반영. 삭제 P1은 근거 재검토 후 철회 |
| 안정성 | main 대체 | output-plus-error/늦은 취소에도 commit-unknown과 원인 보존하도록 보완 |
| 운영 | main 대체 | 모듈 capability와 plain Redis 분리, 신뢰 endpoint·quota·deadline 책임 명시 |
| 개발자/API | main 대체 | nil context, EXPANSION 0, 엄격한 RESP2/RESP3 결과와 namespace 계약 확인 |
| 사용자/호출자 | main 대체 | 성공 삽입 장부·삭제 전제·unknown 재실행 금지, 근사값 제한 확인 |
| 통합 | main | 아래 수정 사항을 설계에 반영. 작성된 spec의 사용자 확인은 대기 |

## 지적 처리

- P1 재시도: Do 인터페이스가 설정을 강제하지 못한다는 한계를 인정하고 모든
  mutation 경로의 no-replay를 필수 호출자 계약으로 명시했다. transport 응답 유실
  회귀에서 기본 retry와 no-retry client의 차이를 검증하도록 추가했다.
- 삭제 P1 철회: 성공 삽입 수 이내의 올바른 삭제 자체가 false negative를 만든다는
  초기 주장은 근거가 부족했다. 독립 reviewer가 철회했다. 남은 위험은 never-added,
  초과 삭제, unknown을 성공으로 오인하는 사용이며 이를 설계·예제 수용 기준에 반영했다.
- P2 자원: 서버 허용 옵션 범위는 안전한 tenant quota가 아니다. 신뢰하는 운영 설정과
  호출자 메모리·CPU·namespace 수 제한을 요구했다.
- P2 비노출: 최상위 오류 문자열만 안전하며 unwrap 원인은 민감할 수 있음을 명시했다.
  errors.As 원인 보존과 직접 로깅 금지를 구분했다.
- P2 RESP 할당: raw Do의 전체 디코딩을 adapter가 제한하지 못한다는 기존 한계를
  유지했다. 신뢰된 endpoint 전제이지 적대적인 Redis 서버에 대한 방어 완료가 아니다.
- P3 입력: namespace 128 byte와 기존 validator 규칙, 별도 RESP argv, binary item
  회귀를 명시했다.
- main P1/P2: output-plus-error·late-cancel 예외 우선순위, nil context 오류,
  동시 동일 item 64회 삽입의 bucket/capacity 조건을 보완했다.

## 문서 검증과 남은 작업

독자·목적·범위, 대안과 근거, 명시적인 소유권, 검증 가능한 수용 기준, 비목표를
대조했다. 한국어 조사·띄어쓰기·용어 일관성 및 코드 식별자 원형을 확인했다.
미실행 성능·통합·CI를 성공으로 표현하지 않았다.

설계 수준의 지적은 모두 수정 또는 근거 있는 철회로 처리했다. 단, 수정 후 독립
전체 재리뷰는 수행하지 않았으며 main이 최종 diff를 통합 검토했다. 구현 단계에서
실제 테스트로 입증해야 하므로 이 기록은 코드 P0–P3 0건 증거가 아니다.

DoD: 조사·spec·관점 검토 완료. 작성된 spec 확인, 구현 계획, 코드·테스트·PR은
PENDING이다. 머지는 요청 범위 밖이다.
