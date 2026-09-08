# Issue #546 Step 3-R 구현 계획 검토

- 이슈: [#546](https://github.com/bluetape4k/bluetape-go/issues/546)
- 계획: `docs/superpowers/plans/2026-09-09-issue-546-barcode-qr-plan.md`
- 계획 기준 head: `d52ac5a` (provider 경계 보강 `efb4502b` 포함)
- 기준 baseline: `origin/develop` / `51be48337427323db96d3d3ee944e6331e313b28`
- 설계·spec review: amended spec `1ebee2a`, Step 2-R review `3a5f043`
- 검토 범위: 계획 문서만 read-only 검토. 구현·dependency·managed skill은 아직 변경하지 않음.
- 최종 판정: **PASS — P0=0, P1=0, P2=0, P3=0**

## 독립 6개 관점 검토

| 관점 | 결과 | 근거와 disposition |
| --- | --- | --- |
| Performance | PASS | checked span/geometry 산술, 이미지 상한, capped PNG, 호출별 buffer 분리와 32-way race/stress 순서가 Task 3·4·6에 있다. provider 반환 직후 context 검사는 bounded local check이며 추가 allocation을 만들지 않는다. benchmark 우월성 주장은 하지 않으므로 N/A다. |
| Stability | PASS | provider 반환 직후 geometry·allocation·오류 변환 전 checkpoint, row-copy/PNG write/final checkpoint, 원형 cancellation precedence, nil context zero-call, malformed output의 `ErrEncode`, 순차 race 검증이 Task 3·4·6에 고정됐다. 실제 런타임 증거는 구현 후 수집한다. |
| Security | PASS | provider 호출 전 UTF-8/allowlist·길이·축·pixel bound를 검사하고, provider 오류는 `Cause: nil` 고정 오류로 변환한다. 출력은 caller-owned `*image.Gray`와 4 MiB capped writer로 제한하며 파일·네트워크·cgo·OCR·CAPTCHA 경계가 없다. `govulncheck` 부재와 독립 scanner 미실행은 gap으로 남긴다. |
| Operator/Ops | PASS | 순수 helper라 service lifecycle, logger, metrics, container/port, settings/BOM 등록은 N/A로 명시했다. 재실행·rollback, README/CHANGELOG, 기존 `go test ./...`·CI/Nightly 발견 경로를 Task 5·6·7에 적었다. merge/tag/release는 범위 밖이다. |
| Developer/API | PASS | 공개 표면을 `Request`, `Kind`, `QRLevel`, `Render`, `EncodePNG`로 제한하고 provider 타입을 노출하지 않는다. `imagekit` sentinel alias, package-local seam, external `package barcode_test`, Go doc/gofmt/lint와 기존 API 비변경 경계를 명시했다. |
| User/Caller | PASS | EN/KO package README와 parent/root README 동기화, 한글 QR·printable ASCII Code128 examples, 입력/크기/quiet-zone/context/error 계약, scanner certification 아님과 Transform resize/crop/JPEG caveat를 제공한다. 기존 caller migration은 없다. |

모든 lane은 계획의 최신 read-back만 검토했고 구현 파일을 수정하거나 heavy test를
실행하지 않았다. Performance와 Stability lane의 초기 지적은 계획에 반영한 뒤
최신 head에서 재검토했다.

## 초기 지적과 반영 결과

| 지적 | 최초 priority | 반영 내용 | 최신 상태 |
| --- | --- | --- | --- |
| Code128 세로 geometry가 provider 높이에 종속될 위험 | P1 | pinned provider `symbolHeight != 1`을 `ErrEncode`로 분류하고 dark bar를 모든 출력 y행에 복제 | RESOLVED |
| provider seam·context 우선순위가 불충분함 | P1/P2 | `renderWithEncoder` seam과 provider 반환 직후 checkpoint를 geometry/할당/오류 변환보다 앞에 배치 | RESOLVED |
| nil context와 provider 호출 0회 계약이 모호함 | P2 | `Render(nil, req)`·`EncodePNG(nil, req)` 별도 테스트와 zero-call 검증을 명시 | RESOLVED |
| malformed/nil/empty provider 출력 분류가 불명확함 | P2 | 모두 `ErrEncode`, checked bounds span 후 geometry 진입으로 고정 | RESOLVED |
| dependency tag와 pinned commit readback이 약함 | P2 | tag ref의 object SHA/type와 source/license를 `gh api`로 대조 | RESOLVED |
| Task 1 공통 context 표와 nil context 예외가 혼재 | P2 | nil context 행을 별도 호출 예외로 명시 | RESOLVED |

최신 계획에는 미해결 P0/P1/P2/P3가 없다. 다만 실제 구현 전이므로 계획에
기록된 테스트·CI·dependency source readback은 아직 실행 증거가 아니다.

## Main integration review

| 통합 확인 | 판정 | 확인 내용 |
| --- | --- | --- |
| spec traceability | PASS | amended spec의 AC-01~AC-07이 Task 1~7의 API·검증·geometry·error·cancel·docs·commands에 일대일로 매핑된다. |
| 실행 가능성 | PASS | `Task 1 RED → Task 2 validation → Task 3 renderer → Task 4 PNG/cancel → Task 5 docs → Task 6 verification → Task 7 review → Task 8 pattern promotion` 순서다. |
| dependency ordering | PASS | provider import 직후 `go mod tidy && go mod verify`를 실행하고, 그 전 RED 단계에서는 tidy를 지연한다. |
| success/failure/edge | PASS | 정상 QR/Code128, invalid input, exact/short canvas, malformed provider output, provider/PNG error, cap boundary, pixel detachment를 모두 계획했다. |
| cancellation lifecycle | PASS | pre-call, provider 이후, row-copy, PNG write, final return checkpoint와 원형 context error precedence를 명시했다. non-cooperative provider는 호출 중 선점하지 않고 반환 후 checkpoint로 경계를 둔다. |
| concurrency/ownership | PASS | global seam/buffer와 detached goroutine을 금지하고, 호출별 새 `*image.Gray`·byte slice 및 race/stress를 검증한다. |
| arithmetic/resource bound | PASS | checked subtraction/addition/multiplication, 4,096 축, 4,194,304 pixel, 4 MiB PNG cap을 provider 호출·할당 경계에 둔다. |
| API/docs parity | PASS | package, parent imagekit, root README의 EN/KO 위치와 범위를 맞추고 exported Go doc과 실행 example을 계획했다. |
| repository topology | PASS/N/A | 새 module/catalog/workflow/settings/BOM/test-resource/coverage wiring은 subpackage라 N/A이며 기존 `go test ./...`, coverage, CI/Nightly discovery를 확인한다. |
| framework scope | PASS/N/A | Spring, Exposed, coroutine, server/container lifecycle은 해당 없는 순수 Go image helper라 N/A다. `context.Context`는 cancellation 계약에만 사용한다. |
| rollback/recovery | PASS | dependency/provider 계약 실패, test 실패, docs drift, hosted CI 실패별 재실행·rollback 경로가 Task별로 있다. |
| review/PR gate | PASS | Task 7에 spec-plan traceability, 6 lanes, writer gate, Korean PR body, `## DoD Status`, exact-head/hosted CI 재확인을 분리했다. merge는 권한 범위 밖이다. |
| pattern improvement | PASS | Task 8이 실제 implementation lesson에서 bounded provider input/output ownership, checked geometry, capped output, cancellation checkpoint를 일반 Go 규칙으로 추출하고 managed source-first 절차를 따른다. |

## Writer gate (SPW-01~05)

- **SPW-01 source/audience:** PASS — amended spec, issue, pinned provider source와 보안·판독성 경계를 계획에 연결했다.
- **SPW-02 actionability:** PASS — 모든 Task에 파일 책임, 명령, 예상 결과, 실패 시 재실행·rollback, 커밋 의도가 있다.
- **SPW-03 Korean technical naturalness:** PASS — 독자용 prose는 한국어이고 Go/API 식별자·명령·URL·sentinel은 원문을 보존한다.
- **SPW-04 completeness/consistency:** PASS — AC traceability, error redaction, context precedence, integer geometry, PNG nil result, README parity, Task 8 scope가 누락되지 않았다.
- **SPW-05 readback:** PASS — plan 전체를 줄 단위로 다시 읽었고 남은 `[ ]`는 승인·후속 실행 상태뿐이다. placeholder 또는 모호한 구현 지시는 없다.

검토 artifact 자체는 다음 writer 검증을 통과해야 한다.

```text
git diff --check
audit-korean-terms.mjs docs/review/2026-09-09-issue-546-plan-review.md <plan>
full Markdown readback
```

## 결정 및 남은 공백

계획 review는 **구현 진입 가능(PASS)** 이다. 그러나 계획이 최초 사용자 승인
이후 `efb4502b`와 표 설명 보정으로 중요하게 수정되었으므로, Task 1을
시작하기 전에 수정된 계획에 대한 사용자 승인을 새로 받아야 한다.

아직 수행하지 않은 항목은 의도적으로 구현 단계에 남긴다.

- Go provider dependency 설치, 코드·테스트·examples 작성
- targeted/full/race/lint/CI와 dependency/license source readback
- implementation 7-Tier review 및 lesson 완성
- managed `$bluetape-go-patterns` source/live parity와 source repository push
- PR 생성과 hosted CI

`govulncheck` 실행 파일 부재, 실제 물리 scanner/independent decoder 시험,
non-cooperative provider 호출 중 선점은 구현 후에도 별도 gap으로 보고하며,
이를 계획 PASS나 CI PASS로 과장하지 않는다.

**통합 결론: PASS — P0=0, P1=0, P2=0, P3=0. 수정 계획 사용자 승인 후
`$executing-plans`를 읽고 Task 1부터 inline 순차 구현을 시작한다.**
