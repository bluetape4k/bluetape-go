# Issue #206 Range와 Collection Primitive 교훈

## 변경된 점

- open/closed boundary constructor, containment, overlap check, zero-value empty
  behavior, NaN-safe membership을 갖춘 Go-native `core.Range`를 추가했다.
- `iter.Seq`를 사용하는 `collections.BoundedStack`, `RingBuffer`, `Page`, lazy
  `Permutations`를 추가했다.
- English/Korean README와 compile-tested example을 갱신했다.

## 교훈

- `cmp.Ordered`는 float를 포함하므로 constructor에서 NaN을 거부하는 것만으로는
  부족하다. NaN과의 일반 비교는 모두 false이므로 membership check도 NaN 값을 거부해야
  한다.
- pagination helper는 `page + 1`, `total + size - 1` 같은 arithmetic을 피한다. zero
  page를 guard한 뒤 `totalPages - 1`과 비교하고, total page는 division과 remainder로
  계산한다.
- contract가 이후 caller mutation이 iteration에 영향을 줄 수 없다고 말한다면
  `iter.Seq` API는 function-call 시점에 caller input을 copy해야 한다. 반환된 closure
  안에서 copy하면 너무 늦다.
- 값을 버리는 fixed-capacity container는 reslice 전에 버려지는 backing slot을 clear해
  reference가 필요 이상 오래 유지되지 않게 한다.
- native subagent review는 manager lifecycle layer에서 실패할 수 있다. 이 경우 gate를
  약화하지 말고 unavailability를 기록한 뒤 main session에서 같은 7-tier review shape를
  수행한다.

## 검증

- `go test -count=1 ./core ./collections`
- `go test -race -count=1 ./core ./collections`
- `go test ./...`
- `git diff --check`
- `make ci`
