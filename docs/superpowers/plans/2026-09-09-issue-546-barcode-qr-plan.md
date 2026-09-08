# #546 QR·Code128 이미지 helper 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `imagekit/barcode`에서 bounded QR·Code128 생성 API와 PNG 출력을 제공하고, 기존 `imagekit` API를 변경하지 않은 채 이슈 #546의 검증 가능한 개별 PR을 만든다.

**Architecture:** 공개 API는 `Request`, `Kind`, `QRLevel`, `Render`, `EncodePNG`로 제한한다. provider 호출과 표준 `*image.Gray` 복사 사이에 검증·오류 변환·정수 배율·고정 여백 경계를 둔다. provider의 `Content()`가 반환 이미지에 남지 않도록 매 호출 새 이미지를 만들고, PNG는 context-aware 4 MiB 제한 writer로 메모리에서 원자적으로 반환한다.

**Tech Stack:** Go 1.26.3 module, `github.com/boombuler/barcode v1.1.0` pinned commit `11e32e438ffcc2af3d65aa3547c065972a743d70`, standard `image`, `image/png`, `context`, 기존 `imagekit.Error`/sentinel, Go test/race/gofmt/vet/golangci-lint.

---

## 파일 책임과 변경 경계

### 새 파일

- `imagekit/barcode/doc.go`: package 설명, 지원 범위, 취소·오류·판독성 제한.
- `imagekit/barcode/types.go`: `Kind`, `QRLevel`, `Request`, 상수와 입력 검증.
- `imagekit/barcode/render.go`: provider adapter, checked geometry, QR/Code128 모듈 복사, context 검사와 표준 이미지 반환.
- `imagekit/barcode/png.go`: context-aware capped writer와 PNG 인코딩.
- `imagekit/barcode/barcode_test.go`: 공개 계약·검증·오류·취소·동시성·geometry 단위 테스트.
- `imagekit/barcode/example_test.go`: QR 한글/Code128 ASCII 사용 예제와 출력 확인.
- `imagekit/barcode/README.md`, `imagekit/barcode/README.ko.md`: 패키지 설치·API·제한·조합 주의점.
- `docs/lessons/2026-09-09-issue-546-barcode-qr.md`: provider 경계와 이미지 판독성의 재발 방지 기록.
- `docs/review/2026-09-09-issue-546-code-review.md`: pre-PR 7-Tier 통합 리뷰.

### 수정 파일

- `go.mod`, `go.sum`: 승인된 `github.com/boombuler/barcode v1.1.0` direct dependency와 checksum.
- `imagekit/README.md`, `imagekit/README.ko.md`: barcode 하위 패키지 링크와 transform 조합 시 resize/crop 주의점.

기존 `imagekit/*.go`의 공개 타입·함수와 오류 구현은 수정하지 않는다. Go module은 별도 package 등록 파일이 필요 없으므로 catalog/workflow 파일은 변경하지 않는다.

## Task 1: dependency를 고정하고 RED 테스트 골격을 만든다

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `imagekit/barcode/barcode_test.go`

- [ ] **Step 1: dependency와 checksum을 추가한다**

Run:

```bash
go get github.com/boombuler/barcode@v1.1.0
go mod tidy
go mod verify
```

Expected: `go.mod`에 `github.com/boombuler/barcode v1.1.0`이 direct require로 기록되고 `go mod verify`가 `all modules verified`를 반환한다. provider source ref와 module version은 pinned commit `11e32e438ffcc2af3d65aa3547c065972a743d70` 및 `gh api` source readback과 대조한다.

- [ ] **Step 2: 공개 계약의 실패 테스트를 먼저 작성한다**

`barcode_test.go`는 `package barcode_test`로 시작하고 다음 table을 만든다.

