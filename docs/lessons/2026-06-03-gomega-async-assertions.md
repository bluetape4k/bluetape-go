# Gomega 비동기 Assertion

Java Awaitility 스타일의 비동기 assertion은 Go에서 Gomega와 잘 맞는다.
`testing` package의 public helper는 작게 유지하고, polling, timeout, failure
reporting은 `gomega.NewWithT(t)`에 위임한다.

상태가 결국 true가 되어야 하면 `Eventually`를 사용하고, 일정 시간 동안 true로
유지되어야 하면 `Consistently`를 사용한다. 기본 25ms polling interval이 너무
느리거나 noisy할 때만 interval을 명시한다.
