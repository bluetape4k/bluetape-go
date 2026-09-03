package ssm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	awsssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/bluetape4k/bluetape-go/cache"
)

type fakeClient struct {
	mu     sync.Mutex
	output *awsssm.GetParameterOutput
	err    error
	calls  int
	last   *awsssm.GetParameterInput
	onCall func()
}

func (f *fakeClient) GetParameter(_ context.Context, input *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
	f.mu.Lock()
	f.calls++
	f.last = cloneInput(input)
	output := cloneOutput(f.output)
	err := f.err
	onCall := f.onCall
	f.mu.Unlock()
	if onCall != nil {
		onCall()
	}
	return output, err
}

func (f *fakeClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeClient) inputs() []*awsssm.GetParameterInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == nil {
		return nil
	}
	return []*awsssm.GetParameterInput{cloneInput(f.last)}
}

func cloneInput(input *awsssm.GetParameterInput) *awsssm.GetParameterInput {
	if input == nil {
		return nil
	}
	return &awsssm.GetParameterInput{Name: cloneString(input.Name), WithDecryption: cloneBool(input.WithDecryption)}
}

func cloneOutput(output *awsssm.GetParameterOutput) *awsssm.GetParameterOutput {
	if output == nil {
		return nil
	}
	if output.Parameter == nil {
		return &awsssm.GetParameterOutput{}
	}
	parameter := *output.Parameter
	parameter.Name = cloneString(parameter.Name)
	parameter.Value = cloneString(parameter.Value)
	parameter.Selector = cloneString(parameter.Selector)
	parameter.ARN = cloneString(parameter.ARN)
	parameter.DataType = cloneString(parameter.DataType)
	return &awsssm.GetParameterOutput{Parameter: &parameter}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func TestProviderGetsParameterAndRedactsValue(t *testing.T) {
	parameter := "db-password"
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: parameterPtr(parameter)}}
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	value, err := provider.Get(context.Background(), "/prod/database/password")
	if err != nil {
		t.Fatal(err)
	}
	if !value.IsSet() || value.Text() != parameter || value.IsBinary() {
		t.Fatalf("value = %#v", value)
	}
	bytes := value.Bytes()
	bytes[0] = 'X'
	if value.Text() != parameter {
		t.Fatal("Bytes returned an aliased parameter")
	}
	if got := fmt.Sprintf("%+v %#v", value, value); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("parameter formatting leaked value: %q", got)
	}
	input := fake.inputs()[0]
	if input == nil || aws.ToString(input.Name) != "/prod/database/password" || aws.ToBool(input.WithDecryption) {
		t.Fatalf("request = %#v", input)
	}
}

func TestProviderGetSecureForcesDecryption(t *testing.T) {
	parameter := "decrypted"
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: parameterPtr(parameter)}}
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.GetSecure(context.Background(), "secure"); err != nil {
		t.Fatal(err)
	}
	input := fake.inputs()[0]
	if input == nil || !aws.ToBool(input.WithDecryption) {
		t.Fatalf("secure request = %#v", input)
	}
}

func TestProviderConfiguredDecryptionAndCacheModesDoNotCollide(t *testing.T) {
	parameter := "value"
	parameterOutput := parameterValue(parameter)
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: &parameterOutput}}
	provider, err := New(Options{Client: fake, Cache: testCache(), CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Get(context.Background(), "param"); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.GetSecure(context.Background(), "param"); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 {
		t.Fatalf("cache modes collided; calls = %d", fake.count())
	}
}

func TestProviderRejectsMissingParameterOutput(t *testing.T) {
	tests := map[string]*awsssm.GetParameterOutput{
		"nil output":    {},
		"nil parameter": {Parameter: nil},
		"nil value":     {Parameter: func() *awsssmtypes.Parameter { parameter := parameterValue(""); return &parameter }()},
	}
	for name, output := range tests {
		t.Run(name, func(t *testing.T) {
			if name == "nil value" {
				output.Parameter.Value = nil
			}
			provider, err := New(Options{Client: &fakeClient{output: output}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Get(context.Background(), "param")
			if !errors.Is(err, ErrMalformedOutput) && !errors.Is(err, ErrMissingValue) {
				t.Fatalf("Get() error = %v", err)
			}
		})
	}
}

func TestProviderCancellationWinsBeforeAndAfterLookup(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: func() *awsssmtypes.Parameter { parameter := parameterValue("secret"); return &parameter }()}}
		provider, err := New(Options{Client: fake})
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.Get(ctx, "param")
		if !errors.Is(err, context.Canceled) || fake.count() != 0 {
			t.Fatalf("Get() error/calls = %v/%d", err, fake.count())
		}
	})

	t.Run("after response", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		fake := &fakeClient{
			output: &awsssm.GetParameterOutput{Parameter: func() *awsssmtypes.Parameter { parameter := parameterValue("secret"); return &parameter }()},
			onCall: cancel,
		}
		provider, err := New(Options{Client: fake})
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.Get(ctx, "param")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Get() error = %v, want context.Canceled", err)
		}
	})
}

