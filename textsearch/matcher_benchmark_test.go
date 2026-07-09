package textsearch_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch"
	cfaho "github.com/cloudflare/ahocorasick"
	rraho "github.com/rrethy/ahocorasick"
)

var (
	benchmarkBool           bool
	benchmarkMatch          textsearch.Match
	benchmarkMatches        []textsearch.Match
	benchmarkString         string
	benchmarkCloudflareIDs  []int
	benchmarkRRethyMatches  []*rraho.Match
	benchmarkMatcher        *textsearch.Matcher
	benchmarkCloudflareAho  *cfaho.Matcher
	benchmarkRRethyMatcher  *rraho.Matcher
	benchmarkBlockwordMatch []textsearch.BlockwordMatch
)

type matcherBenchmarkCase struct {
	name     string
	patterns []string
	input    string
	cfg      textsearch.Config
}

func BenchmarkMatcherCompile(b *testing.B) {
	for _, tc := range matcherBenchmarkCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				matcher, err := textsearch.CompileStrings(tc.patterns, tc.cfg)
				if err != nil {
					b.Fatalf("CompileStrings failed: %v", err)
				}
				benchmarkMatcher = matcher
			}
		})
	}
}

func BenchmarkMatcherContains(b *testing.B) {
	for _, tc := range matcherBenchmarkCases() {
		matcher := compileBenchmarkMatcher(b, tc.patterns, tc.cfg)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkBool = matcher.Contains(tc.input)
			}
		})
	}
}

func BenchmarkMatcherFirst(b *testing.B) {
	for _, tc := range matcherBenchmarkCases() {
		matcher := compileBenchmarkMatcher(b, tc.patterns, tc.cfg)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				match, ok := matcher.First(tc.input)
				benchmarkMatch = match
				benchmarkBool = ok
			}
		})
	}
}

func BenchmarkMatcherFindAll(b *testing.B) {
	for _, tc := range matcherBenchmarkCases() {
		matcher := compileBenchmarkMatcher(b, tc.patterns, tc.cfg)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkMatches = matcher.FindAll(tc.input)
			}
		})
	}
}

func BenchmarkMatcherReplace(b *testing.B) {
	patterns := []textsearch.Pattern{
		{ID: "long", Text: "badword"},
		{ID: "short", Text: "bad"},
		{ID: "secret", Text: "secret"},
		{ID: "token", Text: "token"},
	}
	matcher, err := textsearch.Compile(patterns, textsearch.Config{
		IgnoreCase: true,
		Overlap:    textsearch.OverlapLeftmostLongest,
	})
	if err != nil {
		b.Fatalf("Compile failed: %v", err)
	}
	input := strings.Repeat("tenant badword ok SECRET token clean ", 96)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkString = matcher.Replace(input, func(match textsearch.Match) string {
			return "[" + match.Pattern.ID + "]"
		})
	}
}

func BenchmarkMatcherMask(b *testing.B) {
	matcher := compileBenchmarkMatcher(b, []string{"badword", "secret", "token"}, textsearch.Config{
		IgnoreCase: true,
		Overlap:    textsearch.OverlapLeftmostLongest,
	})
	input := strings.Repeat("tenant badword ok SECRET token clean ", 96)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkString = matcher.Mask(input, '*')
	}
}

func BenchmarkBlockwordDictionaryProcess(b *testing.B) {
	entries := []textsearch.BlockwordEntry{
		{ID: "badword", Text: "badword", Severity: textsearch.SeverityHigh},
		{ID: "secret", Text: "secret", Severity: textsearch.SeverityMiddle},
		{ID: "token", Text: "token", Severity: textsearch.SeverityLow},
	}
	dictionary, err := textsearch.NewBlockwordDictionary(entries, textsearch.Config{IgnoreCase: true})
	if err != nil {
		b.Fatalf("NewBlockwordDictionary failed: %v", err)
	}
	request, err := textsearch.NewBlockwordRequest(strings.Repeat("tenant badword ok SECRET token clean ", 96), textsearch.BlockwordOptions{})
	if err != nil {
		b.Fatalf("NewBlockwordRequest failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		response, err := dictionary.Process(request)
		if err != nil {
			b.Fatalf("Process failed: %v", err)
		}
		benchmarkString = response.MaskedText
		benchmarkBlockwordMatch = response.Matches
	}
}

func BenchmarkAhoCorasickCandidateCompile(b *testing.B) {
	for _, tc := range rawCandidateBenchmarkCases() {
		b.Run("cloudflare/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkCloudflareAho = cfaho.NewStringMatcher(tc.patterns)
			}
		})
		b.Run("rrethy/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkRRethyMatcher = rraho.CompileStrings(tc.patterns)
			}
		})
	}
}

