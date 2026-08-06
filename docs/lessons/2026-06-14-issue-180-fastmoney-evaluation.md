# Money API를 복제하기 전에 benchmark한다

## 맥락

Issue #180은 JVM money helper surface와 유사한 public long-backed `FastMoney` type을
bluetape-go에 추가해야 하는지 평가했다.

기존 Go package에는 이미 decimal-backed immutable `Money` type과 minor-unit 입력/출력
helper가 있었다.

- `NewMinor`
- `MinorUnits`

## 교훈

performance를 이유로 public API를 복제하려면 두 가지 evidence가 필요하다.

- 기존 API가 충분하지 않다는 measured hot-path evidence
- 기존 type이 필요한 boundary를 표현할 수 없다는 caller contract

#180의 benchmark는 현재 minor-unit 및 arithmetic path가 direct `govalues/money`
reference에 이미 가깝고 operation당 allocation이 0임을 보여줬다. 별도 public
`FastMoney` type은 충분한 evidence 없이 construction, arithmetic, parsing,
serialization, exchange-rate, README, example, error contract를 중복시킨다.

## 다음에 적용할 규칙

- performance-shaped public type을 추가하기 전에 현재 wrapper cost를 측정한다.
- table이나 chart를 만들기 전에 raw benchmark output을 보존한다.
- benchmark-heavy decision에는 실제 chart를 사용해 reviewer가 숫자만 읽지 않고도
  scale과 direction을 볼 수 있게 한다.
- direct dependency benchmark row는 public API recommendation이 아니라 reference
  data로 유지한다.
- benchmark가 threshold를 넘으면 research issue 안에서 scope를 넓히지 말고 public
  API expansion follow-up issue를 요구한다.

## 증거

- Raw output: `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`
- Chart: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`
- Decision note: `docs/research/2026-06-14-issue-180-fastmoney-evaluation.md`
