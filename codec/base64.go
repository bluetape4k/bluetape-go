package codec

import "encoding/base64"

// EncodeBase64 EncodeBase64 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase64가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeBase64(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// DecodeBase64 DecodeBase64 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase64가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase64(input string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(input)
}

// EncodeBase64URL EncodeBase64URL 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase64URL가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeBase64URL(input []byte) string {
	return base64.RawURLEncoding.EncodeToString(input)
}

// DecodeBase64URL DecodeBase64URL 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase64URL가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase64URL(input string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(input)
}

// EncodeBase64String EncodeBase64String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase64String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EncodeBase64String(input string) string {
	return EncodeBase64([]byte(input))
}

// DecodeBase64String DecodeBase64String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase64String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase64String(input string) (string, error) {
	decoded, err := DecodeBase64(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Base64 string", decoded)
}
