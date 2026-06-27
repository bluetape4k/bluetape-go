# Issue 336 - Kagome Japanese Adapter Boundary

## Lesson

Kagome's public token has both `Position` and `Start`/`End`. `Position` is the
byte offset that matches `textsearch.TokenSpan`; `Start`/`End` are rune-oriented
positions. The adapter must use `Position + len(Surface)` for caller-visible
byte spans and test this on every returned token.

## Boundary

- Keep `textsearch` core dependency-free.
- Put Kagome and dictionary imports only under `textsearch/japanese`.
- Default to IPA dictionary and expose `WithDictionary` for deliberate opt-in
  dictionary swaps.
- Treat Kagome POS as metadata while keeping `textsearch.PartOfSpeech` coarse.
- Async/cancellation helpers are not applicable because Kagome tokenization is
  synchronous CPU-local work; concurrency coverage uses `GoroutineStressTester`.

## Verification Targets

- `go test -count=1 ./textsearch ./textsearch/japanese`
- `go test -race -count=1 ./textsearch/japanese`
- `make ci`
