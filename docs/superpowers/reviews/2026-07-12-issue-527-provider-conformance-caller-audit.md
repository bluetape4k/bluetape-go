# Issue #527 Caller Migration And Locale Audit

## Scope And Commands

The first-party Go tree was searched after implementation with:

```bash
rg -n 'Campaign\((context\.)?(Background|TODO)\(' --glob '*.go'
rg -n 'errors\.Is\([^,]+,\s*(leader\.)?ErrNotLeader|ErrNotLeader' --glob '*.go'
rg -n '\.(Campaign|Resign|Leader)\(nil\)' --glob '*.go'
rg -n 'TryLock\(' --glob '*.go'
rg -n 'Token:\s*"[^"]*"' lock --glob '*.go'
```

## Internal Caller Disposition

| Search | Hit class | Disposition | Owner/status |
|---|---|---|---|
| blocking `Campaign(Background/TODO)` | `leader/leadertest` reference tests and runner | Kept only for empty-state acquisition, duplicate local-state checks, injected lost-response, and deterministic cleanup; contention paths use bounded contexts. Unique test identities prevent a live external owner. | conformance helper owner / complete |
| legacy `ErrNotLeader` | Redis/Mongo strategic elector implementation/tests and coordination recipe authorization checks | Kept. These are strategic update/authorization sentinels, not single-elector `Campaign` contention. Redis single-elector contention tests/examples were migrated to caller deadlines and `DeadlineExceeded`. | leader provider owner / complete |
| nil leader contexts | `leader/leadertest` negative contract case | Kept as an explicit no-dispatch assertion for `ErrInvalidContext`; no production caller passes nil. | conformance helper owner / complete |
| `TryLock` value-plus-error | `cache/rediscoord`, integration recipe, Redis lock examples/tests, conformance adapter | `cache/rediscoord` now cleans up every non-nil lease before propagating an error. The integration and package examples perform type-first `ErrCommitUnknown` handling and bounded cleanup. Tests cover the non-nil lease/error tuple, same-callback retry, and replacement-owner protection. | lock/cache owners / complete |
| custom literal token | Redis lock validation and owner tests | Blank-only tokens remain rejected. Valid custom bytes, including surrounding whitespace and Unicode, are preserved byte-for-byte; the old trim expectation was migrated. | lock provider owner / complete |

No internal production single-elector caller required a blocking-context migration beyond the Redis coordination test/example already changed in this branch.

## External Caller Ownership

External repositories were not claimed as scanned. The 0.19.0 `CHANGELOG.md` and bilingual package migration sections are the release contract. Release maintainers own downstream notification and must verify:

- single-elector callers supply bounded cancellation and no longer expect `ErrNotLeader` from contention;
- leader/lock callers check typed commit-unknown errors before bare context errors and perform bounded cleanup/TTL fallback;
- lock callers clean up whenever a lease is non-nil, even when error is also non-nil;
- custom lock token consumers that depended on trimming migrate explicitly;
- Redis rate-limit callers never replay commit-unknown requests and budget for one possible debit.

Status: documented for the 0.19.0 release; downstream repository execution remains owned by each consumer maintainer.

## English/Korean Section Mapping

Automated heading-count parity is supplemented by this manual contract mapping.

| English/Korean pair | Mapped contract section | Required recovery content |
|---|---|---|
| `leader/leadertest/README*` | Contract / 계약; Usage / 사용법; Commit-Unknown Recovery / Commit-Unknown 복구; Diagnostics / 진단; Test / 테스트 | typed `OperationError`, `ErrCommitUnknown`, bounded `Resign`, TTL, redacted runner output |
| `lock/locktest/README*` | Contract / 계약; Usage / 사용법; Commit-Unknown Recovery / Commit-Unknown 복구; Diagnostics / 진단; Test / 테스트 | non-nil callback with error, same callback retry, owner comparison, TTL, bounded/redacted diagnostics |
| `ratelimit/ratelimittest/README*` | Contract / 계약; Usage / 사용법; Commit-Unknown Recovery / Commit-Unknown 복구; Diagnostics / 진단; Test / 테스트 | zero result, one possible debit, no replay, full-refill wait, bounded/redacted diagnostics |
| `leader/README*` | Single-Elector Conformance | blocking campaign, distinct sentinels, type-first errors, bounded `Resign`/TTL |
| `leader/redis/README*` | Conformance And Recovery / Conformance 및 복구 | dual commit-unknown sentinels, owner reconciliation, command-rate canary, bounded `Resign`/TTL |
| `leader/mongo/README*` | Conformance And Recovery / Conformance 및 복구 | typed leader error, commit-unknown, same-elector bounded `Resign`, unchanged BSON schema |
| `lock/redis/README*` | Conformance And Commit-Unknown Recovery / Conformance 및 Commit-Unknown 복구 | `lease != nil`, type-first handling, same lease callback retry, replacement protection, TTL, no token trim |
| `ratelimit/README*` | Provider Conformance | typed error first, no replay, full-refill/caller-budget recovery |
| `ratelimit/redis/README*` | Conformance And Commit-Unknown Recovery / Conformance 및 Commit-Unknown 복구 | zero result, typed Redis sentinel, one debit, no replay, full-refill wait |

The leader, lock, and rate-limit recovery snippets are present in both languages and carry the same operational action rather than only the same symbol names.
