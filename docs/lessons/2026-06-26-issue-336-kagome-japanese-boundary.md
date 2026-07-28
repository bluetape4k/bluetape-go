# Issue #336 Kagome Japanese Adapter Boundary 교훈

## 교훈

Kagome의 public token에는 `Position`과 `Start`/`End`가 모두 있다. `Position`은
`textsearch.TokenSpan`과 맞는 byte offset이고, `Start`/`End`는 rune-oriented position이다.
adapter는 caller-visible byte span에 `Position + len(Surface)`를 사용해야 하며, 반환되는
모든 token에서 이를 test해야 한다.

## Boundary

- `textsearch` core는 dependency-free로 유지한다.
- Kagome 및 dictionary import는 `textsearch/japanese` 아래에만 둔다.
- IPA dictionary를 default로 삼고, 의도적 opt-in dictionary swap에는 `WithDictionary`를
  노출한다.
- Kagome POS는 metadata로 다루고 `textsearch.PartOfSpeech`는 coarse하게 유지한다.
- Kagome tokenization은 synchronous CPU-local work이므로 async/cancellation helper는
  적용되지 않는다. concurrency coverage에는 `GoroutineStressTester`를 사용한다.

## Verification Targets

- `go test -count=1 ./textsearch ./textsearch/japanese`
- `go test -race -count=1 ./textsearch/japanese`
- `make ci`
