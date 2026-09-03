package secretsmanager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeClient struct {
	mu     sync.Mutex
	output *awssm.GetSecretValueOutput
	err    error
	block  <-chan struct{}
	calls  int
	last   *awssm.GetSecretValueInput
	onCall func()
}

func (f *fakeClient) GetSecretValue(ctx context.Context, input *awssm.GetSecretValueInput, _ ...func(*awssm.Options)) (*awssm.GetSecretValueOutput, error) {
	f.mu.Lock()
	f.calls++
	f.last = cloneInput(input)
	output := cloneOutput(f.output)
	err := f.err
	block := f.block
	onCall := f.onCall
	f.mu.Unlock()
	if onCall != nil {
		onCall()
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return output, err
}

func (f *fakeClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeClient) input() *awssm.GetSecretValueInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneInput(f.last)
}

func cloneInput(input *awssm.GetSecretValueInput) *awssm.GetSecretValueInput {
	if input == nil {
		return nil
	}
	return &awssm.GetSecretValueInput{
		SecretId:     aws.String(aws.ToString(input.SecretId)),
		VersionId:    cloneString(input.VersionId),
		VersionStage: cloneString(input.VersionStage),
	}
}

func cloneOutput(output *awssm.GetSecretValueOutput) *awssm.GetSecretValueOutput {
	if output == nil {
		return nil
	}
	return &awssm.GetSecretValueOutput{
		SecretString:  cloneString(output.SecretString),
		SecretBinary:  cloneBytes(output.SecretBinary),
		Name:          cloneString(output.Name),
		VersionId:     cloneString(output.VersionId),
		VersionStages: append([]string(nil), output.VersionStages...),
	}
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func TestProviderGetsStringAndDoesNotExposeOrAliasIt(t *testing.T) {
	secret := "db-password"
	fake := &fakeClient{output: &awssm.GetSecretValueOutput{SecretString: &secret}}
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}

	value, err := provider.Get(context.Background(), "prod/database")
	if err != nil {
		t.Fatal(err)
	}
	if !value.IsSet() || value.IsBinary() || value.Text() != secret || value.Len() != len(secret) {
		t.Fatalf("value = %#v, want set text value", value)
	}
	bytes := value.Bytes()
	bytes[0] = 'X'
	if value.Text() != secret {
		t.Fatal("Bytes returned an aliased secret")
	}
	if got := fmt.Sprintf("%+v %#v", value, value); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("value formatting leaked secret: %q", got)
	}
	input := fake.input()
	if input == nil || aws.ToString(input.SecretId) != "prod/database" {
		t.Fatalf("request = %#v", input)
	}
}

func TestProviderGetsBinaryAndEmptyStringValues(t *testing.T) {
	tests := []struct {
		name   string
		output *awssm.GetSecretValueOutput
		binary bool
		text   string
		bytes  []byte
	}{
		{name: "binary", output: &awssm.GetSecretValueOutput{SecretBinary: []byte{1, 2, 3}}, binary: true, bytes: []byte{1, 2, 3}},
		{name: "empty binary", output: &awssm.GetSecretValueOutput{SecretBinary: []byte{}}, binary: true, bytes: []byte{}},
		{name: "empty string", output: &awssm.GetSecretValueOutput{SecretString: aws.String("")}, text: "", bytes: []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(Options{Client: &fakeClient{output: tt.output}})
			if err != nil {
				t.Fatal(err)
			}
			value, err := provider.Get(context.Background(), "secret")
			if err != nil {
				t.Fatal(err)
			}
			if value.IsBinary() != tt.binary || value.Text() != tt.text || string(value.Bytes()) != string(tt.bytes) {
				t.Fatalf("value = %#v, want binary=%v text=%q bytes=%x", value, tt.binary, tt.text, tt.bytes)
			}
		})
	}
}

