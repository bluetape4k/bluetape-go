# Issue 337 - Lingua language 경계

## 교훈

Lingua-Go는 source `lingua` module과 가장 가까운 parity 경로지만, language model과
transitive dependency의 운영 비용이 작지 않다. dependency는 `textsearch/language`
뒤에 두고, `go list -deps ./textsearch`에 Lingua import가 없음을 계속 검증한다.

## 경계

- Detector는 language subset별로 한 번 만들고 goroutine 사이에서 재사용한다.
- Domain을 아는 경우 all-language detector보다 caller-selected subset을 선호한다.
- 알 수 없는 입력은 운영 실패가 아니라 caller-visible `Detected=false` result로 둔다.
- Language detection은 preprocessing hint이지 security 또는 moderation boundary가
  아니다.
- Shared detector reuse 주장은 `GoroutineStressTester`와 `go test -race`로
  증명한다.

## 검증 대상

- `go test -count=1 ./textsearch/language`
- `go test -race -count=1 ./textsearch/language`
- `go list -deps ./textsearch | rg "pemistahl|lingua-go" || true`
- `make ci`
