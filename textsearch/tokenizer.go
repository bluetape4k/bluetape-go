package textsearch

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTokenizeTextLength is the maximum input size accepted by
// NewTokenizeRequest and SimpleTokenizer.Tokenize.
const MaxTokenizeTextLength = 100_000

// PartOfSpeech is a tokenizer-owned coarse token class.
//
// The core package intentionally keeps this small. Language-specific
// morphological tags belong in tokenizer implementations that can place their
// original tag in Token.Metadata.
type PartOfSpeech string

const (
	// POSUnknown reports an implementation-specific token class.
	POSUnknown PartOfSpeech = "unknown"
	// POSWord reports contiguous Unicode letters and combining marks.
	POSWord PartOfSpeech = "word"
	// POSNumber reports contiguous Unicode digits.
	POSNumber PartOfSpeech = "number"
	// POSWhitespace reports contiguous Unicode whitespace.
	POSWhitespace PartOfSpeech = "whitespace"
	// POSPunctuation reports contiguous Unicode punctuation.
	POSPunctuation PartOfSpeech = "punctuation"
	// POSSymbol reports contiguous Unicode symbols or other single-rune tokens.
	POSSymbol PartOfSpeech = "symbol"
)

// TokenSpan identifies a token byte range in the original input.
//
// End is exclusive. Spans are byte offsets so callers can slice the original Go
// string without requiring normalized-offset mapping.
type TokenSpan struct {
	Start int
	End   int
}

// Token is one lexical token from a tokenizer.
type Token struct {
	// Text is the original input slice for Span.
	Text string
	// Span identifies Text in the original input.
	Span TokenSpan
	// Normalized is Text after the request normalization mode is applied.
	Normalized string
	// POS is the tokenizer-owned coarse or language-specific token class.
	POS PartOfSpeech
	// Metadata carries tokenizer-owned labels such as language, original POS
	// tag, dictionary source, or confidence. Implementations should copy maps
	// before returning tokens.
	Metadata map[string]string
}

// NormalizedText describes a caller-visible normalization result.
type NormalizedText struct {
	Original   string
	Normalized string
	Mode       NormalizeMode
}

// NormalizeText applies a textsearch normalization mode without case folding.
func NormalizeText(input string, mode NormalizeMode) NormalizedText {
	normalized := normalizeString(input, Config{Normalize: mode})
	return NormalizedText{Original: input, Normalized: normalized.text, Mode: mode}
}

// TokenizeOptions controls tokenizer output.
type TokenizeOptions struct {
	// Normalize applies Unicode normalization to Token.Normalized. Token spans
	// still refer to byte offsets in the original input.
	Normalize NormalizeMode
	// IncludeWhitespace keeps whitespace tokens in the response. The zero value
	// skips whitespace.
	IncludeWhitespace bool
}

// TokenizeRequest is a validated tokenizer request.
type TokenizeRequest struct {
	Text    string
	Options TokenizeOptions
}

// TokenizeResponse contains tokenizer results.
type TokenizeResponse struct {
	Request TokenizeRequest
	Tokens  []Token
}

// Texts returns original token texts in order.
func (r TokenizeResponse) Texts() []string {
	texts := make([]string, len(r.Tokens))
	for i, token := range r.Tokens {
		texts[i] = token.Text
	}
	return texts
}

// NormalizedTexts returns normalized token texts in order.
func (r TokenizeResponse) NormalizedTexts() []string {
	texts := make([]string, len(r.Tokens))
	for i, token := range r.Tokens {
		texts[i] = token.Normalized
	}
	return texts
}

// NewTokenizeRequest validates text and applies tokenizer defaults.
func NewTokenizeRequest(text string, options TokenizeOptions) (TokenizeRequest, error) {
	textLength := utf8.RuneCountInString(text)
	if textLength > MaxTokenizeTextLength {
		return TokenizeRequest{}, fmt.Errorf("%w: %d chars (max %d)", ErrTokenizeTextTooLong, textLength, MaxTokenizeTextLength)
	}
	if strings.TrimSpace(text) == "" {
		return TokenizeRequest{}, ErrBlankTokenizeText
	}
	return TokenizeRequest{Text: text, Options: options}, nil
}

// Tokenizer is the minimal tokenizer contract used by textsearch callers.
type Tokenizer interface {
	Tokenize(TokenizeRequest) (TokenizeResponse, error)
}

// TokenizerFunc adapts a function to Tokenizer.
type TokenizerFunc func(TokenizeRequest) (TokenizeResponse, error)

// Tokenize calls f(request).
func (f TokenizerFunc) Tokenize(request TokenizeRequest) (TokenizeResponse, error) {
	return f(request)
}

// SimpleTokenizer is a dependency-free deterministic lexical tokenizer.
//
// It groups Unicode letters and combining marks, digits, whitespace, and
// punctuation. It is not a morphological analyzer and does not detect language.
type SimpleTokenizer struct{}

// NewSimpleTokenizer returns a dependency-free tokenizer for tests and simple
// lexical workflows.
func NewSimpleTokenizer() SimpleTokenizer {
	return SimpleTokenizer{}
}

// Tokenize validates request text and returns deterministic lexical tokens.
func (t SimpleTokenizer) Tokenize(request TokenizeRequest) (TokenizeResponse, error) {
	normalized, err := NewTokenizeRequest(request.Text, request.Options)
	if err != nil {
		return TokenizeResponse{}, err
	}

	tokens := simpleTokenize(normalized.Text, normalized.Options)
	return TokenizeResponse{Request: normalized, Tokens: tokens}, nil
}

