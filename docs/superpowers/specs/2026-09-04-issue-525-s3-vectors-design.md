# #525 S3 Vectors 지원 설계

## SPW-01 — 대상과 근거

- 대상: `github.com/bluetape4k/bluetape-go`의 선택적 `s3vectors` package.
- 독자: AWS SDK for Go v2를 호출하는 bluetape-go 사용자와 유지보수자.
- 목적: S3 Vectors의 bucket/index discovery와 vector put/get/list/query를
  호출자 소유 client 경계 안에서 제공한다.
- 근거: GitHub issue [#525](https://github.com/bluetape4k/bluetape-go/issues/525),
  조사 issue [#516](https://github.com/bluetape4k/bluetape-go/issues/516),
  AWS SDK for Go v2 `service/s3vectors@v1.13.0` API source.
- 보류된 주장: 현재 Floci에는 S3 Vectors를 지원한다는 검증된 근거가 없으므로
  local emulator 호환성을 문서나 테스트에서 주장하지 않는다.

## 문제와 선택지

S3 Vectors는 일반 S3 object API와 별도 service이며 SDK의 `document.Interface`
와 `VectorDataMemberFloat32` union을 사용한다. 다음 선택지를 검토했다.

1. vector database 공통 abstraction을 만든다. 호출자 소유권과 SDK 고유
   metadata/filter semantics를 잃고 범위가 커지므로 거부한다.
2. package 없이 compile-only example만 둔다. request construction과
   cancellation을 반복해서 검증할 수 없으므로 거부한다.
3. **선택:** SDK request/response를 그대로 노출하는 얇은 `Client`/`Provider`
   경계와 fake-first 테스트를 추가한다. SDK 타입이 바뀌면 재검토가 필요한
   대신 embedding, dimension, distance metric, metadata schema와 lifecycle을
   호출자에게 명시적으로 남긴다.

## 공개 계약

`New(Options)`는 caller-owned SDK client를 검증해 immutable `Provider`를
반환한다. `Client`에는 다음 SDK operation만 포함한다.

- `ListVectorBuckets`, `GetVectorBucket`
- `ListIndexes`, `GetIndex`
- `PutVectors`, `GetVectors`, `ListVectors`, `QueryVectors`

`Provider`는 각 메서드에서 같은 이름의 SDK input/output을 사용하며, SDK
paginator를 숨기거나 결과를 vector database 모델로 변환하지 않는다. `Provider`는
bucket/index 식별자와 finite float32 값 외에도 AWS request bound를 dispatch 전에
검증한다: PutVectors 500개/4,096 dimension, GetVectors 100개 key, vector key
valid UTF-8 1,024자, ListVectors 1,000개와 16 segment, QueryVectors TopK 10,000, vector
metadata 40 KiB, request payload 20 MiB. index dimension, distance metric,
metadata schema와 filter 값의 의미는 호출자가 소유한다.

모든 외부 호출은 전달받은 `context.Context`를 그대로 사용한다. 호출 전과
응답 직후 context를 확인해 caller cancellation이 SDK 오류보다 우선하도록
한다. client 생성, credentials, retry, timeout, logging, metrics와 close는
호출자가 소유한다. package는 global logger나 live client를 만들지 않는다.

## 오류와 zero value

- `ErrNilClient`: nil 또는 typed-nil SDK client.
- `ErrInvalidProvider`: zero-value 또는 손상된 provider.
- `ErrInvalidRequest`: 필수 식별자/벡터/metadata-filter 입력 오류.
- `ErrOperationFailed`: SDK 호출 실패. 원인은 `errors.Is`/`errors.As`로만
  관찰하며 provider 오류 메시지와 request payload는 `Error()`에 넣지 않는다.
- `ErrMalformedOutput`: required output이 nil이거나 계약에 맞지 않는 응답.

`Provider` zero value는 외부 호출 없이 `ErrInvalidProvider`를 반환한다.
입력/응답 slice와 metadata 값은 호출자/SDK 메모리를 보존하도록 필요한
경계에서 복사한다. SDK의 `document.Interface` 자체는 opaque 값으로 유지해
직렬화 의미를 변경하지 않는다. 호출자는 SDK dispatch가 끝날 때까지 해당
opaque document를 변경하지 않아야 하며, provider-specific 구현의 deep-copy는
지원하지 않는다.

## 실패, cancellation, 보안 계약

- validation 실패는 SDK 호출 0회다.
- SDK가 output과 error를 함께 반환해도 context가 취소되지 않았다면
  `ErrOperationFailed`를 반환한다.
- SDK 응답 뒤 context가 취소되면 output을 공개하지 않고 context 오류를
  반환한다.
- 오류 문자열에는 bucket/index 이름, metadata/filter, vector 값, provider
  오류의 raw message를 포함하지 않는다.
- vector 값은 finite `float32`만 허용하고 cosine index의 zero-vector 정책과
  실제 index dimension 일치는 AWS index 계약/호출자 책임으로 남긴다. 이
  package는 서비스의 4,096 dimension 상한만 preflight하고 임의 dimension이나
  distance metric을 추론하지 않는다.

## 수용 기준과 DoD

- 기본 CI는 fake client unit test와 compile-checked example만 실행하고 live
  AWS credentials 또는 검증되지 않은 emulator에 의존하지 않는다.
- request construction, metadata/filter 전달, cancellation, nil/malformed
  output, SDK error wrapping, redaction과 input immutability를 테스트한다.
- live test가 추가되면 명시적 build tag와 environment opt-in 뒤에 두고,
  실행하지 않은 환경을 성공으로 보고하지 않는다.
- `README.md`와 `README.ko.md`가 source-equivalent 범위와 local/live 제한을
  설명한다.

## SPW-02 — 계약 충족 기록

API, zero value, 오류, context owner, metadata/filter ownership, fake-first
검증과 non-goal을 위 설계에 기록했다. goroutine, worker, timer와 cleanup을
package가 생성하지 않으므로 해당 close owner는 N/A이다.

## SPW-03 — 한국어 기술 문체

저장소 언어 정책에 따라 설계 문서는 한국어로 작성하고, Go 식별자, AWS API
이름, 명령, URL과 버전은 그대로 보존했다. 모호한 `지원한다` 대신 호출 경계,
소유권과 검증 결과를 구체적인 동사로 기술했다.

## SPW-04 — 기술 의미와 추적성

Issue #525의 operation 범위와 non-goal을 `Client` 목록, caller-owned 항목,
기본 CI/에뮬레이터 제한에 매핑했다. SDK source의 eight operation과
`VectorDataMemberFloat32`, `document.Interface`, paginator 존재를 확인했으며
dimension/permission/runtime behavior를 package가 추측하지 않도록 명시했다.

## SPW-05 — read-back

이 문서를 저장 후 다시 읽어 heading, code token, 링크, 표준 Markdown 구조와
수용 기준을 확인했다. 구현 중 확정된 API와 범위는 artifact와 일치하며,
검증되지 않은 live/emulator 경로는 별도 후속 작업으로 남겼다.
