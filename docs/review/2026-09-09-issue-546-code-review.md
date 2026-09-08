# #546 barcode·QR pre-PR 통합 리뷰

## 판정 요약

- **판정:** APPROVE / PASS
- **검토 기준:** `origin/develop` (`51be48337427323db96d3d3ee944e6331e313b28`)
- **검토 대상:** `feat/issue-546-barcode-qr` HEAD
  (`0f6e070882c2734c3268cab24a90add7a9458fc0`)
- **범위:** `imagekit/barcode`, 양언어 README, root/imagekit index,
  `CHANGELOG.md`, Go module dependency, 계획·lesson·review 문서
- **심각도:** `P0=0`, `P1=0`, `P2=0`, `P3=0`
- **통합 결론:** 추가 수정 없이 PR gate로 진행할 수 있다. 이 리뷰에는
  merge·tag·release 권한이 포함되지 않는다.

> 아래의 hosted CI baseline lint 보강 후 최종 head는 계획·PR metadata와 함께
> 다시 갱신했다. feature production code와 public API는 변경하지 않았다.

## 이슈와 live metadata

2026-09-09에 `gh issue view 546 --repo bluetape4k/bluetape-go
--json number,title,state,assignees,milestone,labels,url,body`를 다시 실행했다.

| 항목 | 확인값 |
| --- | --- |
| 이슈 | `#546 feat: 순수 Go barcode 및 QR image helper 추가` |
| 상태/담당자 | `OPEN` / `debop` |
| milestone | `0.23.0` (number `35`) |
| labels | `type: task`, `area: testing`, `area: io`, `area: image`, `priority: p2` |
| URL | `https://github.com/bluetape4k/bluetape-go/issues/546` |
| 중복 PR | head `feat/issue-546-barcode-qr` 기준 `gh pr list --state all` 결과 `[]` |

GNO 사전 조회에서는 `bluetape4k-github`의 #546 원문과 #498의 CAPTCHA/OCR
비목표 경계를 확인했고, `bluetape4k-docs`의 기존 barcode 설계·image 경계
자료를 확인했다. `bluetape4k-wiki`에는 직접 관련 hit가 없었다. live GitHub
상태는 위 `gh` 결과를 권위 있는 값으로 사용했다.

## 요구사항 추적

| AC | 구현·검증 근거 | 결과 |
| --- | --- | --- |
| AC-01 공개 API | `imagekit/barcode/types.go`, `render.go`, `png.go`의 `Request`, `Kind`, `QRLevel`, `Render`, `EncodePNG`; 기존 `imagekit` 공개 API diff 없음 | PASS |
| AC-02 입력·크기 경계 | `barcode_test.go`의 nil context, kind, UTF-8, QR/Code128 byte·ASCII, 축·pixel 경계 및 oversized malformed 회귀 | PASS |
| AC-03 geometry·quiet zone | `render.go`의 checked span/덧셈/곱셈, QR 4-module·Code128 10-module 여백, 비영점 bounds·높이 복사 테스트 | PASS |
| AC-04 표준 이미지·PNG | provider 최소 seam을 `image.Image`로 제한하고 새 `*image.Gray`를 반환; PNG decode bounds/pixel 일치와 호출 간 buffer 비공유 테스트 | PASS |
| AC-05 오류·취소·소유권 | fixed `imagekit.Error`와 `Cause:nil`, provider 원문 비노출, pre/post/copy/write/final context checkpoint, caller-owned 결과 테스트 | PASS |
| AC-06 문서·예제 | package README EN/KO, root/imagekit 링크·index, 한국어 exported Go doc, compile-checked examples, CHANGELOG Unreleased | PASS |
| AC-07 품질·의존성 | targeted/full test·race, `make ci`, `go mod verify`, pinned provider source/license/checksum, hazard·diff audit | PASS |

## 독립 리뷰와 통합 리뷰

### 완료된 독립 lane

- `code-reviewer`: 현재 HEAD `0f6e070` 검토, **APPROVE**, 파일 21개,
  `P0/P1/P2/P3 = 0/0/0/0`.
- `architect`: 구현 HEAD의 provider seam, 입력 순서, checked geometry,
  취소 한계를 재검토, **CLEAR**, `P0/P1/P2/P3 = 0/0/0/0`.

두 리뷰에서 발견했던 이전 P2는 모두 처리했다.

| 이전 watchpoint | 처분 |
| --- | --- |
| provider-specific `Barcode`/metadata seam | `image.Image` capability로 축소하고 fake의 `Content`/`Metadata` 의존 제거 |
| UTF-8 검사보다 늦은 QR 길이 거부 | byte length를 먼저 검사하고 oversized invalid UTF-8 회귀 추가 |
| Code128 `quietPixels` 재계산 | checked 결과를 `copyCode128`과 offset에 그대로 재사용 |
| exported Go doc의 영어·linter 경계 | exported declaration과 취소 주석을 한국어 identifier-first 문장으로 정리 |

### 7-Tier 통합 표

동시 실행 슬롯 제약으로 여섯 관점 모두를 별도 native lane으로 만들 수
없었다. 따라서 아래 `main fallback PASS`는 독립 reviewer PASS가 아니라
메인 세션이 최신 diff와 증거로 수행한 통합 검토 결과다. 사용 불가 lane은
PASS로 세지 않았다.

