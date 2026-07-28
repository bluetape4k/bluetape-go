// Package etcdleader leader backend election 계약과 호출자 사용 경계를 설명한다.
// 이 주석은 leader backend election의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
//
// Construct leader backend election에서 caller-visible 상태와 의미를 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
package etcdleader
