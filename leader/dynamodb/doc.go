// Package dynamodbleader caller-owned DynamoDB table에서 조건부 item 쓰기로
// 단일 leader lease를 조정한다.
//
// AWS client와 credential/config, table provisioning, retry policy, logger sink는
// caller가 소유한다. TTL attribute는 비동기 cleanup hint일 뿐이며, active
// ownership는 strongly consistent read와 lease deadline으로 판단한다.
package dynamodbleader
