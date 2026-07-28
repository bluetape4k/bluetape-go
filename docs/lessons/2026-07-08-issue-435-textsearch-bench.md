# Issue #435 Textsearch Benchmark 교훈

Raw Aho-Corasick 속도만으로 `textsearch` 교체를 정당화할 수는 없다.

- external engine benchmark는 비교 가능한 raw matching behavior에만 맞춘다.
  normalization, original byte span, boundary, replacement, masking, blockword
  processing은 first-party API에서 측정해야 한다.
- production replacement에는 측정된 caller bottleneck과 semantic parity proof가
  모두 필요하다. microbenchmark 승리는 dependency decision이 아니라 evidence로
  남긴다.
- Cloudflare는 steady-state raw matcher 후보로 강하지만 build allocation과
  untagged module version 때문에 adoption에는 주의가 필요하다.
- RRethy는 large-dictionary compile cost 측면에서 매력적이지만 adoption signal이
  작고 match API가 allocation을 만들기 때문에 caller-specific profiling이
  필요하다.
- benchmark output에는 environment metadata를 함께 보존해, 이후 optimization
  작업이 ecosystem research를 반복하지 않고 같은 dictionary와 input shape로
  비교할 수 있게 한다.
