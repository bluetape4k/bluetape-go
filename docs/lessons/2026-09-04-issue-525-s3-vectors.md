# Issue #525 S3 Vectors 경계 교훈

일자: 2026-09-04
범위: `s3vectors`

## 교훈

S3 Vectors 연동은 벡터 데이터베이스 추상화가 아니라 AWS SDK 경계를 안전하게
감싸는 caller-owned adapter로 제한해야 한다. 임베딩 생성, 차원·거리 metric,
metadata schema, filter 의미 해석, credentials, retry, pagination, logging은
호출자가 소유한다.

## 적용된 계약

- 요청은 SDK 호출 전에 검증하고, caller가 재사용하는 요청 slice와 vector data를
  복제해 SDK opt-in 변형으로부터 보호한다.
- AWS가 문서화한 batch/count/dimension/key/page/TopK와 20 MiB request bound를
  clone·dispatch 전에 적용해 큰 invalid 입력이 serialization/network 비용을
  먼저 발생시키지 않도록 한다. float32 JSON 표기 차이를 흡수하는 보수적
  component byte budget을 사용하고, index에 설정된 실제 dimension 일치는
  caller가 소유한다.
- vector key는 AWS 계약에 맞춰 공백만으로 이루어지지 않은 valid UTF-8을 최대
  1,024자까지 허용한다. byte 수로 제한하면 다중 byte key를 잘못 거부하므로
  rune 수와 UTF-8 유효성을 검사하고, 기존 non-blank caller 계약은 유지한다.
- `context.Context` 취소는 SDK 호출 전후에 확인하며, 반환된 응답이나 SDK 오류보다
  caller 취소를 우선한다.
- SDK 오류는 `errors.Is`/`errors.As`가 유지되도록 감싸되, resource ARN·bucket/index
  이름·provider 오류 문자열은 adapter 메시지에 복사하지 않는다.
- SDK의 opaque `document.Interface` filter와 metadata는 해석·정규화하지 않고
  그대로 전달한다. 이 값은 provider-specific 구현을 deep-copy하지 않으므로
  caller는 SDK dispatch가 끝날 때까지 변경하지 않을 책임이 있다.
- fake-first 테스트가 기본 경로이며, 검증된 live AWS 또는 local emulator가 없으므로
  기본 테스트에 외부 서비스를 연결하지 않는다. live 검증은 별도의 명시적 opt-in
  환경과 검증된 대상이 준비된 뒤 추가해야 한다.

## 예상 밖의 사항과 예방책

- SDK가 생성한 일부 response field는 포인터와 slice가 함께 존재하므로, malformed
  top-level response를 조기에 식별하려면 fake fixture도 SDK가 요구하는 최소 구조를
  명시적으로 만들어야 했다.
- `document.Interface`는 provider-specific union 경계이므로, adapter가 임의의
  metadata/filter 변환을 시도하면 caller의 표현력과 forward compatibility를
  잃는다. 향후 변경에서도 opaque 값 보존을 회귀 테스트로 유지한다.
- S3 Vectors용 로컬 emulator를 확인하지 못한 상태에서 emulator 지원을 문서화하지
  않는다. 검증되지 않은 실행 경로는 기능 지원 주장이 아니라 `PENDING`으로 남긴다.

## 후속 작업

실제 AWS 계정과 검증된 권한/fixture가 확보되면 live 테스트를 별도 build tag와
명시적 환경 변수로 추가할 수 있다. 그때도 기본 CI는 fake-first로 유지하고, live
검증 결과를 emulator 호환성 주장으로 확대하지 않는다.

## SPW 체크

- SPW-01 문제·수용 기준: 완료 — issue #525와 caller-owned 범위를 대조했다.
- SPW-02 설계 결정: 완료 — SDK boundary와 비목표를 spec에 기록했다.
- SPW-03 구현: 완료 — 요청 검증, 복제, 취소, 오류·응답 guard를 구현했다.
- SPW-04 검증: 완료 — package test, example, race, vet, module-wide test를 실행했다.
- SPW-05 회고: 완료 — opaque metadata/filter, malformed response, live path 보류를
  기록했다.
