package language_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

func TestDetectorDetectsSubsetLanguages(t *testing.T) {
	detector := newDetector(t, []language.Language{language.English, language.German, language.Japanese, language.Korean})

	tests := []struct {
		name string
		text string
		want language.Language
	}{
		{name: "english", text: "Language detection works well for this short English sentence.", want: language.English},
		{name: "german", text: "Die Spracherkennung funktioniert fuer diesen deutschen Satz.", want: language.German},
		{name: "japanese", text: "これは日本語の文章です。", want: language.Japanese},
		{name: "korean", text: "이 문장은 한국어로 작성되었습니다.", want: language.Korean},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.Detect(tt.text)
			if err != nil {
				t.Fatalf("Detect failed: %v", err)
			}
			if !result.Detected || result.Language != tt.want {
				t.Fatalf("Detect = %+v, want %v", result, tt.want)
			}
			if result.Confidence <= 0 || result.ISO6391 == "" || result.ISO6393 == "" {
				t.Fatalf("incomplete result metadata: %+v", result)
			}
		})
	}
}

func TestDetectorSubsetValidationAndUnknownInput(t *testing.T) {
	if _, err := language.NewDetector([]language.Language{language.English}); !errors.Is(err, language.ErrLanguageSubsetTooSmall) {
		t.Fatalf("expected ErrLanguageSubsetTooSmall, got %v", err)
	}
	if _, err := language.NewDetector([]language.Language{language.English, language.Unknown}); !errors.Is(err, language.ErrLanguageSubsetTooSmall) {
		t.Fatalf("expected ErrLanguageSubsetTooSmall after unknown removal, got %v", err)
	}
	if _, err := language.NewDetector([]language.Language{language.English, language.German}, language.WithMinimumRelativeDistance(1)); !errors.Is(err, language.ErrInvalidMinimumRelativeDistance) {
		t.Fatalf("expected ErrInvalidMinimumRelativeDistance, got %v", err)
	}

	detector := newDetector(t, []language.Language{language.English, language.German})
	if _, err := detector.Detect(" \t\n"); !errors.Is(err, language.ErrBlankText) {
		t.Fatalf("expected ErrBlankText, got %v", err)
	}
	if _, err := detector.Detect(strings.Repeat("a", language.MaxTextLength+1)); !errors.Is(err, language.ErrTextTooLong) {
		t.Fatalf("expected ErrTextTooLong, got %v", err)
	}
	result, err := detector.Detect("1234567890")
	if err != nil {
		t.Fatalf("Detect unknown failed: %v", err)
	}
	if result.Detected || result.Language != language.Unknown {
		t.Fatalf("unknown result = %+v", result)
	}
}

func TestDetectorConfidencesAndLanguageCopies(t *testing.T) {
	detector := newDetector(t, []language.Language{language.English, language.German, language.French})
	selected := detector.Languages()
	selected[0] = language.Unknown
	if detector.Languages()[0] == language.Unknown {
		t.Fatal("Languages leaked internal slice")
	}

	confidences, err := detector.Confidences("This is a simple English sentence for confidence ranking.")
	if err != nil {
		t.Fatalf("Confidences failed: %v", err)
	}
	if len(confidences) != 3 {
		t.Fatalf("len(Confidences) = %d, want 3", len(confidences))
	}
	if confidences[0].Language != language.English || confidences[0].Value <= 0 {
		t.Fatalf("top confidence = %+v", confidences[0])
	}
	if confidences[0].ISO6391 == "" || confidences[0].ISO6393 == "" {
		t.Fatalf("confidence metadata missing: %+v", confidences[0])
	}
}

func TestDetectorDetectMultipleLanguages(t *testing.T) {
	detector := newDetector(t, []language.Language{language.English, language.German})
	text := "This sentence is English. Dieser Satz ist Deutsch."

	sections, err := detector.DetectMultiple(text)
	if err != nil {
		t.Fatalf("DetectMultiple failed: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least one section")
	}
	for _, section := range sections {
		if section.Text != text[section.Start:section.End] {
			t.Fatalf("section span %d:%d = %q, want %q", section.Start, section.End, text[section.Start:section.End], section.Text)
		}
		if section.Language == language.Unknown {
			t.Fatalf("unexpected unknown section: %+v", section)
		}
	}
}

func TestScriptHelpers(t *testing.T) {
	tests := []struct {
		name string
		text string
		fn   func(string) bool
	}{
		{name: "korean", text: "한국어", fn: language.ContainsKorean},
		{name: "japanese", text: "かなカナ", fn: language.ContainsJapanese},
		{name: "chinese", text: "中文", fn: language.ContainsChinese},
		{name: "thai", text: "ภาษาไทย", fn: language.ContainsThai},
		{name: "latin", text: "hello", fn: language.ContainsLatin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.fn(tt.text) {
				t.Fatalf("%s helper returned false for %q", tt.name, tt.text)
			}
			if tt.fn("12345!?") {
				t.Fatalf("%s helper returned true for punctuation", tt.name)
			}
		})
	}
}

func TestDetectorConcurrentReuse(t *testing.T) {
	detector := newDetector(t, []language.Language{language.English, language.German, language.Japanese, language.Korean})
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 32,
		Timeout:       5 * time.Second,
	})

	report := tester.RunT(t, func(context.Context) error {
		result, err := detector.Detect("Language detection works well for this English sentence.")
		if err != nil {
			return err
		}
		if !result.Detected || result.Language != language.English {
			return fmt.Errorf("unexpected result: %+v", result)
		}
		return nil
	})
	if report.MaxConcurrent < 2 {
		t.Fatalf("MaxConcurrent = %d, want concurrent execution", report.MaxConcurrent)
	}
}

func newDetector(t *testing.T, languages []language.Language) *language.Detector {
	t.Helper()
	detector, err := language.NewDetector(languages, language.WithLowAccuracyMode())
	if err != nil {
		t.Fatalf("NewDetector failed: %v", err)
	}
	return detector
}
