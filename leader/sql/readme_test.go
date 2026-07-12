package sqlleader

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestREADMEContract(t *testing.T) {
	files := []string{"README.md", "README.ko.md"}
	anchors := []string{
		"DBStats.WaitCount",
		"DBStats.WaitDuration",
		"DBStats.InUse",
		"DBStats.MaxOpenConnections",
		"Lease-RenewInterval",
		"ErrCommitUnknown",
		"ErrCleanupPending",
		"pg_is_in_recovery()",
		"transaction_read_only",
		"full lease",
		"dead tuples",
		"autovacuum",
	}
	var baseline []string
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		for _, anchor := range anchors {
			if !strings.Contains(text, anchor) {
				t.Fatalf("%s is missing %q", file, anchor)
			}
		}
		headings := readmeHeadings(text)
		if baseline == nil {
			baseline = headings
		} else if !reflect.DeepEqual(headings, baseline) {
			t.Fatalf("README headings differ:\n%v\n%v", baseline, headings)
		}
	}
}

func readmeHeadings(contents string) []string {
	var headings []string
	for line := range strings.Lines(contents) {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line))
		}
	}
	return headings
}
