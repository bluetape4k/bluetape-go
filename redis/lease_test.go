package btredis

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
)

func TestNewLeaseValidatesKeyAndToken(t *testing.T) {
	token, err := NewOwnerToken()
	if err != nil {
		t.Fatalf("NewOwnerToken() error = %v", err)
	}
	if _, err := NewLease("", token); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("NewLease(empty key) error = %v, want ErrInvalidKey", err)
	}
	if _, err := NewLease("key", OwnerToken{}); !errors.Is(err, ErrInvalidOwnerToken) {
		t.Fatalf("NewLease(empty token) error = %v, want ErrInvalidOwnerToken", err)
	}

	lease, err := NewLease(" lock key ", token)
	if err != nil {
		t.Fatalf("NewLease(valid) error = %v", err)
	}
	if lease.Key() != " lock key " {
		t.Fatalf("Lease.Key() = %q, want exact key preservation", lease.Key())
	}
	if lease.Token().RedisValue() != token.RedisValue() {
		t.Fatal("Lease.Token() did not preserve owner token")
	}
	if lease.RedactedKeyID() != RedactedKeyID(" lock key ") {
		t.Fatalf("Lease.RedactedKeyID() = %q, want key redaction", lease.RedactedKeyID())
	}
}

func TestLeaseZeroValueInvalid(t *testing.T) {
	var lease Lease
	if err := lease.Validate(); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero Lease.Validate() error = %v, want ErrInvalidKey", err)
	}
}

func TestLeaseFormattingIsRedacted(t *testing.T) {
	token, err := NewOwnerToken()
	if err != nil {
		t.Fatalf("NewOwnerToken() error = %v", err)
	}
	lease, err := NewLease("tenant:secret:key", token)
	if err != nil {
		t.Fatalf("NewLease() error = %v", err)
	}

	for _, rendered := range []string{
		fmt.Sprint(lease),
		fmt.Sprintf("%+v", lease),
		fmt.Sprintf("%#v", lease),
		lease.String(),
		lease.GoString(),
		lease.LogValue().String(),
	} {
		if contains(rendered, lease.Key()) {
			t.Fatalf("Lease formatting leaked raw key: %q", rendered)
		}
		if contains(rendered, token.RedisValue()) {
			t.Fatalf("Lease formatting leaked raw token: %q", rendered)
		}
		if !contains(rendered, lease.RedactedKeyID()) {
			t.Fatalf("Lease formatting = %q, want redacted key id", rendered)
		}
	}
	if value := lease.LogValue(); value.Kind() != slog.KindString {
		t.Fatalf("Lease.LogValue() kind = %s, want string", value.Kind())
	}
}
