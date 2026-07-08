# Redis Probabilistic Follow-Up Lessons

## L1: Client methods do not prove server capability

go-redis v9.20.0 exposes both core Redis HLL commands and RedisBloom-style
Cuckoo commands. That only proves the client can send the commands. The server
runtime still decides whether `CF*` commands exist.

Prevention:

- Treat HLL `PF*` as core Redis scope.
- Treat Cuckoo `CF*` as runtime/module-gated until Testcontainers proves the
  command set.
- Document unsupported-command behavior before exposing Cuckoo APIs.

## L2: Membership and cardinality should not share a facade

Bloom and Cuckoo answer membership-style questions. HLL estimates cardinality.
Combining those into one broad probabilistic Redis facade would make the API
less clear and harder to test.

Prevention:

- Keep HLL API names count/merge/add oriented.
- Keep Cuckoo API names reserve/add/exists/count/delete oriented when it is
  implemented.
- Do not wrap all go-redis probabilistic commands just because the client
  already exposes them.
