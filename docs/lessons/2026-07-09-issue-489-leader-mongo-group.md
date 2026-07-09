# MongoDB Group Leader Lessons

## L1: Model group leadership as bounded slots, not an unbounded owner array

MongoDB can enforce a group elector cap without transactions when each possible
leader slot has its own `_id`. A contender atomically acquires one expired slot
document, and no more than `MaxLeaders` slot documents are eligible for a given
group.

Prevention:

- Keep slot IDs deterministic: `<keyPrefix>:<group>:slot:<slot>`.
- Acquire only from `[0, MaxLeaders)` for the normalized group.
- Count active leadership with `group_key` plus `lease_until > now`.
- Treat duplicate-key and no-match acquisition races as lost attempts, not
  backend errors.

## L2: TTL remains cleanup only for group electors

Group slots can be reused before MongoDB's TTL monitor deletes old documents.
Correctness comes from the acquisition and count predicates on `lease_until`,
not from physical deletion.

Prevention:

- Keep `lease_until <= now` as the takeover predicate.
- Keep `lease_until > now` as the active-count predicate.
- Document the TTL index as cleanup support only.

## L3: Renewal loss must clear local group leadership

`GroupElector.IsLeader` is local state, so renewal must update exactly the slot
and owner token acquired by the elector. A zero-match renewal means the slot was
removed, replaced, or expired and must flip local state to false.

Prevention:

- Store the acquired slot number locally.
- Renew with `_id`, `token`, and `lease_until > now`.
- Race-test token replacement and concurrent contenders.
