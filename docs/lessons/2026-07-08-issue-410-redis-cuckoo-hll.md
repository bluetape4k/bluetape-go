# Redis probabilistic 후속 교훈

## L1: Client method가 server capability를 증명하지는 않는다

go-redis v9.20.0은 core Redis HLL command와 RedisBloom-style Cuckoo command를 모두
노출한다. 이는 client가 command를 보낼 수 있음을 증명할 뿐이다. `CF*` command가 존재하는지는
server runtime이 결정한다.

예방:

- HLL `PF*`는 core Redis 범위로 취급한다.
- Cuckoo `CF*`는 Testcontainers가 command set을 증명하기 전까지 runtime/module-gated로
  취급한다.
- Cuckoo API를 노출하기 전에 unsupported-command behavior를 문서화한다.

## L2: Membership과 cardinality는 facade를 공유하면 안 된다

Bloom과 Cuckoo는 membership-style question에 답한다. HLL은 cardinality를 estimate한다.
이를 하나의 넓은 probabilistic Redis facade로 합치면 API가 덜 명확해지고 테스트하기도
어려워진다.

예방:

- HLL API name은 count/merge/add 중심으로 유지한다.
- Cuckoo API name은 구현 시 reserve/add/exists/count/delete 중심으로 유지한다.
- Client가 이미 노출한다는 이유만으로 모든 go-redis probabilistic command를 감싸지 않는다.
