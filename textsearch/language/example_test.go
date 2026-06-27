package language_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

func ExampleDetector_Detect() {
	detector, err := language.NewDetector([]language.Language{
		language.English,
		language.German,
		language.Japanese,
	})
	if err != nil {
		return
	}

	result, err := detector.Detect("This text is written in English.")
	if err != nil {
		return
	}
	fmt.Println(result.Detected)
	fmt.Println(result.Language)

	// Output:
	// true
	// English
}

func ExampleContainsKorean() {
	fmt.Println(language.ContainsKorean("한국어 text"))
	fmt.Println(language.ContainsKorean("English text"))

	// Output:
	// true
	// false
}
