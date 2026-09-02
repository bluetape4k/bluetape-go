// Package kms - caller-owned AWS KMS client와 local encrypt AES-GCM을 조합하는
// versioned envelope provider를 제공한다.
//
// 이 package는 credentials, client lifecycle, retry, rotation, IAM, cache와 logging을
// 소유하지 않는다. 예제와 테스트는 live AWS 호출 없이 caller가 주입한 fake client만
// 사용한다. envelope metadata에는 key ID와 encryption context가 평문으로 포함되므로
// secret이나 PII를 context에 넣지 않아야 한다.
package kms
