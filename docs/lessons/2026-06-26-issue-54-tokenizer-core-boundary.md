# Issue #54 Tokenizer Core Boundary

`textsearch` tokenizer core should stay dependency-free until Korean/Japanese
tokenizer and language-detection research proves a concrete adapter belongs in
this repository.

What worked:

- Keep spans as original string byte offsets so callers can slice Go strings
  without normalized-offset contracts.
- Put `Tokenizer`, `TokenizerFunc`, `Token`, `TokenizeRequest`, and dictionary
  provider/set contracts in `textsearch`; keep `SimpleTokenizer` explicitly
  lexical and deterministic for tests.
- Reuse existing `NormalizeMode`, `Severity`, blockword request/response, and
  `testing/concurrency.GoroutineStressTester` instead of adding model or
  dependency surfaces.

Boundary:

- `SimpleTokenizer` is not Korean/Japanese morphological analysis and does not
  perform language detection.
- Token metadata is the extension point for analyzer-specific POS tags,
  language labels, source dictionaries, and confidence.
- `AsyncJobTester` is not applicable because #54 adds no asynchronous,
  context-aware tokenizer execution path; `DictionaryProvider` cancellation is
  covered by direct context cancellation test.
