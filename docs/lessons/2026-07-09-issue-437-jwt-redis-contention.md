# Issue #437 JWT Redis Contention Benchmark Lesson

JWT Redis/provider contention benchmarks should keep local and Docker-backed
paths separate.

- Default `go test -bench . ./jwt` should stay local and avoid starting Docker.
- Redis/Testcontainers benchmark rows need an explicit opt-in env flag and
  serial command, because each row owns external Redis setup and cleanup.
- Contention benchmark artifacts must not include raw tokens, HMAC secrets, RSA
  private keys, or serialized key payloads.
- A measured contention row is evidence, not an automatic optimization mandate;
  open follow-up implementation only when the benchmark exposes a specific API,
  allocation, or latency problem.