func TestProviderRejectsMalformedOutputWithoutCaching(t *testing.T) {
	for name, output := range map[string]*awssm.GetSecretValueOutput{
		"missing": {},
		"both":    {SecretString: aws.String("text"), SecretBinary: []byte{1}},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeClient{output: output}
			provider, err := New(Options{Client: fake, CacheTTL: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Get(context.Background(), "secret")
			if !errors.Is(err, ErrMalformedOutput) && !errors.Is(err, ErrMissingValue) {
				t.Fatalf("Get() error = %v", err)
			}
			if fake.count() != 1 {
				t.Fatalf("calls = %d, want 1", fake.count())
			}
		})
	}
}

func TestProviderCancellationWinsBeforeAndAfterLookup(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fake := &fakeClient{output: &awssm.GetSecretValueOutput{SecretString: aws.String("secret")}}
		provider, err := New(Options{Client: fake})
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.Get(ctx, "secret")
		if !errors.Is(err, context.Canceled) || fake.count() != 0 {
			t.Fatalf("Get() error/calls = %v/%d", err, fake.count())
		}
	})

	t.Run("after response", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		fake := &fakeClient{
			output: &awssm.GetSecretValueOutput{SecretString: aws.String("secret")},
			onCall: cancel,
		}
		provider, err := New(Options{Client: fake})
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.Get(ctx, "secret")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Get() error = %v, want context.Canceled", err)
		}
	})
}

func TestProviderWrapsLookupErrorWithoutSecretDetails(t *testing.T) {
	fake := &fakeClient{err: errors.New("provider secret=super-sensitive")}
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Get(context.Background(), "secret-name")
	if !errors.Is(err, ErrLookupFailed) || !stringsNotContains(err.Error(), "super-sensitive") || !stringsNotContains(fmt.Sprintf("%+v", err), "secret-name") {
		t.Fatalf("redaction/error matching failed: %v", err)
	}
}

func TestProviderPositiveTTLCachesOnlySuccessfulValues(t *testing.T) {
	secret := "cached"
	fake := &fakeClient{output: &awssm.GetSecretValueOutput{SecretString: &secret}}
	provider, err := New(Options{Client: fake, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		value, getErr := provider.Get(context.Background(), "secret")
		if getErr != nil || value.Text() != secret {
			t.Fatalf("Get() = %#v, %v", value, getErr)
		}
	}
	if fake.count() != 1 {
		t.Fatalf("cached calls = %d, want 1", fake.count())
	}
}

func TestProviderDoesNotCacheLookupErrors(t *testing.T) {
	fake := &fakeClient{err: errors.New("temporary secret failure")}
	provider, err := New(Options{Client: fake, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, getErr := provider.Get(context.Background(), "secret"); !errors.Is(getErr, ErrLookupFailed) {
			t.Fatalf("Get() error = %v", getErr)
		}
	}
	if fake.count() != 2 {
		t.Fatalf("error was cached; calls = %d", fake.count())
	}
}

func TestProviderConcurrentCacheLoadsShareSuccessfulResult(t *testing.T) {
	secret := "concurrent"
	fake := &fakeClient{output: &awssm.GetSecretValueOutput{SecretString: &secret}}
	provider, err := New(Options{Client: fake, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	const callers = 32
	var group sync.WaitGroup
	group.Add(callers)
	for range callers {
		go func() {
			defer group.Done()
			value, getErr := provider.Get(context.Background(), "secret")
			if getErr != nil || value.Text() != secret {
				t.Errorf("Get() = %#v, %v", value, getErr)
			}
		}()
	}
	group.Wait()
	if fake.count() != 1 {
		t.Fatalf("concurrent cache loads = %d, want 1", fake.count())
	}
}

func TestNewRejectsNilClientAndNegativeTTL(t *testing.T) {
	var typedNil *fakeClient
	if _, err := New(Options{Client: typedNil}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("typed nil error = %v", err)
	}
	if _, err := New(Options{Client: &fakeClient{}, CacheTTL: -time.Second}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("negative ttl error = %v", err)
	}
}

func stringsNotContains(value, unwanted string) bool {
	return !contains(value, unwanted)
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
