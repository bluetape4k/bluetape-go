# Issue #598 Fory Redis Value Cache Spec Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Design: `docs/superpowers/specs/2026-07-10-issue-598-fory-redis-value-cache-design.md`
- Gate: Step 2-R, six independent perspectives plus main-session integration
- Blocking threshold: P0/P1 must be zero after amendments and targeted re-review

## 독립 검토

| Perspective | Initial P0 | Initial P1 | Notable non-blocking findings | Resolution |
|---|---:|---:|---|---|
| Performance | 0 | 0 | mutex contention evidence; benchmark provenance | Added #599 contention rows and machine/revision metadata |
| Stability | 0 | 0 | all-method context preflight; typed-nil client; post-encode cancellation | Added constructor validation and explicit context dispatch boundaries |
| Security | 0 | 2 | bounded defaults/profile tuple; Redis-visible logical keys | Added exact defaults, payload cap, complete Fory options, key secrecy warning; re-review P0=0 P1=0 |
| Operator/Ops | 0 | 0 | Redis Cluster slot behavior; bounded old-generation cleanup; observability | Added single-key/hash-tag contract, SCAN cleanup recipe, low-cardinality telemetry guidance |
| Developer/API | 0 | 2 | inspectable typed error; undefined public registration boundary | Added public `Registration`, `Profile`, `Reason`, and `CacheError` accessors; re-review P0=0 P1=0 |
| User/Caller | 0 | 0 | Delete key validation; exact defaults/root shapes; nil context | Added all-method validation, exact defaults, and supported/rejected root contract |

## 메인 세션 통합

The integration review found one additional cancellation race: cancellation can occur during
Fory serialization or envelope allocation after the initial context check. The amended `Set`
flow requires a second `ctx.Err()` check immediately before Redis `SET`, and the test contract
requires proof that this path leaves no Redis write.

The amended spec also fixes the following cross-perspective contracts:

- `BTFV` total bound is `14 + MaxPayloadBytes`; payload length is capped at `math.MaxUint32`.
- `WithXlang(false)`, profile-specific compatibility, and `WithTrackRef(false)` form the complete
  Fory profile identity.
- Nil and typed-nil clients, nil registration, invalid keys, invalid generation, and invalid
  resource limits fail before Redis I/O.
- Redis Cluster hash tags remain caller-controlled; the package does not inject a tag.
- Old-generation cleanup uses bounded `SCAN` plus batched `UNLINK`/`DEL`, never `KEYS`.
- Benchmark issue #599 owns result tables, charts, analysis, provenance, and contention evidence.

## 최종 판정

PASS. Targeted security and developer/API re-reviews confirmed P0=0 and P1=0.
All six perspectives and the main-session integration review are closed for Step 2-R.
