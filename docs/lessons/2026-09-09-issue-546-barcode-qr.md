# #546 QR·Code128 구현 lesson

## 결정

`github.com/boombuler/barcode v1.1.0`은 provider 인코딩만 담당하게 두고,
`imagekit/barcode`가 입력·크기·geometry·출력 소유권과 오류 경계를 책임진다.
provider의 `Content()`가 반환 image에 남지 않도록 매 호출 새 `*image.Gray`를
만든다.

## 구현 watchpoint

- provider 호출 전에 UTF-8/printable ASCII allowlist와 byte 길이, 축·pixel 상한을
  검사한다. provider가 자체적으로 허용하는 범위보다 좁은 caller 계약을 먼저
  적용한다.
- provider 반환 직후 geometry 계산·image allocation·오류 변환 전에 context를
  다시 확인한다. context-aware API가 없는 provider 호출을 detached goroutine으로
  감싸지 않고, 반환 후 checkpoint가 늦은 성공을 폐기하게 한다.
- bounds의 `Max-Min` span과 geometry 덧셈·곱셈을 checked arithmetic으로 처리한다.
  QR은 네 방향 4-module, Code128은 좌우 10-module quiet zone을 표준 Gray 복사
  단계에서 보장한다.
- Code128 provider의 1-pixel symbol을 요청 높이 전체에 복제하고, QR/Code128
  결과를 호출별로 새로 만들어 caller가 반환 image와 PNG buffer를 독립적으로
  소유하게 한다.
- PNG writer는 context를 각 write에서 확인하고 4 MiB cap을 넘으면 부분 byte를
  반환하지 않는다. provider 오류와 writer 오류는 fixed `ErrEncode`로 redaction한다.

## 재사용 가능한 Go 규칙

외부 image/binary provider adapter를 추가할 때는 (1) dispatch 전 bounded input,
(2) provider metadata/content API가 아닌 `image.Image`·`io.Writer` 같은 최소
표준 capability seam, (3) caller-owned 표준 출력 복사, (4) checked geometry와
명시적 여백, (5) bounded output writer, (6) provider 반환·publish checkpoint의
cancellation 우선순위를 함께 검토한다. byte-size 상한은 UTF-8 검사·canonicalization·
decode보다 먼저 적용하고, oversized malformed input도 같은 분류를 유지하는지
테스트한다. checked area·scale·offset은 한 번 계산한 값을 allocation과 copy에
재사용해 검증된 bound와 실제 geometry가 어긋나지 않게 한다. provider가 caller
cancellation을 직접 지원하지 않는 경우에는 즉시 선점을 주장하지 않고 그 한계를
README와 테스트에 적는다.

## 검증 근거

Task 3·4의 package-local seam과 public tests가 malformed/nil/output-plus-error,
오류 문자열·`Unwrap`·`errors.As`, nonzero bounds, quiet zone, detachment,
PNG cap, cancellation precedence를 확인한다. 최종 HEAD에서 다음 검증을 fresh하게
실행했다.

- `go test ./imagekit/barcode -count=1` — PASS
- `go test -race ./imagekit/barcode -run '^TestConcurrent$' -count=10` — PASS
- `go test -race ./imagekit/barcode -count=1` — PASS
- `go test ./imagekit -count=1` — PASS
- `go test ./imagekit/barcode -run '^Example' -count=1` — PASS
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint` — PASS
- `make test`, `make race`, `make ci` — PASS
- `go mod verify` — PASS

첫 aggregate 실행에서는 기존 `sqlkit/postgis` Testcontainers가 exit 139로
실패했으나 해당 패키지 단독 재실행과 aggregate 재실행이 모두 PASS했다. 이는
barcode 변경과 무관한 환경성 관찰로 review에 남긴다.

## 남은 공백

`govulncheck` 실행 파일이 없는 환경이므로 vulnerability scan PASS를 주장하지
않는다. 독립 decoder 왕복과 physical scanner 시험은 범위 밖이며, non-cooperative
provider 호출 중 즉시 선점도 보장하지 않는다.
