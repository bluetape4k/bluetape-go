# Issue #436 NLP Memory Benchmark 교훈

Optional NLP adapter benchmark에는 warm steady-state row와 one-shot startup
evidence가 모두 필요하다.

- Kagome IPA dictionary startup은 일반 Go benchmark calibration의 package-level
  warm cache에 가려질 수 있다. first-load cost가 중요하면 isolated one-subcase
  `-benchtime=1x` process snapshot을 유지한다.
- single-process `-bench . -benchtime=1x` 실행은 smoke artifact로 유용하지만,
  benchmark 순서와 cache가 뒤쪽 startup row를 오염시킬 수 있다.
- Lingua high-accuracy first use가 의미 있는 cost center다. 예측 가능한 service
  behavior를 위해 low-accuracy subset과 detector reuse를 기본 권장으로 둔다.
- module cache size와 process RSS snapshot은 deployment signal로 유용하지만 local
  evidence이지 production memory limit은 아니다.
- Kagome와 Lingua는 `textsearch/japanese`, `textsearch/language` 뒤에 둔다. core
  `textsearch` package가 optional dependency를 갖지 않는지 검증한다.
