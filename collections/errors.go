package collections

import "errors"

// ErrInvalidArgument는 변수 공개 값이다.
// 호출자는 이 식별자를 패키지의 오류, 옵션, 상수, 또는 기본값 계약을 비교할 때 사용한다.
var ErrInvalidArgument = errors.New("collections: invalid argument")
