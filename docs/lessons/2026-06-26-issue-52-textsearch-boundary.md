# Issue #52 Textsearch Boundary 교훈

`textsearch`는 full tokenizer나 moderation system이 아니라 deterministic compiled search
package로 유지해야 한다.

- immutable compiled matcher는 concurrent read를 단순하고 race-testable하게 유지한다.
- replacement와 masking은 #53의 integration hook이지만, masking은 security boundary가
  아니라 helper behavior로 남는다.
- Unicode normalization은 caller ergonomics에 유용하지만, offset reporting은 계속
  original input byte span을 가리켜야 한다.
- boundary mode는 명시적이며 ASCII 또는 Unicode word rune으로 제한된다.
- external Aho-Corasick engine은 benchmark가 first-party implementation이 부족하다고
  증명할 때까지 deferred로 둔다.
