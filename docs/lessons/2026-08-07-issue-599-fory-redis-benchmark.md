# Issue #599 Fory/Redis benchmark lesson

## 결정

Fory profile 비교는 codec 단독, direct Redis value cache, complete
stampede coordination을 분리해 측정해야 한다. 하나의 round-trip 숫자로 합치면
serialization cost와 Redis lock/result/polling cost를 구분할 수 없다.

## 재사용할 패턴

- Go benchmark output은 `-count=3` raw text를 보존하고, benchmark 이름을
  profile/scenario 단위로 정규화한 parser summary를 함께 저장한다.
- `wire-bytes`는 계층을 명시한다. direct cache는 실제 Redis envelope 길이를
  읽고, coordination은 inner codec payload 길이로 기록한다.
- `NativeFast`와 `NativeCompatible`는 schema/metadata profile이므로 동일한
  serialization mode로 취급하지 않는다. compatibility는 별도 schema evolution
  test로 확인한다.
- Testcontainers Redis benchmark는 top-level workload를 직렬 실행하고 image
  digest, Go/Fory version, host architecture를 raw header에 기록한다.
- shared mutex와 codec pool은 benchmark-only contention 가설로 비교하며,
  pool 결과만으로 production default/API를 변경하지 않는다.

## 근거

- Raw: `docs/research/outputs/issue-599/benchmark.txt`
- Parsed: `docs/research/outputs/issue-599/summary.json`
- Report: `docs/benchmarks/2026-08-07-issue-599-fory-redis.md`
