# Distributed JWT Redis Key Rotation 교훈 (2026-06-13)

**Related issue**: #173
**Related PR**: #226
**Affected modules**: `jwt`, `jwt/redis`

## L1: Distributed JWT storage는 signing authority boundary다

### 문제

이 feature에서 Redis는 JWT metadata만 cache하지 않는다. active 및 retained signing
KeyChain을 저장하므로, operator 실수는 원래 valid한 token을 invalid하게 만들거나
약한 infrastructure control을 통해 signing material을 노출할 수 있다.

### 교훈

distributed JWT repository를 추가할 때마다 Redis를 trusted signing-key boundary로
문서화한다. README/runbook evidence는 TLS, ACL, namespace isolation, persistence,
eviction policy, diagnostics, reset limit, 명시적 token invalidation decision을
포함해야 한다.

### 증거

- `jwt/README.md`와 `jwt/README.ko.md`는 Redis trust-boundary와 runbook command를
  문서화한다.
- Redis provider와 repository 구현 뒤 `go test -race -p 1 -count=1 ./jwt ./jwt/redis`
  가 통과했다.

## L2: Benchmark evidence에는 chart와 diagram context가 필요하다

### 문제

raw benchmark number와 Markdown table만 있으면 reviewer가 개별 값을 손으로 비교해야
한다. 또한 측정 operation을 만드는 Redis key-rotation path도 설명하지 못한다.

### 교훈

public docs 또는 PR evidence에 benchmark data를 포함할 때는 실제 chart asset과
source-grounded diagram을 함께 넣는다. README file은 PNG asset을 embed하고, SVG와
Graphviz evidence는 generated image 옆에 보관한다.

### 증거

- `docs/images/readme-charts/distributed-jwt-redis-benchmark.png`
- `docs/images/readme-diagrams/redis-jwt-distributed-key-rotation.png`
- diagram generator output은 Redis JWT key-rotation diagram에 대해
  `nodes=8 routes=9 segments=45`를 보고했다.

## L3: P2/P3 review finding은 gate를 무기한 확장하지 않아야 한다

### 문제

non-blocking finding이 full re-review cycle을 유발하면 Step 6-R이 느려질 수 있다.
이는 실제 gate condition을 가리고 PR evidence를 지연시킨다.

### 교훈

progression gate는 `P0=0 P1=0`으로 유지한다. P2/P3 finding은 local하고 risk를
줄이는 경우에만 gate 중에 수정한다. 그렇지 않으면 명확한 근거와 함께 follow-up
work로 기록한다.

### 증거

- Step 6-R은 `P0=0 P1=0 P2=0 P3=1`로 종료됐다.
- 남은 P3는 parallel Redis/provider contention benchmark coverage다. #173의
  correctness blocker는 아니지만 유용한 follow-up evidence다.
