package japanese_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

func ExampleTokenizer_Tokenize() {
	tokenizer, err := japanese.NewTokenizer()
	if err != nil {
		return
	}
	request, err := textsearch.NewTokenizeRequest("日本語を勉強します。", textsearch.TokenizeOptions{})
	if err != nil {
		return
	}

	response, err := tokenizer.Tokenize(request)
	if err != nil {
		return
	}
	for _, token := range japanese.FilterNouns(response.Tokens) {
		fmt.Println(token.Text, token.Metadata[japanese.MetadataPOS])
	}

	// Output:
	// 日本語 名詞/一般/*/*
	// 勉強 名詞/サ変接続/*/*
}

func ExampleTokenizer_blockwords() {
	tokenizer, err := japanese.NewTokenizer()
	if err != nil {
		return
	}
	request, err := textsearch.NewTokenizeRequest("お寿司が好きです。", textsearch.TokenizeOptions{})
	if err != nil {
		return
	}
	response, err := tokenizer.Tokenize(request)
	if err != nil {
		return
	}

	entries := make([]textsearch.BlockwordEntry, 0, len(response.Tokens))
	for _, token := range japanese.FilterNouns(response.Tokens) {
		entries = append(entries, textsearch.BlockwordEntry{ID: token.Text, Text: token.Text})
	}
	dictionary, err := textsearch.NewBlockwordDictionary(entries, textsearch.Config{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		return
	}
	masked, err := dictionary.Mask("寿司と刺身", textsearch.BlockwordOptions{Mask: "#"})
	if err != nil {
		return
	}
	fmt.Println(masked)

	// Output:
	// ##と刺身
}
