# #546 — QR·Code128 이미지 생성 설계

## 상태와 목표

- 상태: 작성된 spec 검토 대기. QR·Code128과 순수 Go dependency 사용 방향은 승인됐으며, 아래 세부 계약은 이번 문서의 승인 대상이다.
- 이슈: [#546](https://github.com/bluetape4k/bluetape-go/issues/546), milestone `0.23.0`.
- 저장소: `bluetape4k/bluetape-go`, base `develop`, head `feat/issue-546-barcode-qr`.
- 기준 커밋: `51be48337427323db96d3d3ee944e6331e313b28`.
- 목표: 호출자가 QR·Code128을 표준 `image.Image` 또는 PNG 바이트로 생성하고 기존 `imagekit`과 조합할 수 있게 한다.
- 전달 범위: 이슈별 구현·검증·독립 PR 생성과 CI 확인. 머지·배포·브랜치 삭제는 포함하지 않는다.

## 현재 근거

기존 `imagekit`은 입력 디코딩과 변환, PNG/JPEG 인코딩을 담당한다.
`imagekit/types.go`에는 크기 제한이 있고, `imagekit/errors.go`에는 `Error`와 오류 분류가 있다.
`imagekit/encode.go`의 인코더는 비공개이므로 새 패키지가 내부 함수를 노출하도록 기존 API를 변경하지 않는다.
기준 커밋에서 `go test -count=1 ./imagekit`이 통과했다. 새 barcode 구현을 검증한 결과는 아니다.

채택할 `github.com/boombuler/barcode v1.1.0`은 MIT이며, 확인한 `go.mod`에 외부 dependency가 없다.
태그가 가리키는 커밋은 `11e32e438ffcc2af3d65aa3547c065972a743d70`이다.

- [Code128 소스](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/code128/encode.go): 길이를 1–80 rune으로 제한하며, 일부 오류에 원문을 포함한다.
- [QR 소스](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/qr/encoder.go): 인코딩 모드에 따라 다른 인코더를 호출하므로 외부 enum 값을 그대로 전달하지 않는다.
- [배율 조정 소스](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/scaledbarcode.go): 정수 배율과 중앙 정렬을 제공하지만 고정 여백을 보장하지 않으며, 반환 객체가 `Content()`를 통해 원문을 제공한다.
- 조사 기록: `bluetape4k-wiki`의 `docs/go023-provider-research` 브랜치, `research/2026-09-08-go023-barcode-provider-contracts.md`. 별도 작업공간의 문서여서 GNO 검색 반영은 아직 확인되지 않았다.

## 대안과 선택

1. **격리된 `imagekit/barcode` 하위 패키지 — 채택.** QR·Code128 provider를 이 패키지 안에서만 사용한다. 기존 `imagekit` API를 유지하면서 책임을 분리한다. 같은 Go module의 direct dependency이므로 별도 선택 설치 module이라고 설명하지 않는다.
2. **기존 `imagekit`에 생성 API 추가 — 미채택.** 변환만 쓰는 호출자의 패키지 의존 경로까지 barcode 생성과 결합한다.
3. **인코딩 알고리즘 직접 구현 — 미채택.** QR 오류 정정·마스킹과 Code128 인코딩의 검증 부담이 이번 utility 범위를 넘는다.

새 provider 추상화, registry, 사용자 정의 색상·로고·문자 캡션, 이미지 디코더, QR 읽기 기능은 추가하지 않는다.
OCR, CAPTCHA, cgo, HTTP 다운로드, 파일 쓰기도 제외한다.

## 공개 API와 입력 계약

새 패키지의 공개 함수는 `Render(ctx context.Context, req Request) (image.Image, error)`와
`EncodePNG(ctx context.Context, req Request) ([]byte, error)` 두 개로 제한한다.
`Request`에는 `Kind`, `Content`, `Width`, `Height`, `QRLevel`을 둔다.
`Kind`는 자체 타입의 `QR`, `Code128`만 허용한다. provider 타입은 공개 API에 노출하지 않는다.

| 항목 | 계약 |
| --- | --- |
| `Kind` | 0 또는 미등록 값은 거부한다. |
| QR 콘텐츠 | 유효한 UTF-8, 1–1,024 byte. 공백을 자르거나 문자열을 정규화하지 않는다. |
| Code128 콘텐츠 | printable ASCII `0x20`–`0x7e`, 1–80 byte. 제어 문자·FNC 확장은 지원하지 않는다. |
| `QRLevel` | 자체 타입의 기본값 0은 M. L/M/Q/H만 허용한다. Code128에서는 0만 허용한다. |
| 크기 | 두 값 모두 1–4,096, 전체 픽셀 수는 4,194,304 이하. 0은 기본 크기가 아니라 잘못된 입력이다. |
| QR 비율 | `Width == Height`. 비정방형 요청은 거부한다. |
| 입력 크기 검사 | provider 호출과 이미지 할당 전에 수행한다. 곱셈 전에 개별 상한을 확인한다. |

QR은 명시적인 UTF-8 byte 인코딩 경로를 사용한다. 자동 숫자·영숫자 모드 선택은 이번 범위에 넣지 않는다.
입력 검증을 통과한 뒤 provider가 인코딩을 거부하면 인코딩 오류로 처리한다.
콘텐츠가 허용된다는 사실만으로 모든 크기에 렌더링할 수 있다고 보장하지 않는다.

## 이미지와 PNG 계약

검정 모듈과 흰 배경만 사용하고, 경계는 `image.Rect(0, 0, Width, Height)`로 고정한다.
호출마다 새 `*image.Gray`를 만들며 provider 객체를 반환하지 않는다.
이 방식은 provider의 `Content()` 접근 경로를 제거하지만, 이미지 자체에 인코딩된 정보를 숨기는 보안 기능은 아니다.

- QR: 심볼의 각 방향에 최소 4모듈의 흰 여백을 포함한다. 전체 심볼과 여백을 담을 수 있는 최대 정수 배율을 적용하고 중앙에 배치한다. 남는 픽셀도 흰색이다.
- Code128: 좌우에 각각 최소 10모듈의 흰 여백을 둔다. 가로 모듈만 정수 배율로 확대하고 중앙에 배치한다. 막대는 요청한 높이를 채운다.
- 위 여백은 이 helper의 고정 계약이다. 출력 장치나 특정 산업 규격에 대한 인증을 의미하지 않는다.
- 최소 1배 심볼과 여백이 들어가지 않으면 `ErrInvalidOptions`를 반환한다. 잘라내기·보간·축소로 억지로 맞추지 않는다.
- PNG는 표준 `image/png`로 메모리에 인코딩한다. 길이 제한 writer로 출력 바이트를 최대 4 MiB로 제한한다. 초과 시 부분 바이트를 반환하지 않는다.
- 이미지와 PNG 모두 오류 시 결과는 `nil`이다. 호출 간 공유 버퍼나 캐시는 없다.

기존 `imagekit.Transform`에는 PNG를 `bytes.NewReader`로 전달할 수 있다.
다만 임의 resize·crop·JPEG 변환은 모듈과 여백을 손상할 수 있으므로, 조합 가능성과 판독 가능성을 구분해 README에 설명한다.

## 취소·동시성·오류

`nil` context는 입력 오류로 거부한다. 시작 전에 취소됐으면 provider를 호출하지 않는다.
provider 호출 후, 이미지 복사 행마다, PNG writer의 각 쓰기와 최종 반환 직전에 context를 확인한다.
늦게 확인한 취소는 생성한 결과를 폐기하고 원래 context 오류를 반환한다.
provider 호출은 협력적 취소 API가 없으므로 호출 도중 즉시 중단하거나 deadline 안에 복귀한다고 보장하지 않는다.
이를 감추기 위한 분리 goroutine은 만들지 않는다. provider 내부의 QR goroutine 사용과 helper의 goroutine 생성 여부도 구분한다.

공유 가변 상태 없이 호출별 입력·이미지·버퍼를 소유한다. 동시 호출은 지원하지만 반환 이미지를 동시에 수정할 때의 동기화는 호출자 책임이다.
요청별 상한은 전체 프로세스 메모리 상한이 아니다. 동시 실행 제한은 호출자가 담당한다.

기존 `imagekit.Error`를 재사용하고 `Operation`과 `Format`에는 고정된 안전한 값만 넣는다.
분류는 `ErrInvalidOptions`, `ErrInputTooLarge`, `ErrImageTooLarge`, `ErrEncode`를 사용한다.
콘텐츠 길이 초과는 `ErrInputTooLarge`, 양수 크기·픽셀 상한 초과는 `ErrImageTooLarge`, 나머지 입력 위반은 `ErrInvalidOptions`다.
PNG 출력 한도 초과도 `ErrEncode`로 분류한다. `errors.Is`로 분류할 수 있어야 한다.
원문이 들어갈 수 있는 provider 오류는 `Cause`로 보관하거나 문자열에 붙이지 않는다.
따라서 provider 오류의 `errors.As` 복원은 지원하지 않는다. context 오류는 원형으로 반환해 `errors.Is`를 유지한다.
로깅, 오류 시 원문 출력, 포괄적인 panic 복구는 하지 않는다.

## 실패 모드와 검증

| 실패 모드 | 기대 결과와 검증 |
| --- | --- |
| 빈 콘텐츠·잘못된 UTF-8·미등록 enum·Code128 제어 문자 | provider 호출 전 거부. 경계값과 입력 보존을 table test로 확인한다. |
| 음수·0·상한 초과·큰 정수 크기 | 할당 전 거부. 픽셀 상한과 오버플로 회피를 검증한다. |
| 심볼과 여백보다 작은 캔버스 | 크기 오류. 첫 실패 크기와 최소 성공 크기를 비교한다. |
| provider 인코딩 오류 | 원문 없는 `ErrEncode`. 내부 오류 변환 경계에 원문을 포함한 오류를 주입해 문자열과 unwrap 경로를 확인한다. 공개 provider 교체 API는 만들지 않는다. |
| 사전·처리 중·최종 확인 시 취소 | context 오류와 nil 결과. 비협력적 provider의 즉시 중단을 주장하지 않는다. |
| PNG 출력 한도 초과 | `ErrEncode`와 nil 결과. 제한 writer를 작은 한도로 직접 검증한다. |
| 여러 동시 호출 | 독립적인 결과와 race 검사 통과. 반환 객체끼리 픽셀 버퍼를 공유하지 않는다. |

## 수용 기준과 예정 검증

- **AC-01:** 기존 `imagekit` API 변경 없이 QR·Code128 두 종류와 두 출력 함수를 제공한다.
- **AC-02:** 위 입력·크기·출력 한도를 경계값 테스트로 입증한다.
- **AC-03:** QR 정방형·정수 배율·흰 여백, Code128 막대·좌우 여백을 구조 기반 테스트로 확인한다.
- **AC-04:** 반환 이미지에 provider의 `Content()` 접근자가 없고, PNG 디코딩 결과의 크기·흑백 픽셀이 Render 결과와 일치한다.
- **AC-05:** 오류 분류·원문 비노출·취소·공유 상태 부재를 검증한다.
- **AC-06:** `imagekit`과 새 하위 패키지의 README/README.ko, Go doc, 실행 가능한 예제를 동기화한다. ASCII와 한글 QR 예제를 구분한다.
- **AC-07:** targeted test와 race, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make ci`, PR exact-head CI를 통과한다.

픽셀 전체 덤프나 PNG 압축 바이트의 고정 golden 대신 크기·모듈·여백·호출 간 결정성을 검사한다.
provider 전달 콘텐츠는 내부 경계 테스트로 확인하되 공개 객체에는 노출하지 않는다.
독립 decoder 왕복 검사와 물리적 스캐너 시험은 이번 검증 범위 밖이다. 따라서 스캐너 판독 인증이나 모든 리더의 Unicode 호환성을 주장하지 않는다.

## 호환성·변경 범위·롤백

새 파일은 `imagekit/barcode/` 아래에 두고, `go.mod`/`go.sum`에 승인된 provider 버전을 추가한다.
기존 package README 두 언어에는 새 패키지 링크와 조합 시 주의점을 추가한다.
기존 이미지 변환 동작, 기본 제한, dependency 버전은 이 작업에서 변경하지 않는다.
배포 전 롤백 단위는 새 하위 패키지, 해당 dependency 및 문서 변경이다. 외부 데이터나 서비스 상태는 변경하지 않는다.

## 설계 단계 DoD

- [x] 이슈·base·분리 브랜치·기존 imagekit 근거 확인.
- [x] 대안·공개 계약·실패 모드·수용 기준·제외 범위 작성.
- [ ] 작성된 spec 사용자 승인.
- [ ] 6개 관점과 메인 통합 spec review 완료.
- [ ] 구현 계획 작성·검토·승인.
- [ ] 구현·검증·PR 생성·CI 확인.

현재 문서는 구현 완료 보고가 아니다. 다음 단계는 작성된 spec 승인 후 spec review와 구현 계획이다.
