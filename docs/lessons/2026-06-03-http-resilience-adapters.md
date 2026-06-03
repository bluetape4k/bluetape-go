# HTTP resilience adapters

HTTP retry examples need stricter contracts than plain service-call retry.
`net/http` request bodies are not always replayable, and retryable response
bodies must be closed before the next attempt so clients do not leak
connections.

For bluetape-go HTTP adapters:

- Keep the adapter close to `net/http`; no framework dependency is needed.
- Convert retryable response statuses to an explicit error before policy
  composition so retry and circuit breaker policies can observe failures.
- Close retryable response bodies before retrying.
- Require `Request.GetBody` for request bodies that may be retried.
- Prefer retry on outbound client calls. Server handlers can use admission and
  timeout policies, but retrying a handler after it writes a response is unsafe.
