# #546 설계 문서 7-Tier 검토

- 검토 대상: `docs/superpowers/specs/2026-09-08-issue-546-barcode-qr-design.md`
- 기준 head: `c4b5ef2a` (설계 문서만 변경)
- 기준 base: `develop` / `51be48337427323db96d3d3ee944e6331e313b28`
- 검토 범위: QR·Code128 provider 경계, 표준 이미지·PNG 출력, 입력/크기 제한, 취소·오류·동시성, 문서·dependency·롤백 계약
- 외부 근거: provider v1.1.0 pinned commit `11e32e438ffcc2af3d65aa3547c065972a743d70`; [Code128 source](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/code128/encode.go), [QR source](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/qr/encoder.go), [scaled barcode source](https://github.com/boombuler/barcode/blob/11e32e438ffcc2af3d65aa3547c065972a743d70/scaledbarcode.go)
- 로컬 근거: `imagekit/types.go`, `imagekit/errors.go:29-66`, `imagekit/transform.go:215-230`, `imagekit/README.md`, 기준 테스트 `go test -count=1 ./imagekit` PASS 0.370s

## 관점별 결과

| 우선순위 | 관점 | 결과 | 근거 | 필요한 조치 |
|---|---|---|---|---|
| — | 성능 | PASS | 입력 QR 1,024 byte·Code128 80 byte, 캔버스 4,096²/4,194,304 pixels, provider·할당 전 검증, capped PNG 4 MiB, 호출별 이미지·버퍼가 명시됐다. spec 46–68, 73–82. | 구현에서 checked arithmetic, 한도 초과 전 buffer write 방지, 정확히 맞는 크기/1 pixel 부족 경계를 테스트한다. P0/P1 없음. |
| P1 | 안정성 | FAIL | spec 84–90은 provider 오류를 `Cause`에 보관할 수 있다고 하면서 `errors.As` 미지원이라고 한다. `imagekit.Error.Unwrap()`은 Cause를 반환하므로 원인이 복원된다. | provider 오류 Cause를 보관하지 않고 sentinel만 반환한다고 계약을 단일화한다. context 오류만 원형 보존한다. 안정성 재검토가 필요하다. |
| P1 | 보안 (메인 inline fallback) | FAIL | 안정성 P1과 동일한 경계에서 provider가 포함할 수 있는 원문이 `Unwrap`/errors.As로 노출될 수 있다. 독립 보안 lane은 기존 native slot 한도로 실행하지 못해 메인 세션이 읽기 검토했다. | 안정성 P1과 하나의 수정으로 중복 제거한다. provider 오류 문자열·Cause·로그에 콘텐츠를 넣지 않는 테스트를 추가한다. |
| P2 | 운영 (메인 inline fallback) | PASS + watchpoint | 외부 서비스/파일/상태 변경은 없고 롤백 단위가 새 하위 패키지·dependency·문서로 닫혀 있다. 다만 dependency checksum/license 확인 명령이 AC에 직접 적히지 않았다. | 계획에 `go mod verify`, pinned version/license와 repository vulnerability/dependency 검사를 넣는다. 구현 차단 P0/P1은 아니다. |
| — | Developer/API (메인 inline fallback) | PASS | `imagekit/barcode` 격리, `Render`/ `EncodePNG` 두 함수, provider 타입 비노출, 기존 imagekit API 유지가 명확하다. `imagekit.Error` 오류 정책은 안정성 P1로 추적한다. | Go doc·README 두 언어·실행 예제를 계획에 매핑한다. P0/P1 없음. |
| — | User/caller (메인 inline fallback) | PASS | Kind·콘텐츠·크기·QR level allowlist, QR UTF-8/Code128 printable ASCII 구분, zero size 거부, resize/crop 판독성 caveat가 명시됐다. | 잘못된 입력과 unsupported 동작 예제를 README에 넣고 오류 분류를 그대로 노출한다. P0/P1 없음. |

## 메인 세션 통합 판정

검토자 간 중복을 제거하면 P0 0건, P1 1건이다. 핵심 결함은 provider 오류의 Cause 보존과 `errors.As` 비지원 선언이 동시에 존재하는 계약 충돌이다. 새 subpackage가 기존 `imagekit.Error`를 재사용하더라도 `Unwrap` 동작은 바뀌지 않으므로 현재 문장 그대로 구현하면 원문 오류가 호출자에게 노출될 수 있다.

### 제안 수정

1. provider 오류는 Cause에 저장하지 않고 `ErrEncode` sentinel과 고정 Operation/Format만 반환한다.
2. context 취소·deadline만 원형 오류로 반환해 `errors.Is`를 보존한다.
3. provider 오류를 문자열·Cause·로그에 넣지 않는 테스트를 AC-05에 추가한다.
4. dependency 무결성·license 확인은 구현 계획과 AC-07에 명시한다.

이 수정은 오류 관측 계약을 확정하는 의미 있는 변경이므로, 수정된 spec을 사용자에게 다시 승인받은 뒤 안정성 관점을 재실행한다. 그 전에는 plan·implementation·PR 단계로 진행하지 않는다.

## Writer gate

- SPW-01 PASS: 대상/독자/언어, exact head/base, 로컬·외부 근거와 미검증 구현 상태를 고정했다.
- SPW-02 PASS: 관점별 결과, severity, evidence, required edit, 통합 verdict와 재검토 조건을 포함했다.
- SPW-03 PASS: Korean technical register와 고정 API/URL/commit token을 보존했다.
- SPW-04 PASS: spec→source traceability와 `Error.Unwrap` 모순을 재현해 P1로 정규화했다.
- SPW-05 PASS: 표·링크·코드 토큰·미완료 경계를 최종 read-back했다.

## 보완안

메인 세션은 위 P1을 반영해 설계 문서를 다음과 같이 수정했다(아직 사용자 재승인 전이다).

- provider 오류는 `Cause`에 저장하지 않고 `ErrEncode`와 고정 `Operation`/`Format`만 반환한다.
- context 취소·deadline만 원형 오류로 반환한다.
- AC-05에 `Cause == nil`, `Unwrap`/`errors.As` 비복원 검증을 추가했다.
- AC-07과 호환성 절에 `go mod verify`, pinned license/checksum 및 dependency 검사를 추가했다.

수정된 spec의 사용자 재승인 후 `spec-stability-rerun`을 수행하고, 기존 실패 lane과 exact evidence digest를 연결해 resolution receipt를 기록한다.

## 상태

- Spec review: **PENDING — 보완된 spec 사용자 재승인 및 안정성 재검토 필요**
- 구현/계획/PR/CI: 미착수
- 머지·배포·정리: 권한 없음, 수행하지 않음
