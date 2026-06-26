package textsearch

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxBlockwordTextLength is the maximum input size accepted by
// NewBlockwordRequest and BlockwordDictionary.Process.
const MaxBlockwordTextLength = 100_000

// Severity represents a blockword exposure level.
type Severity int

const (
	// SeverityLow is the default blockword severity.
	SeverityLow Severity = iota
	// SeverityMiddle represents medium severity blockwords.
	SeverityMiddle
	// SeverityHigh represents high severity blockwords.
	SeverityHigh
)

// BlockwordEntry is one compiled blockword dictionary entry.
type BlockwordEntry struct {
	// ID is caller-owned metadata. When empty, a stable decimal index is used.
	ID string
	// Text is the word or pattern text to detect.
	Text string
	// Severity describes the entry's exposure level.
	Severity Severity
	// Metadata carries caller-owned labels such as category or source.
	Metadata map[string]string
}

// BlockwordOptions controls detection and masking.
type BlockwordOptions struct {
	// Mask is repeated once per matched rune when building MaskedText.
	Mask string
	// MinSeverity keeps entries with Severity >= MinSeverity. The zero value
	// includes every entry.
	MinSeverity Severity
}

// BlockwordRequest is a validated blockword processing request.
type BlockwordRequest struct {
	Text    string
	Options BlockwordOptions
}

// BlockwordResponse contains blockword processing results.
type BlockwordResponse struct {
	Request    BlockwordRequest
	MaskedText string
	Matches    []BlockwordMatch
}

// BlockwordExists reports whether the response detected at least one entry.
func (r BlockwordResponse) BlockwordExists() bool {
	return len(r.Matches) > 0
}

// BlockWords returns detected dictionary texts in match order.
func (r BlockwordResponse) BlockWords() []string {
	words := make([]string, len(r.Matches))
	for i, match := range r.Matches {
		words[i] = match.Entry.Text
	}
	return words
}

// BlockwordMatch describes one blockword occurrence in the original input.
type BlockwordMatch struct {
	Entry BlockwordEntry
	Start int
	End   int
	Text  string
}

// BlockwordDictionary is an immutable compiled blockword matcher.
type BlockwordDictionary struct {
	matcher *Matcher
	entries map[string]BlockwordEntry
}

// NewBlockwordRequest validates text and applies default options.
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

// NewBlockwordDictionary compiles entries into an immutable dictionary.
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

// Detect returns deterministic non-overlapping blockword matches.
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

// Process validates request text, detects blockwords, and returns masked text.
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

// Mask validates text and returns the masked output for convenience.
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
