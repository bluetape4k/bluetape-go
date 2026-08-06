package postgrestestcontainer

import (
	"regexp"
	"testing"
)

func TestDefaultImageIsImmutable(t *testing.T) {
	const want = "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
	if defaultImage != want {
		t.Fatalf("defaultImage = %q, want %q", defaultImage, want)
	}
	if !regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`).MatchString(defaultImage) {
		t.Fatalf("defaultImage = %q, want immutable image reference", defaultImage)
	}
}
