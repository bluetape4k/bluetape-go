// Package sqsextended - SQS body와 S3 object를 연결하는 bounded large-payload
// envelope adapter를 제공한다.
//
// Provider는 호출자가 생성한 SQS/S3 client만 사용하고 credential, retry,
// queue/bucket provisioning, lifecycle, DLQ와 visibility extension을 소유하지
// 않는다. Send는 S3 object를 먼저 기록하고 envelope를 SQS에 전달한다.
// Receive는 envelope와 payload를 검증하지만 message를 삭제하지 않는다.
// Delete는 SQS message를 먼저 acknowledge한 다음 S3 object를 삭제한다.
// 기본 테스트는 live AWS 대신 fake client를 사용한다.
package sqsextended