```go
tests := []struct {
    name string
    req  barcode.Request
    want error
}{
    {"nil context", barcode.Request{Kind: barcode.QR, Content: "x", Width: 128, Height: 128}, barcode.ErrInvalidOptions},
    {"unknown kind", barcode.Request{Kind: barcode.Kind(99), Content: "x", Width: 128, Height: 128}, barcode.ErrInvalidOptions},
    {"empty content", barcode.Request{Kind: barcode.QR, Content: "", Width: 128, Height: 128}, barcode.ErrInvalidOptions},
    {"zero size", barcode.Request{Kind: barcode.QR, Content: "x", Width: 0, Height: 128}, barcode.ErrInvalidOptions},
    {"non-square QR", barcode.Request{Kind: barcode.QR, Content: "x", Width: 128, Height: 127}, barcode.ErrInvalidOptions},
    {"QR content limit", barcode.Request{Kind: barcode.QR, Content: strings.Repeat("x", 1025), Width: 128, Height: 128}, barcode.ErrInputTooLarge},
    {"Code128 control", barcode.Request{Kind: barcode.Code128, Content: "A\nB", Width: 256, Height: 64}, barcode.ErrInvalidOptions},
    {"width limit", barcode.Request{Kind: barcode.Code128, Content: "A", Width: 4097, Height: 64}, barcode.ErrImageTooLarge},
}
```

각 행은 `Render(context.Background(), req)`의 `errors.Is`를 검사하고 provider가 호출되지 않는 입력은 내부 호출 카운터 seam으로 확인한다. 이 시점에는 package와 함수가 없으므로 컴파일 또는 테스트가 실패해야 한다(RED).

- [ ] **Step 3: RED 결과를 읽고 첫 커밋을 만든다**

Run: `go test ./imagekit/barcode -run 'TestRenderRejects|TestEncodePNGRejects' -count=1`

Expected: `package .../imagekit/barcode` 또는 미정의 symbol 오류로 실패한다. 실패가 아닌 경우 테스트가 잘못된 것이므로 assertion을 먼저 고친다.

Commit: `test: barcode 입력과 크기 경계를 먼저 고정한다`

## Task 2: 타입·검증·오류 변환을 구현한다

**Files:**
- Create: `imagekit/barcode/types.go`
- Create: `imagekit/barcode/render.go`
- Create: `imagekit/barcode/doc.go`
- Test: `imagekit/barcode/barcode_test.go`

- [ ] **Step 1: 공개 타입과 sentinel alias를 정의한다**

`types.go`의 핵심 형태는 다음과 같다.

```go
type Kind uint8

const (
    QR Kind = iota + 1
    Code128
)

type QRLevel uint8

const (
    QRLevelM QRLevel = iota
    QRLevelL
    QRLevelQ
    QRLevelH
)

type Request struct {
    Kind    Kind
    Content string
    Width   int
    Height  int
    QRLevel QRLevel
}
```

`var`로 `imagekit.ErrInvalidOptions`, `ErrInputTooLarge`, `ErrImageTooLarge`, `ErrEncode`를 alias해 호출자가 `errors.Is`를 쓸 수 있게 한다. `validateRequest`는 context nil, kind, UTF-8, 콘텐츠 byte/rune 범위, QR level, 양수 크기, 4,096 각 축, 4,194,304 픽셀을 provider 호출 전 검사한다. QR의 `QRLevelM/L/Q/H` 값과 Code128에서 `QRLevelM`만 허용하는 매핑을 명시한다.

- [ ] **Step 2: 안전한 provider 오류 변환을 구현한다**

`newEncodeError(format string)`는 `&imagekit.Error{Kind: imagekit.ErrEncode, Operation: "render", Format: format, Cause: nil}`만 만든다. provider가 반환한 오류 객체는 저장·출력·로그에 사용하지 않는다. `context.Canceled`와 `context.DeadlineExceeded`는 원형으로 반환하고, 나머지 helper 오류는 고정 sentinel로 변환한다.

- [ ] **Step 3: 검증과 오류 테스트를 GREEN으로 만든다**

