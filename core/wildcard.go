package core

import (
	"errors"
	"fmt"
	"strings"
)

// ErrMalformedWildcardPattern 패키지에서 공개하는 변수 값이다.
var ErrMalformedWildcardPattern = errors.New("malformed wildcard pattern")

type wildcardTokenKind uint8

const (
	wildcardLiteral wildcardTokenKind = iota
	wildcardAny
	wildcardStar
)

type wildcardToken struct {
	kind wildcardTokenKind
	rune rune
}

// MatchWildcard 문자열이 wildcard pattern과 일치하는지 반환한다.
//
// 매개변수:
//   - pattern: MatchWildcard가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: MatchWildcard가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func MatchWildcard(pattern, value string) (bool, error) {
	tokens, err := parseWildcardPattern(pattern)
	if err != nil {
		return false, err
	}
	return matchWildcardTokens(tokens, []rune(value)), nil
}

// FirstWildcardMatch 처음 일치하는 wildcard pattern을 반환한다.
//
// 매개변수:
//   - value: FirstWildcardMatch가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - patterns: FirstWildcardMatch에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func FirstWildcardMatch(value string, patterns ...string) (int, error) {
	for i, pattern := range patterns {
		matched, err := MatchWildcard(pattern, value)
		if err != nil {
			return -1, err
		}
		if matched {
			return i, nil
		}
	}
	return -1, nil
}

// MatchWildcardPath path가 wildcard pattern과 일치하는지 반환한다.
//
// 매개변수:
//   - pattern: MatchWildcardPath가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - path: MatchWildcardPath가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func MatchWildcardPath(pattern, path string) (bool, error) {
	patternSegments := splitWildcardPatternPath(pattern)
	pathSegments := splitWildcardPath(path)
	return matchWildcardPathSegments(patternSegments, pathSegments)
}

// FirstWildcardPathMatch 처음 일치하는 wildcard path pattern을 반환한다.
//
// 매개변수:
//   - path: FirstWildcardPathMatch가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - patterns: FirstWildcardPathMatch에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func FirstWildcardPathMatch(path string, patterns ...string) (int, error) {
	for i, pattern := range patterns {
		matched, err := MatchWildcardPath(pattern, path)
		if err != nil {
			return -1, err
		}
		if matched {
			return i, nil
		}
	}
	return -1, nil
}

func parseWildcardPattern(pattern string) ([]wildcardToken, error) {
	runes := []rune(pattern)
	tokens := make([]wildcardToken, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '\\':
			if i+1 >= len(runes) {
				return nil, fmt.Errorf("%w: trailing escape", ErrMalformedWildcardPattern)
			}
			i++
			tokens = append(tokens, wildcardToken{kind: wildcardLiteral, rune: runes[i]})
		case '?':
			tokens = append(tokens, wildcardToken{kind: wildcardAny})
		case '*':
			if len(tokens) == 0 || tokens[len(tokens)-1].kind != wildcardStar {
				tokens = append(tokens, wildcardToken{kind: wildcardStar})
			}
		default:
			tokens = append(tokens, wildcardToken{kind: wildcardLiteral, rune: r})
		}
	}
	return tokens, nil
}

func matchWildcardTokens(tokens []wildcardToken, value []rune) bool {
	dp := make([]bool, len(value)+1)
	dp[0] = true

	for _, token := range tokens {
		next := make([]bool, len(value)+1)
		switch token.kind {
		case wildcardStar:
			next[0] = dp[0]
			for i := 1; i <= len(value); i++ {
				next[i] = dp[i] || next[i-1]
			}
		case wildcardAny:
			for i := 1; i <= len(value); i++ {
				next[i] = dp[i-1]
			}
		case wildcardLiteral:
			for i := 1; i <= len(value); i++ {
				next[i] = dp[i-1] && value[i-1] == token.rune
			}
		}
		dp = next
	}
	return dp[len(value)]
}

func splitWildcardPath(path string) []string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	segments := parts[:0]
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func splitWildcardPatternPath(pattern string) []string {
	var segments []string
	var current strings.Builder
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '/':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		case '\\':
			if i+1 < len(runes) && (runes[i+1] == '*' || runes[i+1] == '?' || runes[i+1] == '\\') {
				current.WriteRune(r)
				i++
				current.WriteRune(runes[i])
				continue
			}
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

func matchWildcardPathSegments(patternSegments, pathSegments []string) (bool, error) {
	dp := make([][]bool, len(patternSegments)+1)
	for i := range dp {
		dp[i] = make([]bool, len(pathSegments)+1)
	}
	dp[0][0] = true

	for i, segment := range patternSegments {
		if segment == "**" {
			dp[i+1][0] = dp[i][0]
			for j := 1; j <= len(pathSegments); j++ {
				dp[i+1][j] = dp[i][j] || dp[i+1][j-1]
			}
			continue
		}

		tokens, err := parseWildcardPattern(segment)
		if err != nil {
			return false, err
		}
		for j := 1; j <= len(pathSegments); j++ {
			dp[i+1][j] = dp[i][j-1] && matchWildcardTokens(tokens, []rune(pathSegments[j-1]))
		}
	}

	return dp[len(patternSegments)][len(pathSegments)], nil
}
