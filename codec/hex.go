package codec

import "encoding/hex"

// EncodeHex는 EncodeHex 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeHex가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeHex(input []byte) string {
	return hex.EncodeToString(input)
}

// DecodeHex는 DecodeHex 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeHex가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeHex(input string) ([]byte, error) {
	return hex.DecodeString(input)
}

// EncodeHexString는 EncodeHexString 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeHexString가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EncodeHexString(input string) string {
	return EncodeHex([]byte(input))
}

// DecodeHexString는 DecodeHexString 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeHexString가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeHexString(input string) (string, error) {
	decoded, err := DecodeHex(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Hex string", decoded)
}
