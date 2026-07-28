# Issue #207 Wildcard와 Hash Utility 교훈

## 변경된 점

- `core`에 case-sensitive string matching, lexical path matching, escaped literal,
  `**` path segment를 위한 Go-native wildcard helper를 추가했다.
- 명시적 non-cryptographic boundary를 가진 deterministic XXH64 byte/string helper를
  추가했다.
- English/Korean `core` README를 갱신하고 `cespare/xxhash` dependency rationale을
  `bluetape4k-wiki`에 보존했다.

## 교훈

- slash-separated wildcard path pattern에는 명시적 escape rule이 필요하다. `/`와 `\`는
  input separator로 다루되, slash-separated pattern segment 안에서는 `\*`, `\?`,
  `\\` escape를 지원해 caller가 Windows-path portability를 잃지 않고 literal wildcard
  character를 match할 수 있게 한다.
- generic object hashing은 Go의 잘못된 parity target이다. JVM `hashCode()` behavior를
  흉내 내기보다 hash helper를 byte/string 또는 caller-owned encoding boundary에 둔다.
- `make tidy-check`는 intentional `go.mod` 또는 `go.sum` update가 check 전에 staged되어
  있기를 기대한다. 이 check가 `go mod tidy` 실행 후 추가 unstaged diff가 없음을
  검증하기 때문이다.
- worktree를 edit할 때는 `apply_patch`에 absolute path를 사용한다. relative patch는
  tool이 working directory를 받지 않을 때 main checkout에 잘못 쓸 수 있다.
- native subagent cleanup은 manager layer에서 hang될 수 있다. stale slot을 회복할 수
  없다면 unavailability를 기록하고, 무기한 blocking하지 말고 main session에서 같은
  7-tier review shape를 수행한다.

## 검증

- `go test -count=1 ./core`
- `go test -race -count=1 ./core`
- `go test ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `git diff --check`
