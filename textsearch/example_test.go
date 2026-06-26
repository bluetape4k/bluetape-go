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
