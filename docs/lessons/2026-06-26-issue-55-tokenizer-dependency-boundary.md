# Issue #55 Tokenizer dependency 경계

대형 NLP dictionary와 language model은 optional package 뒤에 둔다.

결정을 정리한 근거:

- Kagome은 Japanese 경로로 강하지만 module과 IPA dictionary만으로 module cache가
  약 53M이다. UniDic을 쓰면 더 커진다.
- Lingua-Go는 source parity에 가장 가깝지만 cache size가 약 123M이고, upstream도
  lazy/eager model loading tradeoff를 문서화한다.
- Whatlanggo는 작지만 mixed-language parity가 첫 adapter로 선택할 만큼 맞지 않았다.
- Korean full tokenizer parity는 성숙한 Go-native analyzer가 나타나기 전까지 Go
  helper package 범위로는 너무 넓다.

향후 규칙:

- `textsearch` core는 dependency-free로 유지한다.
- Japanese와 language detection 기능은 별도 optional package여야 한다.
- dependency-backed text 기능에서 detector/tokenizer 재사용을 주장하려면
  `GoroutineStressTester`와 race gate가 필요하다.
- 후속 구현은 #336과 #337에서 다룬다. Korean full tokenizer는 구현 이슈가 생기기
  전까지 보류한다.
