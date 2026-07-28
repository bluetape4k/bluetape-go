# Issue #222 교훈

- standard `testing`, `cmp.Diff`, package-local builder, 기존 bluetape-go helper가
  이미 workflow를 덮는다면 public assertion DSL보다 focused testing example이 Go에 더
  잘 맞는다.
- golden fixture는 package-local `testdata` 아래에 둔다. `TempOutputPath`는 generated
  output용이지 canonical expected file용이 아니다.
- seeded `math/rand/v2` example은 정확히 생성된 값을 assert해야 한다. 그래야 CI가
  random-looking data를 보여주기만 하는 대신 determinism을 증명한다.
- `cmp.Diff`를 직접 import하면 `github.com/google/go-cmp`가 이미 transitive로 있어도
  direct module dependency여야 한다.