func BenchmarkAhoCorasickCandidateFindAll(b *testing.B) {
	for _, tc := range rawCandidateBenchmarkCases() {
		cloudflareMatcher := cfaho.NewStringMatcher(tc.patterns)
		rrethyMatcher := rraho.CompileStrings(tc.patterns)

		b.Run("cloudflare/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkCloudflareIDs = cloudflareMatcher.Match([]byte(tc.input))
			}
		})
		b.Run("rrethy/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchmarkRRethyMatches = rrethyMatcher.FindAllString(tc.input)
			}
		})
	}
}

func compileBenchmarkMatcher(b *testing.B, patterns []string, cfg textsearch.Config) *textsearch.Matcher {
	b.Helper()
	matcher, err := textsearch.CompileStrings(patterns, cfg)
	if err != nil {
		b.Fatalf("CompileStrings failed: %v", err)
	}
	return matcher
}

func matcherBenchmarkCases() []matcherBenchmarkCase {
	smallPatterns := []string{"bad", "secret", "token", "tenant"}
	mediumPatterns := generatedPatterns(128)
	largePatterns := generatedPatterns(2048)
	overlapPatterns := []string{"he", "she", "his", "hers", "hero", "her"}
	unicodePatterns := []string{"카페", "東京", "Café", "admin"}

	return []matcherBenchmarkCase{
		{
			name:     "small_success_contains",
			patterns: smallPatterns,
			input:    strings.Repeat("clean tenant payload with SECRET token ", 64),
			cfg:      textsearch.Config{IgnoreCase: true},
		},
		{
			name:     "medium_no_match_heavy",
			patterns: mediumPatterns,
			input:    strings.Repeat("clean payload without monitored markers ", 96),
			cfg:      textsearch.Config{},
		},
		{
			name:     "large_success_tail",
			patterns: largePatterns,
			input:    strings.Repeat("noise ", 128) + "key-2047",
			cfg:      textsearch.Config{},
		},
		{
			name:     "overlap_leftmost_longest",
			patterns: overlapPatterns,
			input:    strings.Repeat("ushers hero hers ", 96),
			cfg:      textsearch.Config{Overlap: textsearch.OverlapLeftmostLongest},
		},
		{
			name:     "unicode_nfkc_case",
			patterns: unicodePatterns,
			input:    strings.Repeat("prefix ｶﾌｪ and 東京 plus CAFE\u0301 ADMIN ", 48),
			cfg: textsearch.Config{
				IgnoreCase: true,
				Normalize:  textsearch.NormalizeNFKC,
			},
		},
	}
}

func rawCandidateBenchmarkCases() []matcherBenchmarkCase {
	mediumPatterns := generatedPatterns(128)
	largePatterns := generatedPatterns(2048)
	return []matcherBenchmarkCase{
		{
			name:     "small_success_contains",
			patterns: []string{"bad", "secret", "token", "tenant"},
			input:    strings.Repeat("clean tenant payload with secret token ", 64),
		},
		{
			name:     "medium_no_match_heavy",
			patterns: mediumPatterns,
			input:    strings.Repeat("clean payload without monitored markers ", 96),
		},
		{
			name:     "large_success_tail",
			patterns: largePatterns,
			input:    strings.Repeat("noise ", 128) + "key-2047",
		},
		{
			name:     "overlap_raw",
			patterns: []string{"he", "she", "his", "hers", "hero", "her"},
			input:    strings.Repeat("ushers hero hers ", 96),
		},
	}
}

func generatedPatterns(count int) []string {
	patterns := make([]string, count)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("key-%04d", i)
	}
	return patterns
}