콘텐츠 길이의 정확히 1,024/1,025 byte, Code128 80 byte 및 printable ASCII 경계, invalid UTF-8 문자열, 각 축 4,096/4,097, 픽셀 4,194,304/4,194,305를 table test로 추가한다. 오류 문자열에는 payload가 없고 테스트 전용 `fakeProviderError`에 대해 `var cause *fakeProviderError; errors.As(err, &cause)`가 거짓이며, `errors.Is`가 imagekit sentinel과 일치하는지 확인한다.

Run: `go test ./imagekit/barcode -run 'TestRenderRejects|TestValidate|TestError' -count=1`

Expected: all tests PASS.

Commit: `feat: barcode 요청과 안전한 오류 경계를 추가한다`

## Task 3: provider adapter와 정수 모듈 renderer를 구현한다

**Files:**
- Modify: `imagekit/barcode/render.go`
- Test: `imagekit/barcode/barcode_test.go`

- [ ] **Step 1: provider 호출 seam을 만든다**

공개 함수는 `Render(ctx context.Context, req Request) (image.Image, error)`로 고정하고, 내부 `renderWithEncoder(ctx, req, encoder)`는 테스트용 함수 주입만 허용한다. 실제 encoder는 `qr.Encode(req.Content, mappedLevel, qr.Unicode)` 또는 `code128.Encode(req.Content)`를 호출한다. provider 타입은 공개 구조체에 저장하지 않는다.

- [ ] **Step 2: provider 결과를 checked geometry로 복사한다**

provider 결과가 nil이거나 빈 bounds면 `ErrEncode`를 반환한다. `symbolWidth`와 `symbolHeight`를 bounds에서 읽고, QR은 두 값이 같음을 확인한다. quiet zone은 QR 4 module, Code128 좌우 10 module로 둔다. 다음 조건을 checked integer arithmetic으로 계산한다.

```text
requiredWidth  = symbolWidth + 2*quiet
requiredHeight = symbolHeight + 2*quiet  // QR; Code128은 symbolHeight
scaleX = Width / requiredWidth
scaleY = Height / requiredHeight
scale  = min(scaleX, scaleY)
```

`scale < 1`이면 `ErrInvalidOptions`; 곱셈·덧셈 overflow 또는 전체 pixel 상한 위반이면 `ErrImageTooLarge`다. `*image.Gray`를 흰색으로 채운 뒤 중앙 offset에 provider module을 정수 배율로 복사한다. QR은 두 축에 quiet zone을 포함하고, Code128은 x축 quiet zone과 요청 높이를 사용한다. `color.GrayModel.Convert` 결과의 명도 `< 128`만 검정으로, 나머지는 흰색으로 정규화한다.

- [ ] **Step 3: geometry·detachment·결정성 테스트를 GREEN으로 만든다**

실제 provider 출력으로 QR 256×256, Code128 512×128을 생성하고 다음을 검사한다.

- bounds가 정확히 요청 크기다.
- QR은 정방형이고 네 방향 quiet zone이 흰색이다.
- Code128 좌우 10 module 이상이 흰색이고 bars는 높이를 채운다.
- 1 pixel 부족 canvas는 실패하고 정확히 필요한 canvas는 성공한다.
- 두 번의 동일 요청 결과는 모든 픽셀이 같으며 반환 타입에는 provider `Content()` 메서드가 없다.
- 반환 이미지끼리 pixel buffer를 공유하지 않는다.

Run: `go test ./imagekit/barcode -run 'TestRender|TestGeometry|TestDetachment' -count=1`

Expected: all tests PASS.

Commit: `feat: QR과 Code128을 표준 Gray 이미지로 렌더링한다`

## Task 4: bounded PNG와 취소 lifecycle을 구현한다

**Files:**
- Create: `imagekit/barcode/png.go`
- Modify: `imagekit/barcode/render.go`
- Test: `imagekit/barcode/barcode_test.go`

- [ ] **Step 1: context-aware capped writer를 테스트로 고정한다**

