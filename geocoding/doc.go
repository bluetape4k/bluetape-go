// Package geocoding 은 geo.Point를 주소 결과로 변환하는 caller-owned provider 계약을 제공한다.
//
// 기본 public endpoint, 전역 client, retry, rate-limit, cache는 설치하지
// 않는다. Nominatim adapter는 호출자가 지정한 endpoint와 HTTP policy만 사용한다.
package geocoding
