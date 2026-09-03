// Package ssm - caller-owned AWS Systems Manager Parameter Store client로
// parameter 값을 조회하는 좁은 provider를 제공한다.
//
// credentials, AWS config, retry, endpoint, client lifecycle와 parameter
// precedence는 caller가 소유한다. SecureString 복호화는 `GetSecure` 또는
// Options.WithDecryption으로 명시하며 반환된 Value는 기본 formatter에서
// 원문을 숨긴다.
package ssm
