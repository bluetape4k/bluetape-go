package language

import "errors"

var (
	// ErrBlankText reports blank language-detection input.
	ErrBlankText = errors.New("textsearch/language: text must not be blank")
	// ErrTextTooLong reports language-detection input above MaxTextLength.
	ErrTextTooLong = errors.New("textsearch/language: text too long")
	// ErrLanguageSubsetTooSmall reports detector subsets below Lingua's minimum.
	ErrLanguageSubsetTooSmall = errors.New("textsearch/language: at least two known languages are required")
	// ErrInvalidMinimumRelativeDistance reports a Lingua distance outside [0.0, 0.99].
	ErrInvalidMinimumRelativeDistance = errors.New("textsearch/language: minimum relative distance must be between 0.0 and 0.99")
)
