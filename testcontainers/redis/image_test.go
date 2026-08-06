package redistestcontainer

import (
	"regexp"
	"testing"
)

func TestDefaultImageIsImmutable(t *testing.T) {
	const want = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
	if defaultImage != want {
		t.Fatalf("defaultImage = %q, want %q", defaultImage, want)
	}
	if !regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`).MatchString(defaultImage) {
		t.Fatalf("defaultImage = %q, want immutable image reference", defaultImage)
	}
}
