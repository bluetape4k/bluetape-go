// Package auth - AWS RDS IAM database authentication token을 만드는 좁은
// caller-owned helper를 제공한다.
//
// credentials, AWS config, IAM policy, database driver, connection pool과
// token refresh lifecycle은 caller가 소유한다. BuildAuthToken은 SDK가 만드는
// 15-minute token을 검증된 redacted wrapper로 반환하고 database connection을
// 열거나 관리하지 않는다.
package auth
