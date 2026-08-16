package jwks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchReturnsBoundedBodyAndClosesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	provider, err := New(server.URL, WithMaxBodySize(1024))
	if err != nil {
		t.Fatal(err)
	}
	body, err := provider.fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch() error = %v", err)
	}
	if string(body) != `{"keys":[]}` {
		t.Fatalf("body = %q", body)
	}
}

func TestFetchRejectsNon200AndRedirectWithoutFollowing(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("redirect target was contacted")
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.fetch(context.Background())
	if !errors.Is(err, ErrFetch) {
		t.Fatalf("fetch() error = %v, want ErrFetch", err)
	}
	var fetchErr FetchError
	if !errors.As(err, &fetchErr) || fetchErr.Status != http.StatusFound {
		t.Fatalf("FetchError = %#v", fetchErr)
	}
}

func TestFetchPropagatesContextAndTimeoutErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	provider, err := New(server.URL, WithFetchTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.fetch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("fetch(canceled) error = %v", err)
	}
	if _, err := provider.fetch(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("fetch(timeout) error = %v", err)
	}
}

func TestFetchRejectsBodyLimitAndContentLengthEarly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer server.Close()
	provider, err := New(server.URL, WithMaxBodySize(10))
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.fetch(context.Background())
	if !errors.Is(err, ErrFetch) {
		t.Fatalf("fetch() error = %v, want ErrFetch", err)
	}
	var fetchErr FetchError
	if !errors.As(err, &fetchErr) || fetchErr.Class != FetchClassBody {
		t.Fatalf("FetchError = %#v", fetchErr)
	}
}
