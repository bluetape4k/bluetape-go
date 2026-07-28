# Issue #310 Libvips evaluation 교훈 (2026-07-02)

**관련 PR**: #374
**영향 모듈**: `examples/imagekit-govips`, `imagekit` docs/research

## L1: Native image adapter는 CI가 소유하기 전까지 default Go module 밖에 둔다

### 문제

libvips-backed adapter에는 cgo, `pkg-config`, native library installation,
host-specific codec availability가 필요하다. govips를 root module이나 default `imagekit`
package에 직접 추가하면 작은 pure-Go thumbnail만 필요한 일반 호출자도 native dependency와
CI complexity를 부담하게 된다.

### 교훈

bluetape-go native image experiment는 격리된 nested example module에서 시작한다. Native
CI, codec support policy, release packaging을 명시적으로 수용한 뒤에만 root package로
승격한다.

## L2: govips lifecycle은 process-owned여야 한다

### 문제

`govips`는 libvips를 명시적으로 시작할 수 있지만, source는 shutdown 뒤 restart를 거부한다.
Request-scoped start/stop helper는 안전하지 않고 테스트하기도 어렵다.

### 교훈

Process-level `sync.Once` startup을 사용하고, 일반 request path에서는 shutdown을 호출하지
않으며, export 뒤에는 모든 `ImageRef`를 닫는다. Context cancellation은 native work 전후로
확인할 수 있지만 이미 진행 중인 libvips call을 선점할 수 없다는 점을 문서화한다.
