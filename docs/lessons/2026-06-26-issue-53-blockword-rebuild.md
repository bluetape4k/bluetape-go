# Issue #53 Blockword Rebuild Scope 교훈

`textsearch` blockword support는 #52 matcher 위의 deterministic compiled dictionary
layer로 유지해야 한다.

- runtime dictionary mutation은 새 `BlockwordDictionary`를 compile하고 caller boundary에서
  swap하는 방식으로 표현한다.
- severity filtering은 non-overlapping match selection 전에 실행되어야 한다. 그래야
  filtered-out low-severity overlap이 higher-severity match를 숨길 수 없다.
- request validation은 raw user input이 아니라 length만 보고한다.
- Korean 및 Japanese example은 dictionary masking fixture이지 morphological tokenizer
  parity 주장으로 보지 않는다.
- masking은 moderation 또는 security boundary가 아니라 helper behavior로 남는다.
