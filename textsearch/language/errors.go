package language

import "errors"

var (
	// ErrBlankText textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrBlankText = errors.New("textsearch/language: text must not be blank")
	// ErrTextTooLong textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrTextTooLong = errors.New("textsearch/language: text too long")
	// ErrLanguageSubsetTooSmall textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
	ErrLanguageSubsetTooSmall = errors.New("textsearch/language: at least two known languages are required")
	// ErrInvalidMinimumRelativeDistance textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrInvalidMinimumRelativeDistance = errors.New("textsearch/language: minimum relative distance must be between 0.0 and 0.99")
)
