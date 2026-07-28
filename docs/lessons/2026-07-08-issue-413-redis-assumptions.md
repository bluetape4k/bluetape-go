# 교훈: Redis Probabilistic Assumption Docs

Redis probabilistic README diagram을 갱신할 때는 Redis module dependency를
암시하지 말고 구현된 Redis command contract를 명시한다.

- Bloom과 HyperLogLog는 현재 core Redis commands만 사용한다. Cuckoo `CF*`
  commands는 module, persistence, ACL, Testcontainers contract가 생길 때까지
  follow-up scope로 문서화해 둔다.
- Package README guidance는 이 가정을 상속하는 exported constructor를 이름으로
  적고, operator requirement를 package test에서 쓰는 동일 Redis image에 묶어야
  한다.
- Diagram은 connector audit을 통과해도 시각적으로 약할 수 있다. 좌표 변경 후에는
  full-size PNG eye check를 실행하고 모든 connector가 의도한 card boundary에서
  시작하고 끝나는지 확인한다.
