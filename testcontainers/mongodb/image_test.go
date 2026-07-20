package mongodbtestcontainer

import (
	"regexp"
	"testing"
)

func TestDefaultImageIsImmutable(t *testing.T) {
	const want = "mongo:7.0@sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f"
	if defaultImage != want {
		t.Fatalf("defaultImage = %q, want %q", defaultImage, want)
	}
	if !regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`).MatchString(defaultImage) {
		t.Fatalf("defaultImage = %q, want immutable image reference", defaultImage)
	}
}