writer는 `Write(p []byte)` 직전에 context를 확인하고, 누적 길이가 4 MiB를 초과하면 고정 `errPNGTooLarge`를 반환한다. context error가 cap error보다 먼저 관측되도록 precedence를 고정한다. PNG encode 실패 시 호출자에게 부분 buffer를 반환하지 않는다.

- [ ] **Step 2: `EncodePNG`를 구현한다**

`EncodePNG(ctx, req)`는 `Render` 후 `png.Encode(limitWriter, img)`를 실행한다. writer의 각 write와 encode 후 context를 확인한다. 성공 시 새 byte slice를 반환하고, 취소·provider 오류·PNG 한도 초과·writer 오류는 `nil` 결과와 고정 imagekit 오류를 반환한다. detached goroutine, global buffer, 내부 retry를 만들지 않는다.

- [ ] **Step 3: lifecycle과 출력 테스트를 GREEN으로 만든다**

다음을 테스트한다.

- 이미 취소된 context는 provider 호출 전 `context.Canceled`를 반환한다.
- provider 이후·행 복사 중·PNG write 직전·최종 반환 직전 취소는 원형 context error와 nil 결과를 반환한다.
- 정상 PNG를 `image/png.Decode`해 bounds와 흑백 픽셀이 `Render` 결과와 일치한다.
- 4 MiB 경계와 초과 출력은 `ErrEncode`, nil 결과, partial bytes 미반환을 보장한다.
- `EncodePNG` 호출 간 buffer가 공유되지 않는다.

Run: `go test ./imagekit/barcode -run 'TestEncodePNG|TestCancellation|TestPNG' -count=1`

Expected: all tests PASS.

Commit: `feat: bounded PNG 출력과 취소 경계를 추가한다`

## Task 5: examples·README 양언어·lesson을 동기화한다

**Files:**
- Create: `imagekit/barcode/example_test.go`
- Create: `imagekit/barcode/README.md`
- Create: `imagekit/barcode/README.ko.md`
- Modify: `imagekit/README.md`, `imagekit/README.ko.md`
- Create: `docs/lessons/2026-09-09-issue-546-barcode-qr.md`

- [ ] **Step 1: 실행 가능한 example을 추가한다**

`ExampleRender_qr`는 한글 UTF-8 payload를 256×256으로 렌더링하고 `Bounds().Dx()/Dy()`를 출력한다. `ExampleEncodePNG_code128`은 printable ASCII payload를 512×128 PNG로 인코딩하고 `image/png` decode bounds를 출력한다. 예제 출력은 크기와 오류 없는 결과만 고정하며 raw PNG byte golden은 고정하지 않는다.

- [ ] **Step 2: package README 두 언어를 작성한다**

설치 경로, dependency 버전, `Request` 예제, QR UTF-8/Code128 ASCII allowlist, 4 MiB·4,096² 제한, quiet zone, context 한계, 오류 sentinel, provider 원문 비노출, 스캐너 인증이 아님을 설명한다. 기존 `imagekit.Transform`에 PNG를 전달할 수 있지만 arbitrary resize/crop/JPEG는 판독성을 훼손할 수 있다는 경계를 명시한다.

- [ ] **Step 3: parent README와 lesson을 동기화한다**

`imagekit/README.md`와 `README.ko.md`의 지원 기능/하위 패키지 링크를 같은 위치에 추가한다. lesson에는 provider의 `Content()` 누출, quiet zone 보장 부재, non-cooperative cancellation을 구현 watchpoint로 기록한다.

- [ ] **Step 4: 문서 검증을 수행한다**

Run:

```bash
gofmt -w imagekit/barcode/*.go
go test ./imagekit/barcode -run '^Example' -count=1
git diff --check
node /Users/debop/.codex/skills/bluetape-writer/scripts/audit-korean-terms.mjs imagekit/barcode/README.ko.md imagekit/README.ko.md docs/lessons/2026-09-09-issue-546-barcode-qr.md
```

Expected: examples, diff check, Korean terminology audit all PASS.

