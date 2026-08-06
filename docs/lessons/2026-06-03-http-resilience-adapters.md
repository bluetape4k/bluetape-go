# HTTP Resilience Adapters

HTTP retry example은 일반 service-call retry보다 엄격한 contract가 필요하다.
`net/http` request body는 항상 replay 가능하지 않고, retry 가능한 response body는
다음 attempt 전에 닫아야 connection leak이 생기지 않는다.

bluetape-go HTTP adapter의 기준은 다음과 같다.

- adapter는 `net/http`에 가깝게 유지하고 framework dependency를 추가하지 않는다.
- retry와 circuit breaker policy가 failure를 관찰할 수 있도록 retryable response
  status를 policy composition 전에 명시적 error로 변환한다.
- retry 전에 retryable response body를 닫는다.
- retry될 수 있는 request body에는 `Request.GetBody`를 요구한다.
- retry는 outbound client call에 우선 적용한다. Server handler는 admission과
  timeout policy를 사용할 수 있지만, response를 쓴 뒤 handler를 retry하는 것은
  안전하지 않다.
