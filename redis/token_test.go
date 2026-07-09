package btredis

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"testing"
)

func TestNewOwnerTokenReturnsCanonicalRedactedToken(t *testing.T) {
	token, err := NewOwnerToken()
	if err != nil {
		t.Fatalf("NewOwnerToken() error = %v", err)
	}
	if err := token.Validate(); err != nil {
		t.Fatalf("token.Validate() error = %v", err)
	}
	raw := token.RedisValue()
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(raw) {
		t.Fatalf("RedisValue() length = %d, want 64 lowercase hex characters", len(raw))
	}
	if got := token.String(); got == raw || got == "" {
		t.Fatalf("String() = %q, want non-empty redacted value different from raw token", got)
	}
	if printed := fmt.Sprint(token); printed == raw {
		t.Fatal("fmt.Sprint(token) leaked raw token")
	}
	if printed := fmt.Sprintf("%#v", token); contains(printed, raw) {
		t.Fatal("debug formatting leaked raw token")
	}
	if printed := fmt.Sprintf("%+v", token); contains(printed, raw) {
		t.Fatal("verbose formatting leaked raw token")
	}
	if printed := token.GoString(); contains(printed, raw) {
		t.Fatal("GoString leaked raw token")
	}
	if value := token.LogValue(); value.Kind() != slog.KindString || contains(value.String(), raw) {
		t.Fatal("slog LogValue leaked raw token")
	}
}

func TestParseOwnerTokenRejectsNonCanonicalValues(t *testing.T) {
	valid := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if token, err := ParseOwnerToken(valid); err != nil || token.RedisValue() != valid {
		t.Fatalf("ParseOwnerToken(valid) raw length = %d, has error = %t", len(token.RedisValue()), err != nil)
	}

	invalid := []string{
		"",
		" ",
		"0123456789abcdef",
		valid + "00",
		"0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	}
	for i, value := range invalid {
		t.Run(fmt.Sprintf("case-%02d", i), func(t *testing.T) {
			token, err := ParseOwnerToken(value)
			if err == nil {
				t.Fatalf("ParseOwnerToken invalid case %d returned raw length %d, nil error", i, len(token.RedisValue()))
			}
			if !errors.Is(err, ErrInvalidOwnerToken) {
				t.Fatalf("ParseOwnerToken invalid case %d sentinel match = false, want ErrInvalidOwnerToken", i)
			}
			if value != "" && value != " " && contains(err.Error(), value) {
				t.Fatalf("error leaked invalid token case %d", i)
			}
		})
	}
}

func TestOwnerTokenZeroValueInvalid(t *testing.T) {
	var token OwnerToken
	if err := token.Validate(); !errors.Is(err, ErrInvalidOwnerToken) {
		t.Fatalf("zero token Validate() = %v, want ErrInvalidOwnerToken", err)
	}
	if token.RedisValue() != "" {
		t.Fatalf("zero token RedisValue() = %q, want empty", token.RedisValue())
	}
}

func TestNewOwnerTokenConcurrentBoundedUniqueness(t *testing.T) {
	const goroutines = 16
	const perGoroutine = 32

	var wg sync.WaitGroup
	seen := make(chan string, goroutines*perGoroutine)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGoroutine {
				token, err := NewOwnerToken()
				if err != nil {
					t.Errorf("NewOwnerToken() error = %v", err)
					return
				}
				seen <- token.RedisValue()
			}
		}()
	}
	wg.Wait()
	close(seen)

	unique := make(map[string]struct{}, goroutines*perGoroutine)
	for raw := range seen {
		if _, exists := unique[raw]; exists {
			t.Fatalf("unexpected probabilistic token collision for token length %d", len(raw))
		}
		unique[raw] = struct{}{}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && regexp.MustCompile(regexp.QuoteMeta(sub)).FindStringIndex(s) != nil
}