| 관점 | 독립성 | 확인 내용 | 결과 |
| --- | --- | --- | --- |
| Performance | unavailable (thread limit) | checked arithmetic, O(width×height) bounded copy, 4 MiB writer, 32-worker stress/race | main fallback PASS |
| Stability | unavailable (thread limit) | pre/post cancellation, no detached goroutine, nil/late cancel, deterministic fresh image | main fallback PASS |
| Security | unavailable (thread limit) | bounded input, fixed/redacted provider error, no raw payload, no cgo/OCR/CAPTCHA/file/network | main fallback PASS |
| Operator/Ops | unavailable (thread limit) | caller-owned context/configuration/output, no global state/logging, aggregate CI | main fallback PASS |
| Developer/API | code-reviewer APPROVE | narrow standard capability, identifier-first Korean docs, examples, stable sentinel aliases | PASS |
| User/caller | unavailable (thread limit) | QR UTF-8/Code128 ASCII contract, quiet zones, caller-owned buffers, transform caveat and README pair | main fallback PASS |
| Architecture | architect CLEAR | package boundary, provider isolation, synchronous non-cooperative cancellation documented | PASS |

메인 통합 검토에서 추가 P0/P1/P2/P3는 발견하지 못했다.

## Hosted CI baseline lint 보강

새 계획 문서 commit 이후 hosted `ci` run `34263218885`와 failed job 재실행이
동일 head에서 다음 기존 테스트 파일의 7개 진단으로 실패했다.

- `web/gin/jwt_test.go`: `lostcancel` 2건
- `cache/redisnear/resp3_tracking_spike_test.go`: `SA5011` 4건
- `leader/etcd/campaign_test.go`: `SA5011` 1건

세 파일은 `origin/develop...HEAD` feature diff에 없었고, local
`golangci-lint 2.13.2`에서는 재현되지 않았다. configured hosted
`golangci-lint v2.12.2`와의 차이를 숨기거나 unrelated production code를
고치지 않고, 테스트 전용으로 `defer cancel()`과 nil guard의 명시적 early
return을 추가했다. 변경 후 해당 세 패키지 test/race와 `golangci-lint run
./...`가 PASS했다. 이 보강은 CI gate를 만족하기 위한 최소 test-only 범위이며,
최종 changed file count와 plan의 scope에 반영한다.

## 검증 증거

현재 HEAD에서 다음 명령을 실행했다.

- `gofmt -d imagekit/barcode` — 출력 없음
- `make fmt-check` — PASS
- `make tidy-check` — PASS
- `make vet` — PASS
- `make lint` — `0 issues`
- `go test ./imagekit/barcode -count=1` — PASS
- `go test -race ./imagekit/barcode -count=1` — PASS
- `go test -race ./imagekit/barcode -run '^TestConcurrent$' -count=10` — PASS
- `go test ./imagekit -count=1` — PASS
- `go test ./imagekit/barcode -run '^Example' -count=1` — PASS
- `go mod verify` — PASS (`all modules verified`)
- `make test` — PASS
- `make race` — PASS
- `make ci` — PASS (재실행; 첫 실행의 `sqlkit/postgis` container exit 139은
  단독 재실행 및 aggregate 재실행에서 해소됨)
- `go test ./web/gin ./cache/redisnear ./leader/etcd -count=1` — PASS
- `go test -race -p 1 ./web/gin ./cache/redisnear ./leader/etcd -count=1` — PASS
- `golangci-lint run ./... --timeout=5m` — `0 issues`
- `git diff --check origin/develop...HEAD` — PASS
- `go list ./...` — 110 packages

dependency는 `github.com/boombuler/barcode v1.1.0`이며 tag ref가
`11e32e438ffcc2af3d65aa3547c065972a743d70`으로 고정된다. provider의
Code128/QR/scaling source symbol과 MIT license를 pinned commit에서 읽었고,
checksum은 `go mod verify`로 확인했다.

## 문서 writer gate

- **SPW-01 source/purpose:** 이슈·spec·plan·provider source와 실제 구현
  파일을 연결했다 — PASS.
- **SPW-02 actionability:** AC 표, 명령, 결과, severity, disposition을
  재현 가능하게 기록했다 — PASS.
- **SPW-03 Korean/style:** 독자용 문장은 한국어이고 code/API/명령/URL은
  원문을 유지했다. 용어 audit를 별도로 실행했다 — PASS.
- **SPW-04 completeness:** 입력·geometry·오류·취소·소유권·문서·CI와
  이전 review watchpoint를 빠짐없이 추적했다 — PASS.
- **SPW-05 readback:** 최종 Markdown을 줄 단위로 읽고 base/head 및
  `P0/P1/P2/P3` 수치를 현재 상태와 대조했다 — PASS.

## 남은 검증 공백과 비목표

- 이 환경에는 `govulncheck` 실행 파일이 없어 vulnerability scan PASS를
  주장하지 않는다.
- 독립 QR decoder 왕복과 physical scanner 판독 시험은 이 순수 생성 helper의
  범위 밖이다. quiet zone·pixel·PNG 구조 검증으로 대체하지 않는다.
- provider가 context-aware API를 제공하지 않으므로 synchronous 호출을
  반환 전에 선점하지 않는다. 호출 전·후 checkpoint와 package README의
  한계 설명으로 계약을 명시했다.
- 여섯 관점의 독립 lane은 thread limit으로 unavailable이며 main fallback
  결과와 혼동하지 않는다.
- diagram, module registration, 별도 CI workflow는 이번 subpackage 범위의
  비목표이며 기존 package discovery와 aggregate CI로 N/A를 확인했다.

## 최종 권고

`P0=0`, `P1=0`, `P2=0`, `P3=0`인 **APPROVE / PASS**다. 승인된 base/head와
중복 확인을 다시 읽은 뒤 개별 PR을 생성한다. merge, tag, release, worktree
삭제는 이 리뷰와 현재 작업의 권한 범위가 아니다.
