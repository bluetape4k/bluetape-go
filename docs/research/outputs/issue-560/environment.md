# Issue 560 Provider Benchmark Environment

## Capture authority

- UTC start: `2026-07-20T16:59:36Z`
- Local start: `2026-07-21T01:59:36+0900 KST`
- Timezone: `Asia/Seoul`
- Git SHA: `ef3ef4f3070f516a3c75c2637f8e2bca231d9370`
- Pre-run worktree state: clean outside this artifact directory
- Post-run worktree state: only this artifact directory was modified

## Host and runtime

- Host OS/kernel/architecture: `Darwin 25.5.0 arm64`
- CPU: `Apple M5`
- Logical CPUs: `10`
- RAM bytes: `34359738368`
- Go: `go1.26.5 darwin/arm64`
- Docker client: `29.6.2`
- Docker server: `28.4.0`
- Docker platform: `Docker Engine - Community`, `linux/arm64`

## Reviewed immutable fixtures

- Redis: `redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99`
- MongoDB: `mongo:7.0@sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f`
- PostgreSQL: `postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777`
- etcd (`linux/arm64`): `gcr.io/etcd-development/etcd@sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96`
- Neo4j: `neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5`
- Memgraph: `memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1`

## Provider-reported versions

- Redis: `7.4.9`
- MongoDB: `7.0.37`
- PostgreSQL: `16.14`
- etcd: `3.6.13`
- Neo4j: `Neo4j/5.26.0`
- Memgraph: `3.5.0`
