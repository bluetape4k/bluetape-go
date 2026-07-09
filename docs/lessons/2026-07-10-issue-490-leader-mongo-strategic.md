# Issue #490 Mongo Strategic Leader Elector

MongoDB strategic election should mirror the Redis public contract without
copying its key model. Redis needs a candidate payload plus sorted index key;
MongoDB can store each candidate as one leased document and scan by
`group_key, lease_until`.

Lesson: treat TTL as cleanup only. `ListCandidates` must explicitly prune or
filter expired candidate documents before strategy evaluation, because MongoDB's
TTL monitor is asynchronous and cannot be part of correctness.

Lesson: result counters need an atomic backend update guarded by a live lease
predicate. A read-modify-write candidate refresh would lose concurrent outcome
increments under stress.

Prevention: when adding another strategic backend, prove at least candidate
registration/listing, stale cleanup before TTL deletion, FIFO/scored/random
strategy compatibility, missing/expired result rejection, failure recording, and
contention-safe result updates.
