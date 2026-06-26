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

func TestBlockwordProcessDetectsAndMasksKoreanJapaneseAndASCII(t *testing.T) {
	dictionary := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{
		{ID: "ko", Text: "욕설", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"lang": "ko"}},
		{ID: "ja", Text: "ホモ", Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"lang": "ja"}},
		{ID: "en", Text: "badword", Severity: textsearch.SeverityLow, Metadata: map[string]string{"lang": "en"}},
	}, textsearch.Config{IgnoreCase: true})

	request, err := textsearch.NewBlockwordRequest("이 욕설과 ホモ 그리고 BADWORD", textsearch.BlockwordOptions{Mask: "#"})
	if err != nil {
		t.Fatalf("NewBlockwordRequest failed: %v", err)
	}
	response, err := dictionary.Process(request)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if !response.BlockwordExists() {
		t.Fatal("expected blockwords")
	}
	if response.MaskedText != "이 ##과 ## 그리고 #######" {
		t.Fatalf("MaskedText = %q", response.MaskedText)
	}
	gotWords := response.BlockWords()
	wantWords := []string{"욕설", "ホモ", "badword"}
	if !reflect.DeepEqual(gotWords, wantWords) {
		t.Fatalf("BlockWords = %#v, want %#v", gotWords, wantWords)
	}
	if response.Matches[0].Entry.Metadata["lang"] != "ko" {
		t.Fatalf("metadata was not preserved: %#v", response.Matches[0].Entry.Metadata)
	}
}

func TestBlockwordSeverityFiltersBeforeOverlapSelection(t *testing.T) {
	dictionary := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{
		{ID: "low-long", Text: "badword", Severity: textsearch.SeverityLow},
		{ID: "high-short", Text: "bad", Severity: textsearch.SeverityHigh},
	}, textsearch.Config{})

	matches := dictionary.Detect("badword", textsearch.BlockwordOptions{MinSeverity: textsearch.SeverityHigh})
	if len(matches) != 1 || matches[0].Entry.ID != "high-short" || matches[0].Text != "bad" {
		t.Fatalf("high severity matches = %#v", matches)
	}
}

func TestBlockwordNormalizationBoundaryAndNoMatchBehavior(t *testing.T) {
	dictionary := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{
		{Text: "CAFÉ", Severity: textsearch.SeverityLow},
		{Text: "cat", Severity: textsearch.SeverityLow},
	}, textsearch.Config{
		IgnoreCase: true,
		Normalize:  textsearch.NormalizeNFC,
		Boundary:   textsearch.BoundaryASCIIWord,
	})

	matches := dictionary.Detect("x cafe\u0301 scatter cat!", textsearch.BlockwordOptions{})
	got := blockwordPairs(matches)
	want := []string{"2:8:CAFÉ:café", "17:20:cat:cat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches = %#v, want %#v", got, want)
	}

	response, err := dictionary.Process(textsearch.BlockwordRequest{
		Text:    "clean text",
		Options: textsearch.BlockwordOptions{Mask: "*"},
	})
	if err != nil {
		t.Fatalf("Process no-match failed: %v", err)
	}
	if response.BlockwordExists() || response.MaskedText != "clean text" || len(response.Matches) != 0 {
		t.Fatalf("unexpected no-match response: %+v", response)
	}
}

func TestBlockwordDictionaryRebuildWorkflow(t *testing.T) {
	first := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{{Text: "first"}}, textsearch.Config{})
	second := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{{Text: "second"}}, textsearch.Config{})

	if matches := first.Detect("second", textsearch.BlockwordOptions{}); len(matches) != 0 {
		t.Fatal("first dictionary should not see rebuilt entries")
	}
	matches := second.Detect("second", textsearch.BlockwordOptions{})
	if len(matches) != 1 || matches[0].Entry.Text != "second" {
		t.Fatalf("second dictionary matches = %#v", matches)
	}
}

func TestBlockwordValidationErrors(t *testing.T) {
	if _, err := textsearch.NewBlockwordRequest(" \t\n", textsearch.BlockwordOptions{}); !errors.Is(err, textsearch.ErrBlankBlockwordText) {
		t.Fatalf("expected ErrBlankBlockwordText, got %v", err)
	}
	tooLong := strings.Repeat("가", textsearch.MaxBlockwordTextLength+1)
	if _, err := textsearch.NewBlockwordRequest(tooLong, textsearch.BlockwordOptions{}); !errors.Is(err, textsearch.ErrBlockwordTextTooLong) {
		t.Fatalf("expected ErrBlockwordTextTooLong, got %v", err)
	}
	if _, err := textsearch.NewBlockwordDictionary([]textsearch.BlockwordEntry{
		{ID: "same", Text: "one"},
		{ID: "same", Text: "two"},
	}, textsearch.Config{}); !errors.Is(err, textsearch.ErrDuplicatePatternID) {
		t.Fatalf("expected ErrDuplicatePatternID, got %v", err)
	}
}

func TestBlockwordDictionaryConcurrentReads(t *testing.T) {
	dictionary := mustBlockwordDictionary(t, []textsearch.BlockwordEntry{
		{Text: "욕설", Severity: textsearch.SeverityHigh},
		{Text: "ホモ", Severity: textsearch.SeverityMiddle},
		{Text: "badword", Severity: textsearch.SeverityLow},
	}, textsearch.Config{IgnoreCase: true})
	input := strings.Repeat("욕설 ホモ BADWORD clean ", 8)

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 64,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		response, err := dictionary.Process(textsearch.BlockwordRequest{
			Text:    input,
			Options: textsearch.BlockwordOptions{Mask: "*"},
		})
		if err != nil {
			return err
		}
		if !response.BlockwordExists() || strings.Contains(response.MaskedText, "BADWORD") {
			return fmt.Errorf("unexpected response: %+v", response)
		}
		return nil
	})
}

func mustBlockwordDictionary(t *testing.T, entries []textsearch.BlockwordEntry, cfg textsearch.Config) *textsearch.BlockwordDictionary {
	t.Helper()
	dictionary, err := textsearch.NewBlockwordDictionary(entries, cfg)
	if err != nil {
		t.Fatalf("NewBlockwordDictionary failed: %v", err)
	}
	return dictionary
}

func blockwordPairs(matches []textsearch.BlockwordMatch) []string {
	result := make([]string, len(matches))
	for i, match := range matches {
		result[i] = fmt.Sprintf("%d:%d:%s:%s", match.Start, match.End, match.Entry.Text, match.Text)
	}
	return result
}
