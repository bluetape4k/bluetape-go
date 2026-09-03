// Package eventbridge - audit/sqloutbox.Record를 Amazon EventBridge로 전달하는
// caller-owned publisher adapter를 제공한다.
//
// Publisher는 한 번에 하나의 entry만 PutEvents로 전송하며 stable event ID와
// idempotency key를 JSON detail에 보존한다. Event bus/rule/target provisioning,
// AWS client lifecycle, retry와 downstream idempotency는 호출자가 소유한다.
// Publisher와 Options는 constructor 경계에서 유효성이 보장되며, Publisher의
// zero value는 사용할 수 없고 Publish가 안전한 오류를 반환한다.
// 기본 테스트는 live AWS 대신 fake Client를 사용하고, context cancellation은
// 기존 sqloutbox.Relay 계약에 따라 retry/dead-letter와 구별된다.
package eventbridge
