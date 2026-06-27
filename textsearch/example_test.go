package textsearch_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/textsearch"
)

func ExampleMatcher_FindAll() {
	matcher, err := textsearch.Compile([]textsearch.Pattern{
		{ID: "greeting", Text: "hello"},
		{ID: "subject", Text: "world"},
	}, textsearch.Config{IgnoreCase: true})
	if err != nil {
		return
	}

	for _, match := range matcher.FindAll("Hello, world!") {
		fmt.Println(match.Pattern.ID, match.Text)
	}

	// Output:
	// greeting Hello
	// subject world
}

func ExampleMatcher_Replace() {
	matcher, err := textsearch.CompileStrings([]string{"secret", "token"}, textsearch.Config{
		IgnoreCase: true,
	})
	if err != nil {
		return
	}

	fmt.Println(matcher.Replace("Secret token", func(match textsearch.Match) string {
		return "[" + match.Pattern.Text + "]"
	}))

	// Output:
	// [secret] [token]
}

func ExampleBlockwordDictionary_Process() {
	dictionary, err := textsearch.NewBlockwordDictionary([]textsearch.BlockwordEntry{
		{ID: "ko", Text: "욕설", Severity: textsearch.SeverityHigh},
		{ID: "ja", Text: "ホモ", Severity: textsearch.SeverityMiddle},
	}, textsearch.Config{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		return
	}
	request, err := textsearch.NewBlockwordRequest("욕설 그리고 ホモ", textsearch.BlockwordOptions{
		Mask:        "*",
		MinSeverity: textsearch.SeverityMiddle,
	})
	if err != nil {
		return
	}

	response, err := dictionary.Process(request)
	if err != nil {
		return
	}
	fmt.Println(response.MaskedText)
	fmt.Println(response.BlockwordExists())

	// Output:
	// ** 그리고 **
	// true
}
