package textsearch

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTokenizeTextLength는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
// NewTokenizeRequest는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
const MaxTokenizeTextLength = 100_000

// PartOfSpeech는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
//
// 이 주석은 tokenizer, normalization, language detection, image/example 경계를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
type PartOfSpeech string

const (
	// POSUnknown는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSUnknown PartOfSpeech = "unknown"
	// POSWord는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSWord PartOfSpeech = "word"
	// POSNumber는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSNumber PartOfSpeech = "number"
	// POSWhitespace는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSWhitespace PartOfSpeech = "whitespace"
	// POSPunctuation는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSPunctuation PartOfSpeech = "punctuation"
	// POSSymbol는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	POSSymbol PartOfSpeech = "symbol"
)

// TokenSpan는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
//
// End는 textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
type TokenSpan struct {
	Start int
	End   int
}

// Token는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Token struct {
	// Text는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
	// Span는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Span TokenSpan
	// Normalized는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Normalized string
	// POS는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	POS PartOfSpeech
	// Metadata는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	Metadata map[string]string
}

// NormalizedText는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type NormalizedText struct {
	Original   string
	Normalized string
	Mode       NormalizeMode
}

// NormalizeText는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func NormalizeText(input string, mode NormalizeMode) NormalizedText {
	normalized := normalizeString(input, Config{Normalize: mode})
	return NormalizedText{Original: input, Normalized: normalized.text, Mode: mode}
}

// TokenizeOptions는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type TokenizeOptions struct {
	// Normalize는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	Normalize NormalizeMode
	// IncludeWhitespace는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	IncludeWhitespace bool
}

// TokenizeRequest는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type TokenizeRequest struct {
	Text    string
	Options TokenizeOptions
}

// TokenizeResponse는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type TokenizeResponse struct {
	Request TokenizeRequest
	Tokens  []Token
}

// Texts는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (r TokenizeResponse) Texts() []string {
	texts := make([]string, len(r.Tokens))
	for i, token := range r.Tokens {
		texts[i] = token.Text
	}
	return texts
}

// NormalizedTexts는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (r TokenizeResponse) NormalizedTexts() []string {
	texts := make([]string, len(r.Tokens))
	for i, token := range r.Tokens {
		texts[i] = token.Normalized
	}
	return texts
}

// NewTokenizeRequest는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
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

// Tokenizer는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Tokenizer interface {
	Tokenize(TokenizeRequest) (TokenizeResponse, error)
}

// TokenizerFunc는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type TokenizerFunc func(TokenizeRequest) (TokenizeResponse, error)

// Tokenize는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func (f TokenizerFunc) Tokenize(request TokenizeRequest) (TokenizeResponse, error) {
	return f(request)
}

// SimpleTokenizer는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
//
// 이 주석은 tokenizer, normalization, language detection, image/example 경계를 설명한다.
// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
type SimpleTokenizer struct{}

// NewSimpleTokenizer는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
func NewSimpleTokenizer() SimpleTokenizer {
	return SimpleTokenizer{}
}

// Tokenize는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
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

// DictionaryEntry는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type DictionaryEntry struct {
	// ID는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	ID string
	// Text는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
	// Normalized는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Normalized string
	// POS는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	POS PartOfSpeech
	// Severity는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	Severity Severity
	// Metadata는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	Metadata map[string]string
}

// DictionaryProvider는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type DictionaryProvider interface {
	Entries(context.Context) ([]DictionaryEntry, error)
}

// StaticDictionaryProvider는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type StaticDictionaryProvider []DictionaryEntry

// Entries는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (p StaticDictionaryProvider) Entries(ctx context.Context) ([]DictionaryEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return copyDictionaryEntries([]DictionaryEntry(p)), nil
}

// DictionarySet는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type DictionarySet struct {
	entries []DictionaryEntry
	byText  map[string]DictionaryEntry
}

// NewDictionarySet는 textsearch language image example에서 생성과 초기화 계약을 설명한다.
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

// Entries는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (s *DictionarySet) Entries() []DictionaryEntry {
	if s == nil {
		return nil
	}
	return copyDictionaryEntries(s.entries)
}

// Contains는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (s *DictionarySet) Contains(text string) bool {
	if s == nil {
		return false
	}
	_, ok := s.byText[text]
	return ok
}

// Entry는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
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
