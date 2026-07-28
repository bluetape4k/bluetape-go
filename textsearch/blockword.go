package textsearch

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxBlockwordTextLength textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
// NewBlockwordRequest textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
const MaxBlockwordTextLength = 100_000

// Severity textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Severity int

const (
	// SeverityLow textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	SeverityLow Severity = iota
	// SeverityMiddle textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	SeverityMiddle
	// SeverityHigh textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	SeverityHigh
)

// BlockwordEntry textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type BlockwordEntry struct {
	// ID textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	ID string
	// Text textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
	// Severity textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	Severity Severity
	// Metadata textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	Metadata map[string]string
}

// BlockwordOptions textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type BlockwordOptions struct {
	// Mask textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Mask string
	// MinSeverity textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	MinSeverity Severity
}

// BlockwordRequest textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type BlockwordRequest struct {
	Text    string
	Options BlockwordOptions
}

// BlockwordResponse textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type BlockwordResponse struct {
	Request    BlockwordRequest
	MaskedText string
	Matches    []BlockwordMatch
}

// BlockwordExists textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (r BlockwordResponse) BlockwordExists() bool {
	return len(r.Matches) > 0
}

// BlockWords textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (r BlockwordResponse) BlockWords() []string {
	words := make([]string, len(r.Matches))
	for i, match := range r.Matches {
		words[i] = match.Entry.Text
	}
	return words
}

// BlockwordMatch textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type BlockwordMatch struct {
	Entry BlockwordEntry
	Start int
	End   int
	Text  string
}

// BlockwordDictionary textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type BlockwordDictionary struct {
	matcher *Matcher
	entries map[string]BlockwordEntry
}

// NewBlockwordRequest textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func NewBlockwordRequest(text string, options BlockwordOptions) (BlockwordRequest, error) {
	textLength := utf8.RuneCountInString(text)
	if textLength > MaxBlockwordTextLength {
		return BlockwordRequest{}, fmt.Errorf("%w: %d chars (max %d)", ErrBlockwordTextTooLong, textLength, MaxBlockwordTextLength)
	}
	if strings.TrimSpace(text) == "" {
		return BlockwordRequest{}, ErrBlankBlockwordText
	}
	return BlockwordRequest{Text: text, Options: normalizeBlockwordOptions(options)}, nil
}

// NewBlockwordDictionary textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func NewBlockwordDictionary(entries []BlockwordEntry, cfg Config) (*BlockwordDictionary, error) {
	if len(entries) == 0 {
		return nil, ErrNoPatterns
	}
	patterns := make([]Pattern, len(entries))
	compiled := make(map[string]BlockwordEntry, len(entries))
	for i, entry := range entries {
		if entry.Text == "" {
			return nil, ErrEmptyPattern
		}
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("%d", i)
		}
		if _, exists := compiled[entry.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicatePatternID, entry.ID)
		}
		entry.Metadata = copyStringMap(entry.Metadata)
		patterns[i] = Pattern{ID: entry.ID, Text: entry.Text}
		compiled[entry.ID] = entry
	}
	cfg.Overlap = OverlapAll
	matcher, err := Compile(patterns, cfg)
	if err != nil {
		return nil, err
	}
	return &BlockwordDictionary{matcher: matcher, entries: compiled}, nil
}

// Detect textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (d *BlockwordDictionary) Detect(text string, options BlockwordOptions) []BlockwordMatch {
	if d == nil || d.matcher == nil {
		return nil
	}
	options = normalizeBlockwordOptions(options)
	matches := d.matcher.FindAll(text)
	result := make([]BlockwordMatch, 0, len(matches))
	for _, match := range matches {
		entry, ok := d.entries[match.Pattern.ID]
		if !ok || entry.Severity < options.MinSeverity {
			continue
		}
		result = append(result, BlockwordMatch{
			Entry: entry,
			Start: match.Start,
			End:   match.End,
			Text:  match.Text,
		})
	}
	return selectBlockwordLeftmostLongest(result)
}

// Process textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (d *BlockwordDictionary) Process(request BlockwordRequest) (BlockwordResponse, error) {
	normalized, err := NewBlockwordRequest(request.Text, request.Options)
	if err != nil {
		return BlockwordResponse{}, err
	}
	matches := d.Detect(normalized.Text, normalized.Options)
	return BlockwordResponse{
		Request:    normalized,
		MaskedText: maskBlockwordMatches(normalized.Text, matches, normalized.Options.Mask),
		Matches:    matches,
	}, nil
}

// Mask textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (d *BlockwordDictionary) Mask(text string, options BlockwordOptions) (string, error) {
	request, err := NewBlockwordRequest(text, options)
	if err != nil {
		return "", err
	}
	response, err := d.Process(request)
	if err != nil {
		return "", err
	}
	return response.MaskedText, nil
}

func normalizeBlockwordOptions(options BlockwordOptions) BlockwordOptions {
	if options.Mask == "" {
		options.Mask = "*"
	}
	return options
}

func maskBlockwordMatches(input string, matches []BlockwordMatch, mask string) string {
	if len(matches) == 0 {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input))
	offset := 0
	for _, match := range matches {
		builder.WriteString(input[offset:match.Start])
		builder.WriteString(strings.Repeat(mask, utf8.RuneCountInString(match.Text)))
		offset = match.End
	}
	builder.WriteString(input[offset:])
	return builder.String()
}

func selectBlockwordLeftmostLongest(matches []BlockwordMatch) []BlockwordMatch {
	selected := make([]BlockwordMatch, 0, len(matches))
	nextStart := 0
	for _, match := range matches {
		if match.Start < nextStart {
			continue
		}
		selected = append(selected, match)
		nextStart = match.End
	}
	return selected
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
