# Redis Leader Key 호환성

Kotlin/JVM Redis leader와 Go Redis leader는 같은 Redis를 사용할 수 있지만 같은
leader group의 참가자로 섞으면 안 된다. key 이름, value token, release/renew 계약이
서로 다르기 때문이다.

Interop 여부를 결정할 때는 “같은 backend 종류”가 아니라 “같은 key/value/TTL 계약”인지
비교해야 한다. 호환되지 않으면 README와 package doc에 독립 운영 지침을 명시하고,
Go 구현이 어떤 key를 쓰는지 테스트로 고정한다.
