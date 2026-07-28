# Issue #54 Tokenizer Core 경계

`textsearch` tokenizer core는 Korean/Japanese tokenizer와 language-detection
조사가 이 저장소에 둘 adapter를 구체적으로 증명하기 전까지 dependency-free로
유지한다.

효과가 있었던 결정:

- span은 원본 문자열의 byte offset으로 유지한다. 호출자가 별도의 normalized-offset
  계약 없이 Go string을 slice할 수 있어야 한다.
- `Tokenizer`, `TokenizerFunc`, `Token`, `TokenizeRequest`, dictionary
  provider/set 계약은 `textsearch`에 둔다. `SimpleTokenizer`는 테스트용으로
  lexical하고 deterministic한 구현임을 명확히 한다.
- 새 model 또는 dependency 표면을 만들기보다 기존 `NormalizeMode`,
  `Severity`, blockword request/response, `testing/concurrency.GoroutineStressTester`를
  재사용한다.

경계:

- `SimpleTokenizer`는 Korean/Japanese morphological analysis가 아니며 language
  detection도 수행하지 않는다.
- Token metadata는 analyzer별 POS tag, language label, source dictionary,
  confidence를 담기 위한 확장 지점이다.
- #54는 asynchronous, context-aware tokenizer 실행 경로를 추가하지 않았으므로
  `AsyncJobTester`는 적용 대상이 아니다. `DictionaryProvider` cancellation은 직접
  context cancellation test로 다룬다.