Commit: `docs: barcode 사용법과 provider 경계를 기록한다`

## Task 6: 통합 검증과 repository hazard 확인

**Files:**
- Inspect: all changed files, `go.mod`, `go.sum`, `.github/workflows`, Makefile

- [ ] **Step 1: package targeted test와 race를 순차 실행한다**

Run:

```bash
go test ./imagekit/barcode -count=1
go test -race ./imagekit/barcode -count=1
go test ./imagekit -count=1
go mod verify
```

Expected: all commands exit 0. race test에는 32개 동시 QR/Code128 호출을 포함하고, 실제 provider 내부 goroutine이 남지 않는지 테스트 종료 후 goroutine count로 추정하지 않고 race/결과 소유권으로 검증한다.

- [ ] **Step 2: repository-wide quality gate를 실행한다**

Run sequentially: `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci`.

Expected: each command exits 0. `make tidy-check`가 dependency drift를 보고하면 go.mod/go.sum만 정리하고 source behavior를 바꾸지 않은 뒤 모든 affected test를 다시 실행한다.

- [ ] **Step 3: dependency·license·source evidence를 고정한다**

Run:

```bash
go list -m -json github.com/boombuler/barcode
go mod verify
gh api 'repos/boombuler/barcode/contents/LICENSE?ref=11e32e438ffcc2af3d65aa3547c065972a743d70' -H 'Accept: application/vnd.github.raw'
```

Expected: module version `v1.1.0`, checksum verified, pinned source commit and MIT license readback. `govulncheck` is not a repository command in this environment; record that absence as a validation gap and do not claim a vulnerability scan PASS.

- [ ] **Step 4: hazard·diff·API audit를 수행한다**

Inspect `git diff --stat`, `git diff --check`, `go list ./...`, exported Go doc, both README pairs, no catalog/workflow registration requirement, no cgo/OCR/CAPTCHA/file/network side effect, and no modifications outside the approved file list. `git status --short` must show only intended files.

Commit: `test: barcode 전체 검증과 dependency 경계를 확인한다`

## Task 7: verifier·7-Tier pre-PR review와 lesson 확인

**Files:**
- Inspect: approved spec and plan, all implementation files
- Modify: `docs/review/2026-09-09-issue-546-code-review.md`, `docs/lessons/2026-09-09-issue-546-barcode-qr.md` when review evidence requires it

- [ ] **Step 1: spec·plan traceability verifier를 수행한다**

AC-01부터 AC-07까지를 public API, input validation, geometry, errors, cancellation, docs, commands와 일대일로 대조한다. 누락은 구현으로 덮지 말고 plan 또는 spec을 먼저 수정하고 해당 승인/검토 gate를 다시 연다.

- [ ] **Step 2: six perspective lanes와 main integration을 실행한다**

Performance, stability, security, operator/Ops, developer/API, user/caller를 각각 최신 diff와 테스트 결과에 대해 검토한다. 성능·안정성 reviewer가 확인할 watchpoint는 checked arithmetic, capped writer, provider non-cooperative call, race와 반환 이미지 소유권이다. P0/P1은 0건이어야 하며 P2/P3는 수정 또는 명시적 후속 issue/rationale를 남긴다.

- [ ] **Step 3: review writer gate와 lesson을 완료한다**

통합 review와 lesson에 SPW-01~05를 기록하고, 한국어 기술 문체·용어 audit·source/command traceability·최종 Markdown readback을 수행한다. `docs/review/2026-09-09-issue-546-code-review.md`에는 priority, 근거, disposition, 남은 검증 gap, P0/P1 count를 넣는다.

- [ ] **Step 4: pre-PR DoD를 렌더링한다**

현재 head/base, changed files, commits, local commands, dependency evidence, docs parity, known gaps(`govulncheck` 부재와 실제 물리 scanner 시험 미수행), no-merge boundary를 Korean PR body 형식으로 정리한다. PR 생성 전 common gate와 linked issue metadata를 다시 읽는다.

## 실행 순서와 승인 경계

