package codec

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58 = newAlphabetEncoding("Base58", base58Alphabet)

// EncodeBase58 EncodeBase58 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase58가 읽거나 복사하는 input 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func EncodeBase58(input []byte) string {
	return base58.encode(input)
}

// DecodeBase58 DecodeBase58 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase58가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase58(input string) ([]byte, error) {
	return base58.decode(input)
}

// EncodeBase58String EncodeBase58String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: EncodeBase58String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func EncodeBase58String(input string) string {
	return EncodeBase58([]byte(input))
}

// DecodeBase58String DecodeBase58String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - input: DecodeBase58String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeBase58String(input string) (string, error) {
	decoded, err := DecodeBase58(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Base58 string", decoded)
}
