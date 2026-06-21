# Issue #179 Locale Currency Mapping Step 2-R Spec Review

## Scope

- Spec: `docs/superpowers/specs/2026-06-14-issue-179-locale-currency-mapping-design.md`
- Diagram generator: `scripts/generate-money-locale-currency-diagram.mjs`
- Diagram assets: `docs/images/readme-diagrams/money-locale-currency-resolution-flow.*`
- Source baseline: `money/currency.go`, `money/currency_test.go`, `money/README.md`, `money/README.ko.md`

Native subagents were not used for this gate because this session has repeatedly shown unreliable subagent waits. Per the standing operating rule, the main session performed the six independent review lenses and then integrated the result.

## Review Lanes

| Lane | Perspective | Verdict | Findings |
|---|---|---:|---|
| 1 | Performance | PASS | `currency.Query(currency.Region(r))` is local CLDR table lookup, not IO. The plan should avoid per-call allocations beyond a small slice/count. |
| 2 | Stability | PASS | Spec requires explicit region only and rejects zero/multi tender counts, preventing unstable likely-region guesses. |
| 3 | Security | PASS | Locale tags are parsed as data with no IO, file, shell, or network boundary. Error wrapping keeps sentinel checks stable. |
| 4 | Operator/Ops | PASS | No service dependency or runtime configuration is introduced. Docs must state CLDR source/update strategy. |
| 5 | Developer/API | PASS | Public API remains `CurrencyByLocale(tag string) (Currency, error)`; backward compatibility cases are preserved. |
| 6 | User/Caller | PASS | Ambiguous/no-tender behavior is explicit; README caveat prevents legal/accounting overclaim. |
| Main | Integration | PASS | The chosen design satisfies issue #179 without a new public type or custom CLDR snapshot. |

## P0/P1 Gate

- P0: 0
- P1: 0

## P2/P3 Notes

- P2: `golang.org/x/text` is currently indirect. Implementation must make imports explicit and run `go mod tidy`.
- P3: The diagram is useful for spec and README context, but README placement should be near the locale mapping behavior text, not above exchange-rate provider details.

## Evidence

```bash
node scripts/generate-money-locale-currency-diagram.mjs
xmllint --noout docs/images/readme-diagrams/money-locale-currency-resolution-flow.svg
git diff --check
```

Generator geometry summary:

```text
money-locale-currency-resolution-flow: nodes=9 routes=8 segments=10 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 margins=L48/R48/T48/B48 titleGap=58
```

Rendered PNG was inspected after label adjustment. No text overflow, card overlap, connector-card intersection, or excessive bottom whitespace remained.

## Verdict

PASS. The spec is implementation-ready.

`P0=0 P1=0`
