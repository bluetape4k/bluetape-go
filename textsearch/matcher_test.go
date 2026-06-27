package textsearch_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/textsearch"
)

func TestFindAllReturnsOverlappingMatches(t *testing.T) {
	matcher, err := textsearch.CompileStrings([]string{"he", "she", "his", "hers"}, textsearch.Config{})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}

	matches := matcher.FindAll("ushers")
	got := matchPairs(matches)
	want := []string{
		"1:4:she:she",
		"2:6:hers:hers",
		"2:4:he:he",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches = %#v, want %#v", got, want)
	}

	first, ok := matcher.First("ushers")
	if !ok {
		t.Fatal("expected first match")
	}
	if first.Pattern.Text != "she" || first.Start != 1 || first.End != 4 {
		t.Fatalf("first = %+v", first)
	}
	if !matcher.Contains("ushers") {
		t.Fatal("expected Contains to report true")
	}
}

func TestFindAllCanSelectLeftmostLongest(t *testing.T) {
	matcher, err := textsearch.CompileStrings([]string{"a", "ab", "bc"}, textsearch.Config{
		Overlap: textsearch.OverlapLeftmostLongest,
	})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}

	got := matchPairs(matcher.FindAll("abc"))
	want := []string{"0:2:ab:ab"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches = %#v, want %#v", got, want)
	}
}

func TestDuplicatePatternsKeepStablePatternIDs(t *testing.T) {
	matcher, err := textsearch.Compile([]textsearch.Pattern{
		{ID: "first", Text: "go"},
		{ID: "second", Text: "go"},
	}, textsearch.Config{})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	matches := matcher.FindAll("gogo")
	got := make([]string, len(matches))
	for i, match := range matches {
		got[i] = fmt.Sprintf("%s@%d", match.Pattern.ID, match.Start)
	}
	want := []string{"first@0", "second@0", "first@2", "second@2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicate matches = %#v, want %#v", got, want)
	}
}

func TestCompileRejectsEmptyDictionaryAndPatterns(t *testing.T) {
	if _, err := textsearch.CompileStrings(nil, textsearch.Config{}); !errors.Is(err, textsearch.ErrNoPatterns) {
		t.Fatalf("expected ErrNoPatterns, got %v", err)
	}
	if _, err := textsearch.CompileStrings([]string{""}, textsearch.Config{}); !errors.Is(err, textsearch.ErrEmptyPattern) {
		t.Fatalf("expected ErrEmptyPattern, got %v", err)
	}
}

func TestNilMatcherMethodsAreSafe(t *testing.T) {
	var matcher *textsearch.Matcher
	if matcher.Contains("input") {
		t.Fatal("nil matcher should not contain matches")
	}
	if match, ok := matcher.First("input"); ok || match != (textsearch.Match{}) {
		t.Fatalf("nil matcher First = %+v ok=%v", match, ok)
	}
	if matches := matcher.FindAll("input"); matches != nil {
		t.Fatalf("nil matcher FindAll = %#v, want nil", matches)
	}
	replaced := matcher.Replace("input", func(textsearch.Match) string {
		return "x"
	})
	if replaced != "input" {
		t.Fatalf("nil matcher Replace = %q", replaced)
	}
	if masked := matcher.Mask("input", '*'); masked != "input" {
		t.Fatalf("nil matcher Mask = %q", masked)
	}
}

func TestNormalizationAndCaseHandlingPreserveOriginalByteSpan(t *testing.T) {
	matcher, err := textsearch.CompileStrings([]string{"CAFÉ"}, textsearch.Config{
		IgnoreCase: true,
		Normalize:  textsearch.NormalizeNFC,
	})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}

	input := "x cafe\u0301 y"
	match, ok := matcher.First(input)
	if !ok {
		t.Fatal("expected normalized match")
	}
	if match.Text != "cafe\u0301" {
		t.Fatalf("match text = %q", match.Text)
	}
	if input[match.Start:match.End] != match.Text {
		t.Fatalf("byte span did not slice original input: %+v", match)
	}
}

func TestBoundaryModes(t *testing.T) {
	matcher, err := textsearch.CompileStrings([]string{"cat"}, textsearch.Config{
		Boundary: textsearch.BoundaryASCIIWord,
	})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}

	got := matchPairs(matcher.FindAll("cat scatter bobcat cat!"))
	want := []string{"0:3:cat:cat", "19:22:cat:cat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("boundary matches = %#v, want %#v", got, want)
	}
}

func TestReplaceAndMaskUseDeterministicNonOverlappingMatches(t *testing.T) {
	matcher, err := textsearch.Compile([]textsearch.Pattern{
		{ID: "short", Text: "bad"},
		{ID: "long", Text: "badword"},
	}, textsearch.Config{})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	replaced := matcher.Replace("badword and bad", func(match textsearch.Match) string {
		return "[" + match.Pattern.ID + "]"
	})
	if replaced != "[long] and [short]" {
		t.Fatalf("replaced = %q", replaced)
	}

	masked := matcher.Mask("badword and bad", '*')
	if masked != "******* and ***" {
		t.Fatalf("masked = %q", masked)
	}
}

func TestLargeDictionary(t *testing.T) {
	patterns := make([]string, 1000)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("key-%04d", i)
	}
	matcher, err := textsearch.CompileStrings(patterns, textsearch.Config{})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}

	match, ok := matcher.First("prefix key-0999 suffix")
	if !ok || match.Pattern.Text != "key-0999" {
		t.Fatalf("unexpected large dictionary match: %+v ok=%v", match, ok)
	}
}

func TestMatcherConcurrentReads(t *testing.T) {
	matcher, err := textsearch.CompileStrings([]string{"alpha", "beta", "gamma", "alphabet"}, textsearch.Config{
		IgnoreCase: true,
		Overlap:    textsearch.OverlapLeftmostLongest,
	})
	if err != nil {
		t.Fatalf("CompileStrings failed: %v", err)
	}
	input := strings.Repeat("ALPHABET beta gamma ", 16)

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 64,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		matches := matcher.FindAll(input)
		if len(matches) == 0 || matches[0].Pattern.Text != "alphabet" {
			return fmt.Errorf("unexpected matches: %v", matchPairs(matches))
		}
		if !matcher.Contains(input) {
			return fmt.Errorf("expected Contains to report true")
		}
		return nil
	})
}

func matchPairs(matches []textsearch.Match) []string {
	result := make([]string, len(matches))
	for i, match := range matches {
		result[i] = fmt.Sprintf("%d:%d:%s:%s", match.Start, match.End, match.Pattern.Text, match.Text)
	}
	return result
}
