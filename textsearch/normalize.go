package textsearch

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type normalizedText struct {
	text   string
	runes  []rune
	starts []int
	ends   []int
}

func normalizeString(input string, cfg Config) normalizedText {
	if input == "" {
		return normalizedText{}
	}

	if cfg.Normalize == NormalizeNone {
		return normalizeByRuneSegments(input, cfg)
	}

	form := normForm(cfg.Normalize)
	var iter norm.Iter
	iter.InitString(form, input)

	var builder strings.Builder
	var runes []rune
	var starts []int
	var ends []int
	for !iter.Done() {
		start := iter.Pos()
		segment := string(iter.Next())
		end := iter.Pos()
		appendNormalizedSegment(&builder, &runes, &starts, &ends, segment, start, end, cfg)
	}
	return normalizedText{text: builder.String(), runes: runes, starts: starts, ends: ends}
}

func normalizeByRuneSegments(input string, cfg Config) normalizedText {
	var builder strings.Builder
	runes := make([]rune, 0, utf8.RuneCountInString(input))
	starts := make([]int, 0, cap(runes))
	ends := make([]int, 0, cap(runes))
	for start, r := range input {
		end := start + utf8.RuneLen(r)
		if r == utf8.RuneError {
			_, width := utf8.DecodeRuneInString(input[start:])
			end = start + width
		}
		appendNormalizedSegment(&builder, &runes, &starts, &ends, string(r), start, end, cfg)
	}
	return normalizedText{text: builder.String(), runes: runes, starts: starts, ends: ends}
}

func appendNormalizedSegment(builder *strings.Builder, runes *[]rune, starts *[]int, ends *[]int, segment string, start int, end int, cfg Config) {
	if cfg.IgnoreCase {
		segment = strings.ToLower(segment)
	}
	for _, r := range segment {
		builder.WriteRune(r)
		*runes = append(*runes, r)
		*starts = append(*starts, start)
		*ends = append(*ends, end)
	}
}

func normForm(mode NormalizeMode) norm.Form {
	switch mode {
	case NormalizeNFC:
		return norm.NFC
	case NormalizeNFKC:
		return norm.NFKC
	default:
		return norm.NFC
	}
}

func normalizedPatternText(pattern Pattern, index int, cfg Config) (compiledPattern, error) {
	if pattern.Text == "" {
		return compiledPattern{}, ErrEmptyPattern
	}
	if pattern.ID == "" {
		pattern.ID = strconv.Itoa(index)
	}
	normalized := normalizeString(pattern.Text, cfg)
	if len(normalized.runes) == 0 {
		return compiledPattern{}, ErrEmptyPattern
	}
	return compiledPattern{
		Pattern: pattern,
		runes:   normalized.runes,
		length:  len(normalized.runes),
		order:   index,
	}, nil
}
