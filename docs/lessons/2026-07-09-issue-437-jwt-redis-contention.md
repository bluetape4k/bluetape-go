# Issue #437 JWT Redis Contention Benchmark 교훈

JWT Redis/provider contention benchmark는 local path와 Docker-backed path를 분리해야
한다.

- 기본 `go test -bench . ./jwt`는 local로 유지하고 Docker를 시작하지 않는다.
- Redis/Testcontainers benchmark row는 각 row가 external Redis setup과 cleanup을
  소유하므로 명시적 opt-in env flag와 serial command가 필요하다.
- Contention benchmark artifact에는 raw token, HMAC secret, RSA private key,
  serialized key payload를 포함하지 않는다.
- 측정된 contention row는 evidence이지 자동 optimization mandate가 아니다.
  benchmark가 특정 API, allocation, latency 문제를 드러낼 때만 follow-up
  implementation을 연다.
