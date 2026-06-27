# Issue #53 Blockword Rebuild Scope

`textsearch` blockword support should stay a deterministic compiled dictionary
layer over the #52 matcher.

- Runtime dictionary mutation is represented by compiling a new
  `BlockwordDictionary` and swapping it at the caller boundary.
- Severity filtering must run before non-overlapping match selection so a
  filtered-out low-severity overlap cannot hide a higher-severity match.
- Request validation reports only lengths, not raw user input.
- Korean and Japanese examples are dictionary masking fixtures, not claims of
  morphological tokenizer parity.
- Masking remains helper behavior and not a moderation or security boundary.
