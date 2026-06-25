package core_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestMatchWildcardString(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "exact match", pattern: "hello", value: "hello", want: true},
		{name: "exact mismatch", pattern: "hello", value: "world", want: false},
		{name: "empty pattern matches empty value", pattern: "", value: "", want: true},
		{name: "empty pattern rejects non-empty value", pattern: "", value: "hello", want: false},
		{name: "star matches zero runes", pattern: "hello*", value: "hello", want: true},
		{name: "star matches many runes", pattern: "h*o", value: "hello", want: true},
		{name: "star mismatch", pattern: "h*x", value: "hello", want: false},
		{name: "question matches one rune", pattern: "h?llo", value: "hello", want: true},
		{name: "question rejects missing rune", pattern: "??????", value: "hello", want: false},
		{name: "consecutive stars collapse", pattern: "h**o", value: "hello", want: true},
		{name: "escaped star is literal", pattern: `h\*llo`, value: "h*llo", want: true},
		{name: "escaped question is literal", pattern: `h\?llo`, value: "h?llo", want: true},
		{name: "escaped backslash is literal", pattern: `dir\\file`, value: `dir\file`, want: true},
		{name: "unicode question matches one rune", pattern: "안?요", value: "안녕요", want: true},
		{name: "unicode star matches runes", pattern: "안*요", value: "안녕하세요", want: true},
		{name: "case sensitive", pattern: "Hello", value: "hello", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := core.MatchWildcard(tt.pattern, tt.value)
			if err != nil {
				t.Fatalf("MatchWildcard returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("MatchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchWildcardRejectsTrailingEscape(t *testing.T) {
	if _, err := core.MatchWildcard(`hello\`, "hello"); err == nil {
		t.Fatal("MatchWildcard should reject a trailing escape")
	} else if !errors.Is(err, core.ErrMalformedWildcardPattern) {
		t.Fatalf("MatchWildcard error = %v, want ErrMalformedWildcardPattern", err)
	}
}

func TestFirstWildcardMatch(t *testing.T) {
	got, err := core.FirstWildcardMatch("hello.kt", "*.java", "*.kt", "*.py")
	if err != nil {
		t.Fatalf("FirstWildcardMatch returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("FirstWildcardMatch returned %d, want 1", got)
	}

	got, err = core.FirstWildcardMatch("hello.rs", "*.java", "*.kt", "*.py")
	if err != nil {
		t.Fatalf("FirstWildcardMatch returned error: %v", err)
	}
	if got != -1 {
		t.Fatalf("FirstWildcardMatch returned %d, want -1", got)
	}

	if _, err := core.FirstWildcardMatch("hello", `hel\`); err == nil {
		t.Fatal("FirstWildcardMatch should return malformed pattern errors")
	}
}

func TestMatchWildcardPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{name: "exact path", pattern: "src/main/Test.go", path: "src/main/Test.go", want: true},
		{name: "star segment", pattern: "src/*/Test.go", path: "src/main/Test.go", want: true},
		{name: "question in segment", pattern: "src/m?in/Test.go", path: "src/main/Test.go", want: true},
		{name: "deep wildcard matches zero segments", pattern: "src/**/Test.go", path: "src/Test.go", want: true},
		{name: "deep wildcard matches one segment", pattern: "src/**/Test.go", path: "src/main/Test.go", want: true},
		{name: "deep wildcard matches many segments", pattern: "src/**/*.go", path: "src/main/core/Test.go", want: true},
		{name: "deep wildcard does not cross filename pattern mismatch", pattern: "**/test/*.go", path: "src/test/core/Test.go", want: false},
		{name: "deep wildcard then segment", pattern: "**/test/**/*.go", path: "src/test/core/Test.go", want: true},
		{name: "windows separators in input path", pattern: "src/*/Test.go", path: `src\main\Test.go`, want: true},
		{name: "windows separators in pattern", pattern: `src\test\Foo.go`, path: "src/test/Foo.go", want: true},
		{name: "mixed separators in input path", pattern: "src/**/*.go", path: `src\main/core/Test.go`, want: true},
		{name: "escaped star in slash separated path pattern", pattern: `src/h\*llo/Test.go`, path: "src/h*llo/Test.go", want: true},
		{name: "case sensitive path", pattern: "src/**/test.go", path: "src/main/Test.go", want: false},
		{name: "double star only special as full segment", pattern: "src/**.go", path: "src/main.go", want: true},
		{name: "double star segment mismatch", pattern: "src/**.go", path: "src/main/test.go", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := core.MatchWildcardPath(tt.pattern, tt.path)
			if err != nil {
				t.Fatalf("MatchWildcardPath returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("MatchWildcardPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestFirstWildcardPathMatch(t *testing.T) {
	got, err := core.FirstWildcardPathMatch("src/test/Foo.go", "**/main/*.go", "**/test/*.go")
	if err != nil {
		t.Fatalf("FirstWildcardPathMatch returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("FirstWildcardPathMatch returned %d, want 1", got)
	}

	got, err = core.FirstWildcardPathMatch("build/classes/Foo.class", "**/main/*.go", "**/test/*.go")
	if err != nil {
		t.Fatalf("FirstWildcardPathMatch returned error: %v", err)
	}
	if got != -1 {
		t.Fatalf("FirstWildcardPathMatch returned %d, want -1", got)
	}

	got, err = core.FirstWildcardPathMatch("src/test/Foo.go", `src\test\Foo.go`)
	if err != nil {
		t.Fatalf("FirstWildcardPathMatch returned error: %v", err)
	}
	if got != 0 {
		t.Fatalf("FirstWildcardPathMatch returned %d, want 0", got)
	}
}
