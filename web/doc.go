// Package web 은 net/http 호출자가 공유할 수 있는 오류 응답과 요청 컨텍스트
// 경계를 제공한다.
//
// 이 package는 framework-neutral helper만 소유하며 인증·인가 정책이나
// middleware 구현은 소유하지 않는다.
package web
