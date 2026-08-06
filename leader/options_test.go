package leader_test

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestOptionsNormalizeRejectsUnsafeIdentity(t *testing.T) {
	tests := []struct {
		name string
		opts leader.Options
	}{
		{"group delimiter", leader.Options{Group: "a:b", MemberID: "m"}},
		{"member hash tag", leader.Options{Group: "g", MemberID: "m{1}"}},
		{"prefix empty segment", leader.Options{Group: "g", MemberID: "m", KeyPrefix: "a::b"}},
		{"control", leader.Options{Group: "g\n", MemberID: "m"}},
		{"group bytes", leader.Options{Group: strings.Repeat("g", 257), MemberID: "m"}},
		{"member bytes", leader.Options{Group: "g", MemberID: strings.Repeat("m", 257)}},
		{"final key bytes", leader.Options{Group: strings.Repeat("g", 256), MemberID: "m", KeyPrefix: strings.Repeat("p", 256)}},
		{"group leading space", leader.Options{Group: " g", MemberID: "m"}},
		{"member trailing space", leader.Options{Group: "g", MemberID: "m "}},
		{"prefix braced", leader.Options{Group: "g", MemberID: "m", KeyPrefix: "leader:{unsafe}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.opts.Normalize(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestOptionsNormalizePreservesValidIdentityBytes(t *testing.T) {
	values := []leader.Options{
		{Group: "그룹", MemberID: "멤버", KeyPrefix: "리더:선거"},
		{Group: "é", MemberID: "member"},
		{Group: "e\u0301", MemberID: "member"},
		{Group: strings.Repeat("g", 255), MemberID: strings.Repeat("m", 256), KeyPrefix: strings.Repeat("p", 256)},
	}
	for _, opts := range values {
		normalized, err := opts.Normalize()
		if err != nil {
			t.Fatalf("Normalize(%+v) error = %v", opts, err)
		}
		if normalized.Group != opts.Group || normalized.MemberID != opts.MemberID {
			t.Fatalf("Normalize mutated identity: got %+v, want %+v", normalized, opts)
		}
		if opts.KeyPrefix != "" && normalized.KeyPrefix != opts.KeyPrefix {
			t.Fatalf("Normalize mutated key prefix: got %q, want %q", normalized.KeyPrefix, opts.KeyPrefix)
		}
	}
}

func TestOptionsNormalizeAcceptsFinalKeyLength512(t *testing.T) {
	opts := leader.Options{
		Group:     strings.Repeat("g", 255),
		MemberID:  "member",
		KeyPrefix: strings.Repeat("p", 256),
	}
	if _, err := opts.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
}
