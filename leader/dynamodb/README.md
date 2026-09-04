# DynamoDB leader

[한국어](README.ko.md)

`github.com/bluetape4k/bluetape-go/leader/dynamodb` provides a single-item
leader lease backed by caller-owned DynamoDB. It implements `leader.Elector`
with conditional `PutItem`/`UpdateItem`/`DeleteItem` operations and strongly
consistent `GetItem` reads.

## Usage

```go
elector, err := dynamodbleader.New(
    client, // caller-owned *dynamodb.Client or the narrow Client interface
    "worker-leases",
    leader.Options{
        Group: "billing-workers", MemberID: "worker-1",
        Lease: 30 * time.Second, RenewInterval: 10 * time.Second,
    },
)
if err != nil { return err }
if err := elector.Campaign(ctx); err != nil { return err }
defer elector.Resign(cleanupCtx)
```

`Client` contains only `PutItem`, `UpdateItem`, `DeleteItem`, and `GetItem`.
The caller creates and closes the AWS client, supplies credentials/region,
chooses retry and timeout policy, and provisions the table/IAM policy. The
package never creates a client or runs migrations.

The inherited `leader.Options.KeyPrefix` is validated for provider compatibility
but is not encoded into the DynamoDB item key. The table is this provider's
namespace boundary: do not share one table across different key-prefix policies
unless `Group` values are globally unique. Table-per-namespace or prefixing the
group at the caller boundary remains an operator decision.

## Item schema

The default attributes are:

| Attribute | Type | Meaning |
|---|---|---|
| `group` | `S` | partition key and `leader.Options.Group` |
| `owner_token` | `S` | opaque member token for this elector instance |
| `lease_until_ms` | `N` | absolute epoch milliseconds used for correctness |
| `expires_at` | `N` | epoch seconds for DynamoDB TTL cleanup only |

Use `WithAttributeNames` when the table uses another schema. Names are aliased
in expressions and are not trimmed or silently renamed. Lease values use the
injected clock (`WithClock`, default `time.Now`); `RenewInterval` must be less
than `Lease`, and both are at least one millisecond. TTL is rounded up to the
next second so asynchronous cleanup cannot make an active lease look expired.

## Lifecycle and consistency

Campaign first tries `attribute_not_exists(#key)`. A conditional failure then
tries an expired-owner update with `attribute_not_exists(#lease) OR
#lease <= :now`. Only that condition can replace a stale owner. Renewal checks
the owner token and `lease_until_ms > :now`; a conditional failure makes
`IsLeader` false without deleting another owner.

`Leader` always requests `ConsistentRead=true` and compares the deadline to the
injected clock. Missing or expired items return `"", nil`. A malformed active
item returns `ErrMalformedItem`. DynamoDB TTL deletion is asynchronous and is
never used as a correctness signal. Provisioned/on-demand capacity, hot-key
behavior, and throttling remain caller/operator concerns.

## Errors and cleanup

Provider failures are `leader.OperationError` values with safe operation labels;
AWS messages, table names, groups, and owner tokens do not appear in `Error()`
or `%+v`. Use `errors.Is`/`errors.As` for the cause and
`leader.ErrCommitUnknown`.

If a write response is lost, or the bounded attempt context expires before a
late response arrives, the provider performs one bounded strongly consistent
probe and never trusts the late response alone. An own active token confirms the
commit; an empty or another owner permits a fresh campaign attempt; a failed
probe returns `leader.ErrCommitUnknown` and leaves cleanup pending. Takeover
deadlines are recomputed immediately before the conditional update. Retry
`Resign` on the same elector with a fresh cleanup context; its probe is bounded
by the same attempt budget. A conditional delete failure means the item is
already gone or replaced and is idempotent success.

The provider logs lifecycle/failure events only through the caller-selected
`log/slog` logger (`WithLogger`); no global logger configuration is changed and
raw provider text is never logged.

## IAM and live testing

The table role needs the actions `dynamodb:GetItem`, `dynamodb:PutItem`,
`dynamodb:UpdateItem`, and `dynamodb:DeleteItem` on the caller-selected table.
Least-privilege resource policy, encryption, network, and credential rotation
are outside this package. Normal CI uses a deep-copying fake and does not need
AWS credentials. Floci or live AWS tests are explicit opt-in only.

See the compile-checked `ExampleNew` for a fake-only construction.
