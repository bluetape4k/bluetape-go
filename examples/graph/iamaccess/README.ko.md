# IAM Access Graph 예제

[English](README.md) | [한국어](README.ko.md)

이 package는 `bluetape4k-graph/examples/iam-access-graph-examples`의 IAM access
graph를 Go다운 예제로 옮깁니다. Backend adapter에 의존하지 않고 `graph` value와
`graph/graphio` record만 사용합니다.

![IAM access graph paths](../../../docs/images/readme-diagrams/graph-iam-access-paths.png)

## 증명하는 것

Seed fixture는 authorization review에 필요한 데이터를 모델링합니다.

- `alice`, `bob`, `carol`, `eve` user,
- `engineering`, `platform-admins` 같은 standing/nested group,
- direct, group, nested-group, contract role assignment,
- resource permission을 부여하거나 차단하는 allow/deny policy,
- production read access를 위한 temporary break-glass grant.

Package는 IAM review에서 실제로 필요한 caller 질문에 답합니다.

| 질문 | Go API |
|---|---|
| 사용자가 어떤 action을 수행할 수 있는 이유는 무엇인가? | `ExplainAccess("alice", "staging-service", "deploy")` |
| 어떤 explicit policy가 access를 차단하는가? | `ExplainAccess("eve", "prod-db", "delete")` |
| 어떤 nested group이 admin access를 부여하는가? | `RiskyPrivilegeChains("alice")` |
| 승인된 least-privilege set을 초과하는 grant는 무엇인가? | `ExcessivePermissions("alice", approved)` |

## Seed Data

Seed data는 `SeedIAMAccessGraph`에 있습니다. User, group, role, policy,
permission, resource, session grant를 나타내는 22개 vertex와 20개 edge를
생성합니다. Query 결과는 `user:alice`, `group:engineering`,
`role:prod-admin-role`, `resource:prod-db` 같은 domain ID를 반환하고, graph
element ID는 `user-alice` 같은 안정적인 transport ID로 유지합니다.

같은 graph는 `WriteNDJSON`과 `ReadIAMAccessGraphNDJSON`을 통해 `graph/graphio`
NDJSON으로 export/import할 수 있습니다.

## Test

```bash
go test -count=1 ./examples/graph/iamaccess
go test -race -count=1 ./examples/graph/iamaccess
```

## Production Omission

이 예제는 policy language completeness, directory sync, identity lifecycle
reconciliation, audit evidence retention, entitlement expiry enforcement,
multi-tenant scoping, backend-specific traversal 성능 주장을 의도적으로
제외합니다. 이런 범위는 graph backend contract를 고른 뒤 adapter-backed service나
follow-up example에서 다룹니다.
