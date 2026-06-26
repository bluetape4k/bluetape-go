package japanese_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
	"github.com/ikawaha/kagome-dict/ipa"
)

func TestTokenizerTokenizesJapaneseWithByteSpansAndMetadata(t *testing.T) {
	tokenizer := newTokenizer(t)
	input := "お寿司が食べたい。"
	request := newRequest(t, input, textsearch.TokenizeOptions{})

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	wantTexts := []string{"お", "寿司", "が", "食べ", "たい", "。"}
	if got := response.Texts(); !reflect.DeepEqual(got, wantTexts) {
		t.Fatalf("Texts = %#v, want %#v", got, wantTexts)
	}
	for _, token := range response.Tokens {
		if got := input[token.Span.Start:token.Span.End]; got != token.Text {
			t.Fatalf("span %d:%d = %q, want %q", token.Span.Start, token.Span.End, got, token.Text)
		}
		if token.Metadata[japanese.MetadataLanguage] != "ja" {
			t.Fatalf("language metadata = %#v", token.Metadata)
		}
		if token.Metadata[japanese.MetadataDictionary] != ipa.DictName {
			t.Fatalf("dictionary metadata = %#v", token.Metadata)
		}
	}

	nouns := japanese.FilterNouns(response.Tokens)
	if got := tokenTexts(nouns); !reflect.DeepEqual(got, []string{"寿司"}) {
		t.Fatalf("FilterNouns = %#v", got)
	}
	verbs := japanese.FilterVerbs(response.Tokens)
	if got := tokenTexts(verbs); !reflect.DeepEqual(got, []string{"食べ"}) {
		t.Fatalf("FilterVerbs = %#v", got)
	}
	if verbs[0].Metadata[japanese.MetadataBaseForm] != "食べる" {
		t.Fatalf("verb metadata = %#v", verbs[0].Metadata)
	}
}

func TestTokenizerNormalizesTokenTextWithoutMovingOriginalSpans(t *testing.T) {
	tokenizer := newTokenizer(t)
	input := "ｶﾀｶﾅ ガ"
	request := newRequest(t, input, textsearch.TokenizeOptions{
		Normalize:         textsearch.NormalizeNFKC,
		IncludeWhitespace: true,
	})

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	var whitespaceSeen bool
	for _, token := range response.Tokens {
		if got := input[token.Span.Start:token.Span.End]; got != token.Text {
			t.Fatalf("span %d:%d = %q, want %q", token.Span.Start, token.Span.End, got, token.Text)
		}
		if strings.TrimSpace(token.Text) == "" {
			whitespaceSeen = true
		}
	}
	if !whitespaceSeen {
		t.Fatalf("expected whitespace token in %#v", response.Texts())
	}

	var foundHalfWidth bool
	for _, token := range response.Tokens {
		if token.Text == "ｶﾀｶﾅ" {
			foundHalfWidth = true
			if token.Normalized != "カタカナ" {
				t.Fatalf("Normalized = %q, want カタカナ", token.Normalized)
			}
		}
	}
	if !foundHalfWidth {
		t.Fatalf("half-width token missing from %#v", response.Tokens)
	}
}

func TestTokenizerDictionaryAndSearchModeOptions(t *testing.T) {
	tokenizer, err := japanese.NewTokenizer(
		japanese.WithDictionary(ipa.DictName, ipa.Dict()),
		japanese.WithMode(japanese.Search),
	)
	if err != nil {
		t.Fatalf("NewTokenizer failed: %v", err)
	}
	request := newRequest(t, "関西国際空港に行く", textsearch.TokenizeOptions{})

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if len(response.Tokens) == 0 {
		t.Fatal("expected tokens")
	}
	for _, token := range response.Tokens {
		if token.Metadata[japanese.MetadataDictionary] != ipa.DictName {
			t.Fatalf("dictionary metadata = %#v", token.Metadata)
		}
	}
}

func TestTokenizerOptionValidation(t *testing.T) {
	if _, err := japanese.NewTokenizer(japanese.WithDictionary("bad", nil)); err == nil {
		t.Fatal("expected nil dictionary error")
	}
	if _, err := japanese.NewTokenizer(japanese.WithDictionary(" ", ipa.Dict())); err == nil {
		t.Fatal("expected blank dictionary name error")
	}
	if _, err := japanese.NewTokenizer(japanese.WithMode(japanese.TokenizeMode(99))); err == nil {
		t.Fatal("expected unsupported mode error")
	}
}

func TestTokenizerConcurrentReads(t *testing.T) {
	tokenizer := newTokenizer(t)
	request := newRequest(t, strings.Repeat("日本語を勉強します。", 4), textsearch.TokenizeOptions{
		Normalize: textsearch.NormalizeNFC,
	})
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 64,
		Timeout:       3 * time.Second,
	})

	report := tester.RunT(t, func(context.Context) error {
		response, err := tokenizer.Tokenize(request)
		if err != nil {
			return err
		}
		if len(response.Tokens) == 0 || response.Tokens[0].Text != "日本語" {
			return fmt.Errorf("unexpected tokens: %v", response.Texts())
		}
		for _, token := range response.Tokens {
			if got := request.Text[token.Span.Start:token.Span.End]; got != token.Text {
				return fmt.Errorf("span %d:%d = %q, want %q", token.Span.Start, token.Span.End, got, token.Text)
			}
		}
		return nil
	})
	if report.MaxConcurrent < 2 {
		t.Fatalf("MaxConcurrent = %d, want concurrent execution", report.MaxConcurrent)
	}
}

func newTokenizer(t *testing.T) *japanese.Tokenizer {
	t.Helper()
	tokenizer, err := japanese.NewTokenizer()
	if err != nil {
		t.Fatalf("NewTokenizer failed: %v", err)
	}
	return tokenizer
}

func newRequest(t *testing.T, text string, options textsearch.TokenizeOptions) textsearch.TokenizeRequest {
	t.Helper()
	request, err := textsearch.NewTokenizeRequest(text, options)
	if err != nil {
		t.Fatalf("NewTokenizeRequest failed: %v", err)
	}
	return request
}

func tokenTexts(tokens []textsearch.Token) []string {
	texts := make([]string, len(tokens))
	for i, token := range tokens {
		texts[i] = token.Text
	}
	return texts
}
