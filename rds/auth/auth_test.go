package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type fakeCredentials struct {
	mu         sync.Mutex
	value      aws.Credentials
	err        error
	calls      int
	onRetrieve func()
}

func (f *fakeCredentials) Retrieve(_ context.Context) (aws.Credentials, error) {
	f.mu.Lock()
	f.calls++
	value := f.value
	err := f.err
	onRetrieve := f.onRetrieve
	f.mu.Unlock()
	if onRetrieve != nil {
		onRetrieve()
	}
	return value, err
}

func (f *fakeCredentials) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func validCredentials() aws.Credentials {
	return aws.Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "secret-access-key",
		SessionToken:    "session-token",
		Source:          "fake",
	}
}

func TestBuildAuthTokenSupportsIPv4AndIPv6Endpoints(t *testing.T) {
	for _, endpoint := range []string{"db.example.com:5432", "127.0.0.1:3306", "[2001:db8::1]:5432"} {
		t.Run(endpoint, func(t *testing.T) {
			credentials := &fakeCredentials{value: validCredentials()}
			token, err := BuildAuthToken(context.Background(), Request{
				Endpoint: endpoint,
				Region:   "ap-northeast-2",
				Username: "app_user",
			}, credentials)
			if err != nil {
				t.Fatal(err)
			}
			if !token.IsSet() || token.Text() == "" || token.Len() == 0 || credentials.count() != 1 {
				t.Fatalf("token = %#v, calls = %d", token, credentials.count())
			}
			if got := fmt.Sprintf("%+v %#v", token, token); got != "[REDACTED] [REDACTED]" {
				t.Fatalf("token formatting leaked token: %q", got)
			}
		})
	}
}

func TestBuildAuthTokenRejectsMalformedRequestsBeforeCredentialLookup(t *testing.T) {
	endpoints := []string{
		"",
		"db.example.com",
		"https://db.example.com:5432",
		"db.example.com:5432/path",
		"db.example.com:5432?sslmode=require",
		"db.example.com:0",
		"db.example.com:65536",
		":5432",
		"2001:db8::1:5432",
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			credentials := &fakeCredentials{value: validCredentials()}
			_, err := BuildAuthToken(context.Background(), Request{Endpoint: endpoint, Region: "us-east-1", Username: "user"}, credentials)
			if !errors.Is(err, ErrInvalidRequest) || credentials.count() != 0 {
				t.Fatalf("error/calls = %v/%d", err, credentials.count())
			}
		})
	}

	for name, request := range map[string]Request{
		"blank region":   {Endpoint: "db.example.com:5432", Region: " ", Username: "user"},
		"blank username": {Endpoint: "db.example.com:5432", Region: "us-east-1", Username: "\t"},
		"invalid utf8":   {Endpoint: "db.example.com:5432", Region: string([]byte{0xff}), Username: "user"},
	} {
		t.Run(name, func(t *testing.T) {
			credentials := &fakeCredentials{value: validCredentials()}
			_, err := BuildAuthToken(context.Background(), request, credentials)
			if !errors.Is(err, ErrInvalidRequest) || credentials.count() != 0 {
				t.Fatalf("error/calls = %v/%d", err, credentials.count())
			}
		})
	}
}

func TestBuildAuthTokenRejectsNilAndTypedNilCredentials(t *testing.T) {
	if _, err := BuildAuthToken(context.Background(), validRequest(), nil); !errors.Is(err, ErrNilCredentials) {
		t.Fatalf("nil credentials error = %v", err)
	}
	var typedNil *fakeCredentials
	if _, err := BuildAuthToken(context.Background(), validRequest(), typedNil); !errors.Is(err, ErrNilCredentials) {
		t.Fatalf("typed nil credentials error = %v", err)
	}
}

func TestBuildAuthTokenCancellationWinsBeforeAndAfterSDKCall(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		credentials := &fakeCredentials{value: validCredentials()}
		_, err := BuildAuthToken(ctx, validRequest(), credentials)
		if !errors.Is(err, context.Canceled) || credentials.count() != 0 {
			t.Fatalf("error/calls = %v/%d", err, credentials.count())
		}
	})

	t.Run("after credential response", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		credentials := &fakeCredentials{value: validCredentials(), onRetrieve: cancel}
		_, err := BuildAuthToken(ctx, validRequest(), credentials)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})
}

func TestBuildAuthTokenWrapsCredentialErrorWithoutDetails(t *testing.T) {
	credentials := &fakeCredentials{err: errors.New("credential secret=super-sensitive")}
	_, err := BuildAuthToken(context.Background(), validRequest(), credentials)
	if !errors.Is(err, ErrBuildFailed) || strings.Contains(err.Error(), "super-sensitive") || strings.Contains(fmt.Sprintf("%+v", err), "secret-access-key") {
		t.Fatalf("redaction/error matching failed: %v", err)
	}
}

func TestTokenBytesAreIndependent(t *testing.T) {
	credentials := &fakeCredentials{value: validCredentials()}
	token, err := BuildAuthToken(context.Background(), validRequest(), credentials)
	if err != nil {
		t.Fatal(err)
	}
	bytes := token.Bytes()
	bytes[0] = 'X'
	if token.Text() == string(bytes) || token.Text() == "" {
		t.Fatal("Token.Bytes returned an aliased or empty token")
	}
}

func validRequest() Request {
	return Request{Endpoint: "db.example.com:5432", Region: "ap-northeast-2", Username: "app_user"}
}
