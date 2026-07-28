# Cross-repo SerDe recommendation matrix

Issue #402는 raw evidence가 생긴 뒤에만 recommendation을 publish한다.

## 교훈

- Recommendation language는 benchmark evidence 범위에 묶는다. Matrix가 첫 candidate를
  이름 붙일 수는 있지만 default를 조용히 바꾸면 안 된다.
- Wire-format, trust-boundary, speed concern을 분리한다. JVM Fory/Kryo 속도가 해당
  format을 untrusted 또는 cross-language boundary에 안전하게 만들지는 않는다.
- Missing adapter evidence도 실제 결과로 취급한다. Rust serialization은 현재
  contract-first이므로 Go/JVM row가 Rust adapter claim을 만들면 안 된다.
- 사용자 README는 짧게 유지한다. 자세한 evidence는 `docs/research`에 두고 package README는
  stable caller guidance만 노출한다.

## 증거

- `docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`
- `docs/research/outputs/issue-401/`
- `serialization/README.md` and `serialization/README.ko.md`
- `codec/README.md` and `codec/README.ko.md`
- `compression/README.md` and `compression/README.ko.md`
