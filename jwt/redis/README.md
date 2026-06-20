# jwt/redis

English | [한국어](README.ko.md)

`jwt/redis` is the Redis-specific import boundary for distributed JWT key-chain
storage. It exposes aliases for `jwt.RedisRepositoryOptions` and
`jwt.RedisRepository`, then delegates construction to the parent `jwt` package.

![jwt redis facade map](../../docs/images/readme-diagrams/jwt-redis-facade-map.png)

## Import

```go
import redisjwt "github.com/bluetape4k/bluetape-go/jwt/redis"
```

## Usage

```go
repo, err := redisjwt.New(redisjwt.Options{
    Client:    redisClient,
    Namespace: "service-auth",
})
if err != nil {
    return err
}

provider, err := jwt.NewDistributedHMACProvider(ctx, repo, jwt.HS256)
if err != nil {
    return err
}
token, err := provider.ComposeContext(ctx, jwt.WithSubject("account-42"))
```

## Behavior

- `Options` aliases `jwt.RedisRepositoryOptions`.
- `Repository` aliases `jwt.RedisRepository`.
- `New` validates Redis repository options through the parent package and
  returns the distributed key-chain repository used by distributed JWT
  providers.
- Signing, parsing, key rotation, retention, and provider cache behavior remain
  owned by `jwt`.

## Operational Boundaries

- Use this package when callers want a Redis-specific import path without
  depending on repository implementation names in the parent package.
- Redis stores signing key material and current `kid` metadata. Configure Redis
  authentication, TLS, ACLs, persistence, and backup policy outside this helper.
- Reset and retention operations are explicit parent-package repository
  operations; keep them behind administrative authorization.

## Test

```bash
go test -count=1 ./jwt/redis
go test -count=1 ./jwt
```
