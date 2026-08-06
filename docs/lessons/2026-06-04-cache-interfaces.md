# Cache Interfaces Lessons (2026-06-04)

**Related issue**: #22
**Affected package**: `cache`

## L1: generic cache key에는 명시적 flight-key namespace가 필요하다

### 문제

`singleflight.Group`은 string key를 받지만 public cache API는 `K comparable`을
사용한다. `fmt.Sprint`로 key를 변환하면 문자열 표현이 같은 서로 다른 값이
우연히 합쳐질 수 있다.

### 교훈

generic API가 string-keyed coordination primitive에 위임할 때는 package-owned
collision-free namespace를 설계하고, `String` output이 같은 key로 test한다.

## L2: cache-aside loader에는 delete/clear ordering 문서가 필요하다

### 문제

`Delete`와 `Clear`는 concurrent-safe method지만 `GetOrLoad`로 이미 시작된 loader를
cancel하지 않는다. ordering contract가 없으면 caller는 delete/clear가 이후
in-flight write를 영구적으로 막는다고 오해할 수 있다.

### 교훈

cache-aside API는 mutation method가 in-flight loader를 cancel하는지, 아니면 race만
하는지 문서화한다. test는 data-race safety를 증명하고 README/package docs는
caller-visible ordering을 설명해야 한다.

## L3: exported concrete type은 zero-value safe이거나 숨긴다

### 문제

첫 `Memory[K,V]` 구현은 concrete type을 export했지만 map 초기화는 `NewMemory`에서만
했다. caller가 `var c cache.Memory[...]`처럼 사용하면 `Set`에서 panic이 날 수 있다.

### 교훈

Go package가 concrete type을 export하면 zero value를 usable하게 만들거나 type을
노출하지 않는다. PR review를 닫기 전에 zero-value test를 추가한다.
