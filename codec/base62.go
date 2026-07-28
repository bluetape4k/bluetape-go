package codec

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base62 = newAlphabetEncoding("Base62", base62Alphabet)

// EncodeBase62는 EncodeBase62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase62가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeBase62(input []byte) string {
	return base62.encode(input)
}

// DecodeBase62는 DecodeBase62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase62가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase62(input string) ([]byte, error) {
	return base62.decode(input)
}

// EncodeBase62String는 EncodeBase62String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase62String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EncodeBase62String(input string) string {
	return EncodeBase62([]byte(input))
}

// DecodeBase62String는 DecodeBase62String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase62String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase62String(input string) (string, error) {
	decoded, err := DecodeBase62(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Base62 string", decoded)
}

// EncodeURL62는 EncodeURL62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeURL62가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeURL62(input []byte) string {
	return EncodeBase62(input)
}

// DecodeURL62는 DecodeURL62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeURL62가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeURL62(input string) ([]byte, error) {
	return DecodeBase62(input)
}
