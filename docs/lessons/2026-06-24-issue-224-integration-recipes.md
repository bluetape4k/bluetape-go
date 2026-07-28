# Issue #224 Integration Recipe 교훈

Issue: #224
날짜: 2026-06-24

- caller에게 새 reusable API가 필요한 경우가 아니라면 cross-package example은
  `examples/` 아래에 둔다. 이렇게 하면 helper contract를 고정하지 않고 recipe code를
  copyable하게 유지할 수 있다.
- Docker-backed recipe는 env-gated로 둬서 일반 `go test ./...`가 local하고
  deterministic하게 유지되게 한다.
- integration doc에서는 failure path를 code로 증명한다. retry/skip recipe가
  happy-path-only snippet보다 유용하다.
- public example package를 추가할 때 English/Korean root README link를 동기화한다.