```text
Task 1 RED → Task 2 validation → Task 3 renderer → Task 4 PNG/cancel
→ Task 5 docs/examples/lesson → Task 6 repository verification
→ Task 7 verifier/7-Tier/pre-PR → individual PR creation
```

각 Task의 commit은 의도 단위로 유지하고, 작업 중 발견한 P0/P1은 다음 Task로 넘기지 않고 해당 테스트와 문서를 먼저 수정한다. PR 생성은 Task 7 완료 후 승인된 저장소 `bluetape4k/bluetape-go`, base `develop`, head `feat/issue-546-barcode-qr`에 한정한다. 머지·tag·release·worktree 삭제는 이 계획의 권한 범위가 아니다.

## 롤백과 재실행

- dependency 또는 provider 계약이 실패하면 `imagekit/barcode` 파일과 해당 `go.mod`/`go.sum` hunk를 함께 되돌리고 기존 `imagekit` 테스트를 재실행한다.
- geometry/cancellation test 실패는 해당 Task의 RED부터 다시 실행하며, 이전 PASS를 근거로 삼지 않는다.
- 문서·README drift는 source/API가 고정된 뒤 양언어 audit와 examples를 다시 실행한다.
- 외부 서비스·파일·네트워크 상태는 변경하지 않으므로 데이터 복구 작업은 없다.

## Plan DoD

- [x] 승인된 amended spec `1ebee2a`와 spec review `3a5f043`의 P0=0/P1=0 결과를 반영했다.
- [x] 모든 AC에 파일·테스트·명령·rollback 지점을 매핑했다.
- [ ] 계획 사용자 승인.
- [ ] 계획 6개 관점 및 메인 통합 review.
- [ ] 구현·검증·lesson·pre-PR review.
- [ ] 개별 PR 생성과 hosted CI.

현재 문서는 코드 구현 승인을 의미하지 않는다. 다음 단계는 작성된 plan 사용자 승인 후 plan review이며, 그 전에는 Task 1 dependency 변경과 구현을 시작하지 않는다.

## 계획 자체 검토와 문서 게이트

- **SPW-01 (audience/purpose/source): PASS** — 승인된 amended spec `1ebee2a`, spec review `3a5f043`, provider source ref, 그리고 이슈 #546의 API·보안·검증 경계를 계획의 목표와 기술 스택에 명시했다. 구현 전 실제 동작은 아직 검증하지 않는다.
- **SPW-02 (actionability): PASS** — 각 Task에 책임 파일, 순서가 있는 명령, 예상 결과, 실패 시 재실행·rollback 경로, 커밋 의도를 적었다. `go get`과 구현은 계획 승인 뒤에만 실행한다.
- **SPW-03 (Korean technical naturalness): PASS** — 독자용 계획·명령 설명은 한국어로 작성하고 Go/API 식별자·명령·URL·정확한 오류 토큰은 원문을 보존했다. `audit-korean-terms.mjs` 결과를 커밋 전 확인한다.
- **SPW-04 (completeness/consistency): PASS** — AC-01~AC-07을 Task 1~7의 파일·테스트·검증 명령에 일대일 매핑했다. provider 오류 `Cause == nil`, `errors.As` 비복원, context 우선순위, 정수 geometry, PNG nil 반환, README 양언어 parity를 빠뜨리지 않았다.
- **SPW-05 (readback): PASS** — 전체 계획을 줄 단위로 다시 읽었고 미정 기입란 및 모호한 구현 지시를 제거했다. 남은 `[ ]`는 사용자 승인·후속 실행 단계의 상태 표시에만 사용한다.

**Coverage summary:** AC-01 → Task 2·3·5, AC-02 → Task 1·2·3·4, AC-03 → Task 3, AC-04 → Task 3·4, AC-05 → Task 2·4·6, AC-06 → Task 5·7, AC-07 → Task 1·6·7. 계획 검토 후 구현 전에 별도 6개 관점 + main integration review를 수행한다.