func simpleTokenize(input string, options TokenizeOptions) []Token {
	tokens := make([]Token, 0, utf8.RuneCountInString(input)/2)
	var current simpleToken

	flush := func() {
		if current.start == current.end {
			return
		}
		if current.pos == POSWhitespace && !options.IncludeWhitespace {
			current = simpleToken{}
			return
		}
		text := input[current.start:current.end]
		tokens = append(tokens, Token{
			Text:       text,
			Span:       TokenSpan{Start: current.start, End: current.end},
			Normalized: NormalizeText(text, options.Normalize).Normalized,
			POS:        current.pos,
		})
		current = simpleToken{}
	}

	for start, r := range input {
		end := start + runeWidth(input[start:], r)
		pos := simpleTokenPOS(r)
		if current.start == current.end {
			current = simpleToken{start: start, end: end, pos: pos}
			continue
		}
		if current.pos == pos && canJoinSimpleToken(current.pos) {
			current.end = end
			continue
		}
		flush()
		current = simpleToken{start: start, end: end, pos: pos}
	}
	flush()
	return tokens
}

type simpleToken struct {
	start int
	end   int
	pos   PartOfSpeech
}

func simpleTokenPOS(r rune) PartOfSpeech {
	switch {
	case unicode.IsLetter(r) || unicode.IsMark(r):
		return POSWord
	case unicode.IsDigit(r):
		return POSNumber
	case unicode.IsSpace(r):
		return POSWhitespace
	case unicode.IsPunct(r):
		return POSPunctuation
	default:
		return POSSymbol
	}
}

func canJoinSimpleToken(pos PartOfSpeech) bool {
	switch pos {
	case POSWord, POSNumber, POSWhitespace, POSPunctuation:
		return true
	default:
		return false
	}
}

func runeWidth(input string, r rune) int {
	if r == utf8.RuneError {
		_, width := utf8.DecodeRuneInString(input)
		return width
	}
	return utf8.RuneLen(r)
}

// DictionaryEntry is one tokenizer dictionary item.
type DictionaryEntry struct {
	// ID is caller-owned metadata. When empty, a stable decimal index is used.
	ID string
	// Text is the dictionary surface form.
	Text string
	// Normalized is optional normalized surface form.
	Normalized string
	// POS is an optional coarse or implementation-specific token class.
	POS PartOfSpeech
	// Severity can be reused by blockword-style dictionary consumers.
	Severity Severity
	// Metadata carries caller-owned labels such as source, language, or model.
	Metadata map[string]string
}

// DictionaryProvider loads tokenizer dictionary entries.
type DictionaryProvider interface {
	Entries(context.Context) ([]DictionaryEntry, error)
}

// StaticDictionaryProvider is an in-memory DictionaryProvider.
type StaticDictionaryProvider []DictionaryEntry

// Entries returns a copy of provider entries.
func (p StaticDictionaryProvider) Entries(ctx context.Context) ([]DictionaryEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return copyDictionaryEntries([]DictionaryEntry(p)), nil
}

// DictionarySet is an immutable lookup over dictionary entries.
type DictionarySet struct {
	entries []DictionaryEntry
	byText  map[string]DictionaryEntry
}

// NewDictionarySet builds an immutable dictionary lookup from entries.
func NewDictionarySet(entries []DictionaryEntry) (*DictionarySet, error) {
	if len(entries) == 0 {
		return nil, ErrNoPatterns
	}
	copied := copyDictionaryEntries(entries)
	byText := make(map[string]DictionaryEntry, len(copied))
	for index, entry := range copied {
		if entry.Text == "" {
			return nil, ErrEmptyPattern
		}
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("%d", index)
		}
		if entry.Normalized == "" {
			entry.Normalized = NormalizeText(entry.Text, NormalizeNFC).Normalized
		}
		if _, exists := byText[entry.Text]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateDictionaryText, entry.Text)
		}
		copied[index] = entry
		byText[entry.Text] = entry
	}
	return &DictionarySet{entries: copied, byText: byText}, nil
}

// Entries returns a copy of dictionary entries in construction order.
func (s *DictionarySet) Entries() []DictionaryEntry {
	if s == nil {
		return nil
	}
	return copyDictionaryEntries(s.entries)
}

// Contains reports whether text exists in the dictionary.
func (s *DictionarySet) Contains(text string) bool {
	if s == nil {
		return false
	}
	_, ok := s.byText[text]
	return ok
}

// Entry returns a copy of the dictionary entry for text.
func (s *DictionarySet) Entry(text string) (DictionaryEntry, bool) {
	if s == nil {
		return DictionaryEntry{}, false
	}
	entry, ok := s.byText[text]
	if !ok {
		return DictionaryEntry{}, false
	}
	entry.Metadata = copyStringMap(entry.Metadata)
	return entry, true
}

func copyDictionaryEntries(entries []DictionaryEntry) []DictionaryEntry {
	if len(entries) == 0 {
		return nil
	}
	copied := make([]DictionaryEntry, len(entries))
	for i, entry := range entries {
		entry.Metadata = copyStringMap(entry.Metadata)
		copied[i] = entry
	}
	return copied
}
