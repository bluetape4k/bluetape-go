# Issue #179 Locale Currency Mapping 교훈

## 맥락

Issue #179는 `money.CurrencyByLocale`을 작은 manual region map에서 CLDR-backed
current legal-tender lookup으로 확장한다.

## 보존할 결정

- BCP47 parsing과 region validation에는 `golang.org/x/text/language`를 사용한다.
- current tender lookup에는 `golang.org/x/text/currency.Query(currency.Region(r))`를
  사용한다.
- 이 API에는 `currency.FromTag`를 사용하지 않는다. likely region을 추론할 수 있어
  language-only tag가 valid해 보이게 만들기 때문이다.
- 명시적으로 제공된 region을 요구한다. `ko`, `und`, `en-001`은 invalid로 유지한다.
- current tender unit이 없거나 여러 개인 region은 `ErrInvalidCurrency`로 거부한다.
- explicit region을 먼저 추출하고 region이 valid한 뒤에만 `language.ValueError`를
  허용해 `at-AT`의 기존 BCP47-like compatibility를 보존한다.
- no-tender fixture로는 `en-QM` 또는 `en-AQ`를 사용한다. `aa-BB`는 `BB`가 Barbados와
  current `BBD`로 mapping되므로 no-tender fixture가 아니다.

## 검증 Pattern

money package test와 repo-local concurrency stress package를 함께 실행한다.

```bash
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
```

locale lookup behavior를 바꿀 때는 `TestCurrencyByLocaleUsesGoroutineStressTester`를
유지한다. package-level cache나 shared state를 추가하지 않고 concurrent success와
rejection path를 검증한다.

## Diagram Pattern

이 issue의 README/spec diagram은 다음 명령으로 생성한다.

```bash
node scripts/generate-money-locale-currency-diagram.mjs
```

gate는 connector/card problem이 0이고 margin이 균형 잡혔다고 보고해야 한다. accepted
asset의 geometry evidence는 다음과 같았다.

```text
nodes=9 routes=8 segments=10 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 margins=L48/R48/T48/B48 titleGap=58
```

## 알려진 검증 주의점

#179 local validation 중 `go test -count=1 -timeout=2m ./...`는 local
`go1.26.4 darwin/arm64`에서 변경되지 않은 `jwt` cached-provider test에서만 실패했다.
#179 branch가 이후 `jwt`를 건드리지 않는 한 이를 money diff 밖으로 본다.
