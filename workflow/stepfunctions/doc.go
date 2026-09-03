// Package stepfunctions - AWS Step Functions execution을 caller-owned client로
// 시작하고, 조회하고, 선택적으로 중지하며, bounded polling으로 기다리는
// 좁은 외부 실행 bridge를 제공한다.
//
// state-machine 정의·배포, credentials, retry, endpoint, IAM과 운영 lifecycle은
// package 바깥의 caller가 소유한다. 기본 테스트와 예제는 live AWS credential을
// 사용하지 않는다.
package stepfunctions
