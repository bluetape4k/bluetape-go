// Package secretsmanager - caller-owned AWS Secrets Manager client로 secret
// 값을 조회하는 좁은 provider를 제공한다.
//
// credentials, AWS config, retry, endpoint, client lifecycle와 secret rotation은
// caller가 소유한다. 반환된 Value는 기본 formatter에서 비밀값을 숨기며,
// 원문은 명시적인 Bytes 또는 Text 호출로만 읽는다.
package secretsmanager
