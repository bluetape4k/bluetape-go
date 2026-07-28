package textsearch

import "errors"

var (
	// ErrEmptyPattern textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrEmptyPattern = errors.New("textsearch: pattern must not be empty")
	// ErrNoPatterns textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrNoPatterns = errors.New("textsearch: at least one pattern is required")
	// ErrDuplicatePatternID textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrDuplicatePatternID = errors.New("textsearch: duplicate pattern id")
	// ErrBlockwordTextTooLong textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrBlockwordTextTooLong = errors.New("textsearch: blockword text too long")
	// ErrBlankBlockwordText textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrBlankBlockwordText = errors.New("textsearch: blockword text must not be blank")
	// ErrTokenizeTextTooLong textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrTokenizeTextTooLong = errors.New("textsearch: tokenize text too long")
	// ErrBlankTokenizeText textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrBlankTokenizeText = errors.New("textsearch: tokenize text must not be blank")
	// ErrDuplicateDictionaryText textsearch language image example에서 반환값과 오류 의미를 설명한다.
	ErrDuplicateDictionaryText = errors.New("textsearch: duplicate dictionary text")
)
