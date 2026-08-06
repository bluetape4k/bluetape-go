# MongoDB Group Leader 교훈

## L1: group leadership은 unbounded owner array가 아니라 bounded slot으로 모델링한다

가능한 leader slot마다 자체 `_id`를 두면 MongoDB는 transaction 없이 group elector
cap을 강제할 수 있다. contender는 expired slot document 하나를 atomic하게
acquire하고, 특정 group에 대해 eligible한 slot document 수는 `MaxLeaders`를 넘지
않는다.

Prevention:

- slot ID는 deterministic하게 유지한다: `<keyPrefix>:<group>:slot:<slot>`.
- normalized group에 대해 `[0, MaxLeaders)`에서만 acquire한다.
- active leadership은 `group_key`와 `lease_until > now`로 count한다.
- duplicate-key와 no-match acquisition race는 backend error가 아니라 lost attempt로
  취급한다.

## L2: group elector에서도 TTL은 cleanup 전용이다

MongoDB TTL monitor가 old document를 삭제하기 전에 group slot은 재사용될 수 있다.
correctness는 physical deletion이 아니라 `lease_until`에 대한 acquisition/count
predicate에서 나온다.

Prevention:

- `lease_until <= now`를 takeover predicate로 유지한다.
- `lease_until > now`를 active-count predicate로 유지한다.
- TTL index를 cleanup support로만 문서화한다.

## L3: renewal loss는 local group leadership을 지워야 한다

`GroupElector.IsLeader`는 local state이므로 renewal은 elector가 획득한 정확한 slot과
owner token을 update해야 한다. zero-match renewal은 slot이 제거, 교체 또는 만료된
것이므로 local state를 false로 바꿔야 한다.

Prevention:

- acquired slot number를 local에 저장한다.
- `_id`, `token`, `lease_until > now`로 renew한다.
- token replacement와 concurrent contender를 race-test한다.
