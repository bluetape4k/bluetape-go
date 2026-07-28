package codec

import "encoding/hex"

// EncodeHex 입력 값을 Hex 형식으로 인코딩한다.
//
// 매개변수:
//   - input: 인코딩할 바이트 슬라이스다. nil이나 빈 슬라이스는 빈 입력으로 처리한다.
func EncodeHex(input []byte) string {
	return hex.EncodeToString(input)
}

// DecodeHex Hex 형식의 입력을 원래 값으로 디코딩한다.
//
// 매개변수:
//   - input: 디코딩할 문자열이다. 빈 문자열과 잘못된 문자는 구현의 검증 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func DecodeHex(input string) ([]byte, error) {
	return hex.DecodeString(input)
}

// EncodeHexString 입력 값을 Hex 형식으로 인코딩한다.
//
// 매개변수:
//   - input: 처리할 문자열이다. 빈 문자열 허용 여부는 해당 함수의 검증 규칙을 따른다.
func EncodeHexString(input string) string {
	return EncodeHex([]byte(input))
}

// DecodeHexString Hex 형식의 입력을 원래 값으로 디코딩한다.
//
// 매개변수:
//   - input: 디코딩할 문자열이다. 빈 문자열과 잘못된 문자는 구현의 검증 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func DecodeHexString(input string) (string, error) {
	decoded, err := DecodeHex(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Hex string", decoded)
}
