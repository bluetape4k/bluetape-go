package redisbloom

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

var redactedRedisKeyIDPattern = regexp.MustCompile(`^redis-key:[0-9a-f]{12}$`)

func TestKeyBuilderForNamespaceKeepsClusterHashTag(t *testing.T) {
	t.Parallel()

	builder, err := keyBuilderForNamespace(keyPrefix, "tenant-a:emails")
	if err != nil {
		t.Fatalf("keyBuilderForNamespace failed: %v", err)
	}
	key, err := builder.StructuralKey("bits")
	if err != nil {
		t.Fatalf("StructuralKey failed: %v", err)
	}
	if got, want := key.Value, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:bits"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}

func TestKeyBuilderForNamespaceRetainsLocalValidation(t *testing.T) {
	t.Parallel()

	for _, namespace := range []string{"tenant:{bad}", "   ", "tenant-secret"} {
		t.Run(namespace, func(t *testing.T) {
			t.Parallel()

			_, err := keyBuilderForNamespace(keyPrefix, namespace)
			if err == nil {
				t.Fatal("expected invalid namespace error")
			}
			for _, sharedError := range []string{"redis: invalid key", "invalid hash tag"} {
				if strings.Contains(err.Error(), sharedError) {
					t.Fatalf("shared key validation escaped: %v", err)
				}
			}
		})
	}
}

func TestBuildKeysUsesClusterHashTag(t *testing.T) {
	t.Parallel()

	keys, err := buildKeys("tenant-a:emails")
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}

	if keys.slot != "bluetape:probabilistic:bloom:v1:{tenant-a:emails}" {
		t.Fatalf("unexpected slot key: %q", keys.slot)
	}
	if keys.bits != "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:bits" {
		t.Fatalf("unexpected bits key: %q", keys.bits)
	}
	if keys.config != "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:config" {
		t.Fatalf("unexpected config key: %q", keys.config)
	}
	if strings.Contains(keys.redactedID, "tenant-a") {
		t.Fatalf("redacted id leaked namespace: %q", keys.redactedID)
	}
}

func TestBuildHyperLogLogKeyKeepsSharedBuilderCompatibleLayout(t *testing.T) {
	t.Parallel()

	key, err := buildHyperLogLogKey("tenant-a:emails")
	if err != nil {
		t.Fatalf("buildHyperLogLogKey failed: %v", err)
	}
	if got, want := key.key, "bluetape:probabilistic:hll:v1:{tenant-a:emails}"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}

	markerNamespace := "tenant-marker"
	markerKey, err := buildHyperLogLogKey(markerNamespace)
	if err != nil {
		t.Fatalf("buildHyperLogLogKey marker failed: %v", err)
	}
	if !redactedRedisKeyIDPattern.MatchString(markerKey.redactedID) {
		t.Fatalf("redacted ID = %q, want redis-key plus 12 lowercase hex chars", markerKey.redactedID)
	}
	for _, sensitive := range []string{markerNamespace, markerKey.key} {
		if strings.Contains(markerKey.redactedID, sensitive) {
			t.Fatalf("redacted ID leaked %q: %q", sensitive, markerKey.redactedID)
		}
	}
}

func TestNormalizeOptionsRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	cfg, hasher := validOptionsParts(t)
	_, err := normalizeOptions(Options[string]{
		Client:    stubCmdable{},
		Namespace: "raw@email.test",
		Config:    cfg,
		Hasher:    hasher,
	})

	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected ErrInvalidOptions, got %v", err)
	}
}

func TestNormalizeOptionsRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	validConfig, validHasher := validOptionsParts(t)
	tests := []struct {
		name    string
		options Options[string]
	}{
		{
			name: "nil client",
			options: Options[string]{
				Namespace: "tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "typed nil client",
			options: Options[string]{
				Client:    (*redis.Client)(nil),
				Namespace: "tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "invalid config",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    probabilistic.Config{},
				Hasher:    validHasher,
			},
		},
		{
			name: "empty hasher",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    validConfig,
			},
		},
		{
			name: "namespace leading whitespace",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: " tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "namespace trailing whitespace",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails ",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeOptions(tt.options)
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("expected ErrInvalidOptions, got %v", err)
			}
		})
	}
}

func TestValidateNamespaceRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("a", 129)
	for _, namespace := range []string{
		"",
		" tenant-a",
		"tenant-a ",
		":tenant-a",
		"tenant-a:",
		"tenant-a:bits",
		"tenant-a:config",
		"tenant{a}",
		"tenant-a:사용자",
		"alice@example.test",
		"token:secret",
		"password:reset",
		"secret:tenant-a",
		"credential:tenant-a",
		"api-key:tenant-a",
		"api_key:tenant-a",
		"api.key:tenant-a",
		"api:key:tenant-a",
		"apikey:tenant-a",
		tooLong,
	} {
		t.Run(namespace, func(t *testing.T) {
			t.Parallel()

			if err := validateNamespace(namespace); err == nil {
				t.Fatal("expected unsafe namespace to fail")
			}
		})
	}
}

func TestHasherKeyRejectsSensitiveOrUnsafeNames(t *testing.T) {
	t.Parallel()

	cfg, _ := validOptionsParts(t)
	for _, key := range []string{
		"",
		" schema:v1",
		"schema:v1 ",
		"schema\nv1",
		strings.Repeat("a", 129),
		"alice@example.test",
		"token:schema:v1",
		"secret:schema:v1",
		"password:schema:v1",
		"credential:schema:v1",
		"api-key:schema:v1",
		"api_key:schema:v1",
		"api.key:schema:v1",
		"api:key:schema:v1",
		"apikey:schema:v1",
	} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			hasher, _ := probabilistic.NewHasher(key, func(value string) []byte { return []byte(value) })

			_, err := normalizeOptions(Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    cfg,
				Hasher:    hasher,
			})
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("expected ErrInvalidOptions, got %v", err)
			}
		})
	}
}

func TestRedisErrorSupportsErrorsAs(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	err := RedisError{Operation: "put", KeyID: "redis-key:abc123", Err: cause}

	if !errors.Is(err, cause) {
		t.Fatalf("expected errors.Is to find cause, got %v", err)
	}
	var redisErr RedisError
	if !errors.As(err, &redisErr) {
		t.Fatalf("expected errors.As to find RedisError, got %v", err)
	}
	if strings.Contains(err.Error(), "tenant-a") {
		t.Fatalf("error leaked namespace: %v", err)
	}
}

func TestDefaultErrorsDoNotExposeSensitiveInputs(t *testing.T) {
	t.Parallel()

	err := RedisError{Operation: "put", KeyID: "redis-key:abc123", Err: errors.New("dial failure")}

	for _, sensitive := range []string{"tenant-a:emails", "bluetape:probabilistic", "probabilistic:string:v1", "inserted@example.test"} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("error leaked %q: %v", sensitive, err)
		}
	}
	if !strings.Contains(err.Error(), "redis-key:abc123") {
		t.Fatalf("error should contain redacted key id: %v", err)
	}
}

func TestMappedScriptErrorsDoNotExposeSensitiveInputs(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{
		errors.New("ERR config_mismatch"),
		errors.New("ERR config_corrupt"),
	} {
		err := mapScriptError("put", "redis-key:abc123", cause)

		for _, sensitive := range []string{"tenant-a:emails", "bluetape:probabilistic", "schema:v1", "inserted@example.test"} {
			if strings.Contains(err.Error(), sensitive) {
				t.Fatalf("error leaked %q: %v", sensitive, err)
			}
		}
		if !strings.Contains(err.Error(), "redis-key:abc123") {
			t.Fatalf("error should contain redacted key id: %v", err)
		}
	}
}

func TestUnrelatedRedisErrorsRemainRedisErrors(t *testing.T) {
	t.Parallel()

	cause := errors.New("network failure while reading config_mismatch telemetry")
	err := mapScriptError("put", "redis-key:abc123", cause)

	var redisErr RedisError
	if !errors.As(err, &redisErr) {
		t.Fatalf("expected RedisError, got %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected cause to be wrapped, got %v", err)
	}
}

func validOptionsParts(t *testing.T) (probabilistic.Config, probabilistic.Hasher[string]) {
	t.Helper()
	cfg, err := probabilistic.NewConfig(1000, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	hasher, err := probabilistic.NewHasher("schema:v1", func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	return cfg, hasher
}

type stubCmdable struct {
	redis.Cmdable
}
