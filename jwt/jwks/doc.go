// Package jwks 는 원격 JWKS JSON에서 서명 검증용 공개키를 제공한다.
//
// RSA, ECDSA, Ed25519 공개키만 허용하며 대칭키와 JWE는 지원하지 않는다.
// Provider 생성은 network-free이고, 운영 readiness가 필요한 호출자는
// traffic을 열기 전에 Refresh를 명시적으로 호출해야 한다.
package jwks
