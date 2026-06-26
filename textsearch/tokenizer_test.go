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

func TestSimpleTokenizerDeterministicUnicodeTokens(t *testing.T) {
	tokenizer := textsearch.NewSimpleTokenizer()
	request, err := textsearch.NewTokenizeRequest("Hello 세계 123! ホモ", textsearch.TokenizeOptions{})
	if err != nil {
		t.Fatalf("NewTokenizeRequest failed: %v", err)
	}

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	got := tokenPairs(response.Tokens)
	want := []string{
		"0:5:word:Hello:Hello",
		"6:12:word:세계:세계",
		"13:16:number:123:123",
		"16:17:punctuation:!:!",
		"18:24:word:ホモ:ホモ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(response.Texts(), []string{"Hello", "세계", "123", "!", "ホモ"}) {
		t.Fatalf("Texts = %#v", response.Texts())
	}
}

func TestSimpleTokenizerNormalizationAndWhitespace(t *testing.T) {
	tokenizer := textsearch.NewSimpleTokenizer()
	request, err := textsearch.NewTokenizeRequest("cafe\u0301 １２", textsearch.TokenizeOptions{
		Normalize:         textsearch.NormalizeNFKC,
		IncludeWhitespace: true,
	})
	if err != nil {
		t.Fatalf("NewTokenizeRequest failed: %v", err)
	}

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	got := tokenPairs(response.Tokens)
	want := []string{
		"0:6:word:café:café",
		"6:7:whitespace: : ",
		"7:13:number:１２:12",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(response.NormalizedTexts(), []string{"café", " ", "12"}) {
		t.Fatalf("NormalizedTexts = %#v", response.NormalizedTexts())
	}
}

func TestTokenizeValidationErrors(t *testing.T) {
	if _, err := textsearch.NewTokenizeRequest(" \t\n", textsearch.TokenizeOptions{}); !errors.Is(err, textsearch.ErrBlankTokenizeText) {
		t.Fatalf("expected ErrBlankTokenizeText, got %v", err)
	}
	tooLong := strings.Repeat("가", textsearch.MaxTokenizeTextLength+1)
	if _, err := textsearch.NewTokenizeRequest(tooLong, textsearch.TokenizeOptions{}); !errors.Is(err, textsearch.ErrTokenizeTextTooLong) {
		t.Fatalf("expected ErrTokenizeTextTooLong, got %v", err)
	}
}

func TestTokenizerFunc(t *testing.T) {
	tokenizer := textsearch.TokenizerFunc(func(request textsearch.TokenizeRequest) (textsearch.TokenizeResponse, error) {
		return textsearch.TokenizeResponse{Request: request, Tokens: []textsearch.Token{{Text: "ok"}}}, nil
	})
	request, err := textsearch.NewTokenizeRequest("ignored", textsearch.TokenizeOptions{})
	if err != nil {
		t.Fatalf("NewTokenizeRequest failed: %v", err)
	}

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if len(response.Tokens) != 1 || response.Tokens[0].Text != "ok" {
		t.Fatalf("response = %+v", response)
	}
}

func TestDictionaryProviderAndSetCopiesEntries(t *testing.T) {
	entries := []textsearch.DictionaryEntry{
		{ID: "ko", Text: "욕설", POS: textsearch.POSWord, Severity: textsearch.SeverityHigh, Metadata: map[string]string{"lang": "ko"}},
		{ID: "ja", Text: "ホモ", POS: textsearch.POSWord, Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"lang": "ja"}},
	}
	provider := textsearch.StaticDictionaryProvider(entries)

	loaded, err := provider.Entries(context.Background())
	if err != nil {
		t.Fatalf("Entries failed: %v", err)
	}
	loaded[0].Metadata["lang"] = "mutated"
	loaded[0].Text = "changed"

	loadedAgain, err := provider.Entries(context.Background())
	if err != nil {
		t.Fatalf("Entries second call failed: %v", err)
	}
	if loadedAgain[0].Text != "욕설" || loadedAgain[0].Metadata["lang"] != "ko" {
		t.Fatalf("provider leaked mutation: %#v", loadedAgain[0])
	}

	set, err := textsearch.NewDictionarySet(loadedAgain)
	if err != nil {
		t.Fatalf("NewDictionarySet failed: %v", err)
	}
	if !set.Contains("욕설") || set.Contains("missing") {
		t.Fatalf("unexpected contains result")
	}
	entry, ok := set.Entry("욕설")
	if !ok || entry.Normalized != "욕설" || entry.Metadata["lang"] != "ko" {
		t.Fatalf("entry = %#v ok=%v", entry, ok)
	}
	entry.Metadata["lang"] = "mutated"
	entryAgain, _ := set.Entry("욕설")
	if entryAgain.Metadata["lang"] != "ko" {
		t.Fatalf("set leaked mutation: %#v", entryAgain)
	}

	if _, err := textsearch.NewDictionarySet([]textsearch.DictionaryEntry{
		{Text: "same"},
		{Text: "same"},
	}); !errors.Is(err, textsearch.ErrDuplicateDictionaryText) {
		t.Fatalf("expected ErrDuplicateDictionaryText, got %v", err)
	}
}

func TestStaticDictionaryProviderHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := textsearch.StaticDictionaryProvider{{Text: "entry"}}

	if _, err := provider.Entries(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestSimpleTokenizerConcurrentReads(t *testing.T) {
	tokenizer := textsearch.NewSimpleTokenizer()
	request, err := textsearch.NewTokenizeRequest(strings.Repeat("Hello 세계 123! ", 8), textsearch.TokenizeOptions{
		Normalize: textsearch.NormalizeNFC,
	})
	if err != nil {
		t.Fatalf("NewTokenizeRequest failed: %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 64,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		response, err := tokenizer.Tokenize(request)
		if err != nil {
			return err
		}
		if len(response.Tokens) == 0 || response.Tokens[0].Text != "Hello" {
			return fmt.Errorf("unexpected tokens: %v", tokenPairs(response.Tokens))
		}
		return nil
	})
}

func tokenPairs(tokens []textsearch.Token) []string {
	result := make([]string, len(tokens))
	for i, token := range tokens {
		result[i] = fmt.Sprintf("%d:%d:%s:%s:%s", token.Span.Start, token.Span.End, token.POS, token.Text, token.Normalized)
	}
	return result
}
