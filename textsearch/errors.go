package textsearch

import "errors"

var (
	// ErrEmptyPattern reports an empty pattern in the dictionary.
	ErrEmptyPattern = errors.New("textsearch: pattern must not be empty")
	// ErrNoPatterns reports an empty dictionary.
	ErrNoPatterns = errors.New("textsearch: at least one pattern is required")
	// ErrDuplicatePatternID reports duplicate caller-owned dictionary IDs.
	ErrDuplicatePatternID = errors.New("textsearch: duplicate pattern id")
	// ErrBlockwordTextTooLong reports blockword input above MaxBlockwordTextLength.
	ErrBlockwordTextTooLong = errors.New("textsearch: blockword text too long")
	// ErrBlankBlockwordText reports a blank blockword request.
	ErrBlankBlockwordText = errors.New("textsearch: blockword text must not be blank")
	// ErrTokenizeTextTooLong reports tokenizer input above MaxTokenizeTextLength.
	ErrTokenizeTextTooLong = errors.New("textsearch: tokenize text too long")
	// ErrBlankTokenizeText reports a blank tokenizer request.
	ErrBlankTokenizeText = errors.New("textsearch: tokenize text must not be blank")
	// ErrDuplicateDictionaryText reports duplicate dictionary surface text.
	ErrDuplicateDictionaryText = errors.New("textsearch: duplicate dictionary text")
)
