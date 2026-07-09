package japanese_test

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

var (
	benchmarkTokenizer         *japanese.Tokenizer
	benchmarkTokenizeResponse  textsearch.TokenizeResponse
	benchmarkTokenizeTokenList []textsearch.Token
)

func BenchmarkTokenizerConstruction(b *testing.B) {
	for _, tc := range []struct {
		name    string
		options []japanese.Option
	}{
		{name: "normal"},
		{name: "search", options: []japanese.Option{japanese.WithMode(japanese.Search)}},
		{name: "extended", options: []japanese.Option{japanese.WithMode(japanese.Extended)}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				tokenizer, err := japanese.NewTokenizer(tc.options...)
				if err != nil {
					b.Fatalf("NewTokenizer failed: %v", err)
				}
				benchmarkTokenizer = tokenizer
			}
		})
	}
}

func BenchmarkTokenizerConstructionAndFirstUse(b *testing.B) {
	for _, tc := range japaneseBenchmarkTextCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				tokenizer, err := japanese.NewTokenizer()
				if err != nil {
					b.Fatalf("NewTokenizer failed: %v", err)
				}
				response, err := tokenizer.Tokenize(tokenizeRequestForBenchmark(b, tc.text, tc.options))
				if err != nil {
					b.Fatalf("Tokenize failed: %v", err)
				}
				benchmarkTokenizer = tokenizer
				benchmarkTokenizeResponse = response
			}
		})
	}
}

func BenchmarkTokenizerTokenize(b *testing.B) {
	for _, mode := range []struct {
		name   string
		option japanese.Option
	}{
		{name: "normal", option: japanese.WithMode(japanese.Normal)},
		{name: "search", option: japanese.WithMode(japanese.Search)},
	} {
		tokenizer, err := japanese.NewTokenizer(mode.option)
		if err != nil {
			b.Fatalf("NewTokenizer(%s) failed: %v", mode.name, err)
		}
		for _, tc := range japaneseBenchmarkTextCases() {
			request := tokenizeRequestForBenchmark(b, tc.text, tc.options)
			if _, err := tokenizer.Tokenize(request); err != nil {
				b.Fatalf("warm Tokenize failed: %v", err)
			}
			b.Run(mode.name+"/"+tc.name, func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					response, err := tokenizer.Tokenize(request)
					if err != nil {
						b.Fatalf("Tokenize failed: %v", err)
					}
					benchmarkTokenizeResponse = response
				}
			})
		}
	}
}

func BenchmarkTokenizerFilterPOS(b *testing.B) {
	tokenizer, err := japanese.NewTokenizer(japanese.WithMode(japanese.Search))
	if err != nil {
		b.Fatalf("NewTokenizer failed: %v", err)
	}
	request := tokenizeRequestForBenchmark(b, japaneseMediumText(), textsearch.TokenizeOptions{})
	response, err := tokenizer.Tokenize(request)
	if err != nil {
		b.Fatalf("Tokenize failed: %v", err)
	}

	b.Run("nouns", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkTokenizeTokenList = japanese.FilterNouns(response.Tokens)
		}
	})
	b.Run("verbs", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkTokenizeTokenList = japanese.FilterVerbs(response.Tokens)
		}
	})
}

type japaneseBenchmarkTextCase struct {
	name    string
	text    string
	options textsearch.TokenizeOptions
}

func japaneseBenchmarkTextCases() []japaneseBenchmarkTextCase {
	return []japaneseBenchmarkTextCase{
		{name: "short", text: "お寿司が食べたい。"},
		{name: "medium", text: japaneseMediumText()},
		{name: "large", text: strings.Repeat(japaneseMediumText()+" ", 24)},
		{
			name: "nfkc_whitespace",
			text: "ｶﾀｶﾅ ガ " + japaneseMediumText(),
			options: textsearch.TokenizeOptions{
				Normalize:         textsearch.NormalizeNFKC,
				IncludeWhitespace: true,
			},
		},
	}
}

func japaneseMediumText() string {
	return "関西国際空港から東京駅まで電車で移動し、日本語の検索インデックスを作ります。"
}

func tokenizeRequestForBenchmark(b *testing.B, text string, options textsearch.TokenizeOptions) textsearch.TokenizeRequest {
	b.Helper()
	request, err := textsearch.NewTokenizeRequest(text, options)
	if err != nil {
		b.Fatalf("NewTokenizeRequest failed: %v", err)
	}
	return request
}
