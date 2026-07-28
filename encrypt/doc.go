// Package encrypt는 local service data를 위한 작은 AES-GCM facade를 제공한다.
//
// 이 package는 Go standard library의 AES-GCM random-nonce AEAD를 사용하고 ciphertext를
// versioned envelope로 감싼다. 호출자는 key 생성, 영속화, 회전, 저장을 소유한다. 이 package는
// durable singleton key를 생성하지 않고 nonce 관리를 노출하지 않는다.
//
// associated data는 tenant, entity, column, message type, protocol version처럼 안정적인
// context에 ciphertext를 묶는 데 사용한다. 복호화할 때도 동일한 associated data를 전달해야 한다.
//
// 이 package는 byte와 UTF-8 string 암호화를 위한 것이다. password hashing helper, JWT signer,
// MAC-only API, deterministic searchable encryption API, KMS envelope system,
// file/stream encryption package가 아니다.
package encrypt
