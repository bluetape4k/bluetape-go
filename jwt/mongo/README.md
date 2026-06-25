# jwt/mongo

English | [한국어](README.ko.md)

`jwt/mongo` is the MongoDB-specific import boundary for distributed JWT
key-chain storage. It exposes aliases for `jwt.MongoRepositoryOptions` and
`jwt.MongoRepository`, then delegates construction to the parent `jwt` package.

## Import

```go
import mongojwt "github.com/bluetape4k/bluetape-go/jwt/mongo"
```

## Usage

```go
repo, err := mongojwt.New(mongojwt.Options{
    Client:    mongoClient,
    Database:  "service_auth",
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

- `Options` aliases `jwt.MongoRepositoryOptions`.
- `Repository` aliases `jwt.MongoRepository`.
- `New` validates MongoDB repository options through the parent package and
  returns the distributed key-chain repository used by distributed JWT
  providers.
- Signing, parsing, key rotation, retention, and provider cache behavior remain
  owned by `jwt`.

## Operational Boundaries

- Use this package when callers want a MongoDB-specific import path without
  depending on repository implementation names in the parent package.
- MongoDB stores signing key material, retained key payloads, and current `kid`
  metadata in one caller-selected collection. Configure authentication, TLS,
  backup policy, and least-privilege collection access outside this helper.
- Reset and retention operations are explicit parent-package repository
  operations; keep them behind administrative authorization.

## Test

```bash
go test -count=1 ./jwt/mongo
go test -count=1 ./jwt
```
