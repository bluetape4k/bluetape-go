package redisvalue

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestReadmeContractMarkersStayInParity(t *testing.T) {
	expected := []string{
		"l1-boundary", "config", "ownership", "l1-provenance", "load-policy", "ttl", "errors",
		"clear", "topology", "operations", "versioning", "resp3", "tests",
		"untrusted-payload", "authentication", "namespace", "scan-bounds",
		"serializer-concurrency", "compatibility-matrix",
	}
	marker := regexp.MustCompile(`<!-- redisvalue-contract: ([a-z0-9-]+) -->`)
	heading := regexp.MustCompile(`(?m)^(#{1,3}) `)
	var headingLevels []int
	for _, path := range []string{"README.md", "README.ko.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		matches := marker.FindAllStringSubmatch(string(data), -1)
		got := make([]string, 0, len(matches))
		for _, match := range matches {
			got = append(got, match[1])
		}
		if !slices.Equal(got, expected) {
			t.Fatalf("%s markers = %v, want %v", path, got, expected)
		}
		headings := heading.FindAllStringSubmatch(string(data), -1)
		levels := make([]int, 0, len(headings))
		for _, match := range headings {
			levels = append(levels, len(match[1]))
		}
		if headingLevels == nil {
			headingLevels = levels
		} else if !slices.Equal(levels, headingLevels) {
			t.Fatalf("%s heading levels = %v, want %v", path, levels, headingLevels)
		}
		for _, required := range []string{
			"bluetape:cache:value:<namespace>:*",
			"DialTimeout",
			"ReadTimeout",
			"WriteTimeout",
			"PoolTimeout",
			"go-redis hooks",
			"cursor 0",
			"maxmemory",
			"correlation pseudonym",
			"dictionary",
		} {
			if !strings.Contains(string(data), required) {
				t.Fatalf("%s omitted operational contract %q", path, required)
			}
		}
	}
}
