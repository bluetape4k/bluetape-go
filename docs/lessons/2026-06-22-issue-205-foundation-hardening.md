# Lessons Learned — Issue #205 Foundation Contract Hardening (2026-06-22)

**Related PR**: TBD
**Affected modules**: `core`, `collections`, `codec`, `serialization`

## L1: Text contracts need a caller-visible sentinel plus negative codec tests

### Problem

`Decode*String`, `StringSerializer`, and `TruncateUTF8Bytes` all described text
behavior, but invalid decoded bytes could still reach callers as Go strings.
Malformed codec input and invalid UTF-8 payloads are different failure classes,
so tests need to prove they do not collapse into one generic error.

### Lesson

For text helpers with error returns, add an exported sentinel and assert it with
`errors.Is`. Also add negative tests proving malformed codec input does not wrap
the text sentinel, while byte-oriented APIs continue accepting arbitrary binary
payloads.

### Evidence

- `core.ErrInvalidUTF8`
- `TestStringDecodersRejectInvalidUTF8`
- `TestMalformedStringDecodersDoNotUseInvalidUTF8Sentinel`
- `TestByteDecodersAcceptArbitraryBinary`
- `TestStringSerializerRejectsInvalidUTF8`

## L2: 7-tier review lanes need a bounded fallback record

### Problem

Step 3-R native review lanes produced useful findings, but agent cleanup/wait
stalled for too long. The work should keep the six review perspectives while
avoiding unbounded waits.

### Lesson

When a review lane exceeds the workflow SLA, close or stop waiting on it, record
the lane fallback explicitly, and complete that perspective in the main session.
Do not let subagent lifecycle management become the critical path after enough
P0/P1 evidence has been collected.

### Evidence

- `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`
- `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`
- `make ci`
