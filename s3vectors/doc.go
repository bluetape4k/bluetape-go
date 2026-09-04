// Package s3vectors (S3 Vectors) 를 위한 얇은 caller-owned
// bridge를 제공한다.
//
// Provider는 AWS SDK for Go v2 request/response type을 경계에 유지한다.
// Embedding을 생성하거나 dimension/distance metric을 추론하지 않고,
// metadata schema/filter를 해석하거나 client/credential, retry, pagination과
// logging을 소유하지 않는다. 이 정책과 client lifecycle은 caller가 소유한다.
//
// 기본 test는 fake client만 사용한다. S3 Vectors emulator compatibility를
// 검증하지 않았으므로 local emulator 지원을 주장하지 않는다. live test를
// 추가하더라도 명시적인 opt-in으로 유지해야 한다.
package s3vectors
