# IAM Access Graph Example

[English](README.md) | [한국어](README.ko.md)

This package ports the IAM access graph from
`bluetape4k-graph/examples/iam-access-graph-examples` into an idiomatic Go
example. It stays backend-neutral by using only `graph` values and
`graph/graphio` records.

![IAM access graph paths](../../../docs/images/readme-diagrams/graph-iam-access-paths.png)

## What It Proves

The seed fixture models authorization review data:

- users `alice`, `bob`, `carol`, and `eve`,
- standing and nested groups such as `engineering` and `platform-admins`,
- direct, group, nested-group, and contract role assignments,
- allow and deny policies that grant permissions to resources,
- a temporary break-glass grant for production read access.

The package answers caller questions that are useful during IAM review:

| Question | Go API |
|---|---|
| Why can a user perform an action? | `ExplainAccess("alice", "staging-service", "deploy")` |
| Which explicit policy denies access? | `ExplainAccess("eve", "prod-db", "delete")` |
| Which nested groups grant admin access? | `RiskyPrivilegeChains("alice")` |
| Which grants exceed the approved least-privilege set? | `ExcessivePermissions("alice", approved)` |

## Seed Data

Seed data lives in `SeedIAMAccessGraph`. It creates 22 vertices and 20 edges for
users, groups, roles, policies, permissions, resources, and a session grant.
Query results return domain IDs such as `user:alice`, `group:engineering`,
`role:prod-admin-role`, and `resource:prod-db`, while graph element IDs remain
stable transport IDs such as `user-alice`.

The same graph can be exported and imported through `graph/graphio` NDJSON with
`WriteNDJSON` and `ReadIAMAccessGraphNDJSON`.

## Test

```bash
go test -count=1 ./examples/graph/iamaccess
go test -race -count=1 ./examples/graph/iamaccess
```

## Production Omissions

This example intentionally omits policy language completeness, directory sync,
identity lifecycle reconciliation, audit evidence retention, entitlement
expiry enforcement, multi-tenant scoping, and backend-specific traversal
performance claims. Those belong in adapter-backed services or follow-up
examples after the graph backend contracts are chosen.
