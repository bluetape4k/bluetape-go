# Issue #55 Tokenizer Dependency Boundary

Large NLP dictionaries and language models must stay behind optional packages.

What resolved the decision:

- Kagome is a strong Japanese path, but the module plus IPA dictionary is about
  53M in module cache; UniDic pushes the path higher.
- Lingua-Go is closest to source parity, but its cache size is about 123M and
  upstream documents lazy/eager model loading tradeoffs.
- Whatlanggo is tiny, but it does not match mixed-language parity enough to be
  the first selected adapter.
- Korean full tokenizer parity remains too broad for a Go helper package unless
  a mature Go-native analyzer appears.

Rule for future work:

- `textsearch` core remains dependency-free.
- Japanese and language detection features must be separate optional packages.
- Dependency-backed text features need `GoroutineStressTester` and race gates
  for detector/tokenizer reuse claims.
- Follow-up implementation belongs in #336 and #337; Korean full tokenizer
  remains deferred without an implementation issue.
