# Issue #205 Foundation Contract Hardening 교훈 (2026-06-22)

**Related PR**: TBD
**Affected modules**: `core`, `collections`, `codec`, `serialization`

## L1: Text contract에는 caller-visible sentinel과 negative codec test가 필요하다

### 문제

`Decode*String`, `StringSerializer`, `TruncateUTF8Bytes`는 모두 text behavior를
설명했지만, invalid decoded byte가 여전히 Go string으로 caller에게 도달할 수 있었다.
malformed codec input과 invalid UTF-8 payload는 서로 다른 failure class이므로, test는
둘이 하나의 generic error로 합쳐지지 않음을 증명해야 한다.

### 교훈

error를 반환하는 text helper에는 exported sentinel을 추가하고 `errors.Is`로 assert한다.
또한 malformed codec input이 text sentinel을 wrap하지 않으며, byte-oriented API는
arbitrary binary payload를 계속 허용한다는 negative test를 추가한다.

### 증거

- `core.ErrInvalidUTF8`
- `TestStringDecodersRejectInvalidUTF8`
- `TestMalformedStringDecodersDoNotUseInvalidUTF8Sentinel`
- `TestByteDecodersAcceptArbitraryBinary`
- `TestStringSerializerRejectsInvalidUTF8`

## L2: 7-tier review lane에는 bounded fallback record가 필요하다

### 문제

Step 3-R native review lane은 유용한 finding을 냈지만 agent cleanup/wait가 너무 오래
멈췄다. 작업은 여섯 review perspective를 유지하되 unbounded wait는 피해야 한다.

### 교훈

review lane이 workflow SLA를 넘으면 lane을 닫거나 더 기다리지 않고, lane fallback을
명시적으로 기록한 뒤 main session에서 해당 perspective를 완료한다. 충분한 P0/P1
evidence를 수집한 뒤에는 subagent lifecycle management가 critical path가 되게 하지
않는다.

### 증거

- `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`
- `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`
- `make ci`
