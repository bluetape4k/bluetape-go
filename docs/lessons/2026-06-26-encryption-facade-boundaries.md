# Encryption Facade Boundary 교훈

Go encryption 작업에서 default facade는 nonce management를 숨겨야 하며, Go standard
library가 contract를 만족할 수 있다면 common path를 dependency-free로 유지해야 한다.
Go 1.26에서는 byte/string AEAD facade의 시작점으로
`cipher.NewGCMWithRandomNonce`가 적절하다.

durable ciphertext에 ephemeral singleton key를 생성하지 않는다. key material은
caller-owned, persisted, 또는 explicit provider에서 load되어야 한다. 이는 persisted
encrypted column이 새로 generated keyset에 의존할 수 없다는 Exposed Tink lesson과도
맞다.

deterministic AEAD, Tink keyset, Redis keyset store, AWS KMS envelope support, age
file encryption, MAC, digest helper는 concrete caller가 operational risk를 소유할 때까지
default package 밖에 둔다.