func TestProviderWrapsLookupErrorWithoutParameterDetails(t *testing.T) {
	fake := &fakeClient{err: errors.New("provider parameter=super-sensitive")}
	provider, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Get(context.Background(), "parameter-name")
	if !errors.Is(err, ErrLookupFailed) || contains(err.Error(), "super-sensitive") || contains(fmt.Sprintf("%+v %#v", err, err), "parameter-name") || contains(fmt.Sprintf("%#v", err), "super-sensitive") {
		t.Fatalf("redaction/error matching failed: %v", err)
	}
}

func TestProviderPositiveTTLCachesOnlySuccessfulValues(t *testing.T) {
	parameter := "cached"
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: func() *awsssmtypes.Parameter { parameter := parameterValue(parameter); return &parameter }()}}
	provider, err := New(Options{Client: fake, Cache: testCache(), CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		value, getErr := provider.Get(context.Background(), "param")
		if getErr != nil || value.Text() != parameter {
			t.Fatalf("Get() = %#v, %v", value, getErr)
		}
	}
	if fake.count() != 1 {
		t.Fatalf("cached calls = %d, want 1", fake.count())
	}
}

func TestProviderPositiveTTLExpiresValues(t *testing.T) {
	parameter := "expiring"
	parameterOutput := parameterValue(parameter)
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: &parameterOutput}}
	provider, err := New(Options{Client: fake, Cache: testCache(), CacheTTL: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Get(context.Background(), "param"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err = provider.Get(context.Background(), "param"); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 {
		t.Fatalf("expired value calls = %d, want 2", fake.count())
	}
}

func TestProviderConcurrentCacheLoadsShareSuccessfulResult(t *testing.T) {
	parameter := "concurrent"
	parameterOutput := parameterValue(parameter)
	fake := &fakeClient{output: &awsssm.GetParameterOutput{Parameter: &parameterOutput}}
	provider, err := New(Options{Client: fake, Cache: testCache(), CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	const callers = 32
	var group sync.WaitGroup
	group.Add(callers)
	for range callers {
		go func() {
			defer group.Done()
			value, getErr := provider.Get(context.Background(), "param")
			if getErr != nil || value.Text() != parameter {
				t.Errorf("Get() = %#v, %v", value, getErr)
			}
		}()
	}
	group.Wait()
	if fake.count() != 1 {
		t.Fatalf("concurrent cache loads = %d, want 1", fake.count())
	}
}

func TestProviderDoesNotCacheLookupErrors(t *testing.T) {
	fake := &fakeClient{err: errors.New("temporary parameter failure")}
	provider, err := New(Options{Client: fake, Cache: testCache(), CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, getErr := provider.Get(context.Background(), "param"); !errors.Is(getErr, ErrLookupFailed) {
			t.Fatalf("Get() error = %v", getErr)
		}
	}
	if fake.count() != 2 {
		t.Fatalf("error was cached; calls = %d", fake.count())
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
	if _, err := New(Options{Client: &fakeClient{}, CacheTTL: time.Minute}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("positive ttl without cache error = %v", err)
	}
}

func testCache() cache.LoadingCache[string, Value] {
	return cache.NewMemory[string, Value]()
}

func parameterValue(value string) awsssmtypes.Parameter {
	return awsssmtypes.Parameter{Value: &value}
}

func parameterPtr(value string) *awsssmtypes.Parameter {
	parameter := parameterValue(value)
	return &parameter
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
