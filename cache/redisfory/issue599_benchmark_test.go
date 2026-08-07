package redisfory_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/fory/go/fory"
	btcache "github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/rediscoord"
	rediscoordfory "github.com/bluetape4k/bluetape-go/cache/rediscoord/fory"
	redisfory "github.com/bluetape4k/bluetape-go/cache/redisfory"
	"github.com/bluetape4k/bluetape-go/cache/redisvalue"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

const issue599TTL = time.Hour

type issue599Event struct {
	Kind     string
	Sequence int
	Payload  string
}

type issue599Value struct {
	ID      string
	Name    string
	Count   int
	Tags    []string
	Events  []issue599Event
	Payload []byte
}

type issue599Fixture struct {
	name  string
	value issue599Value
}

type issue599Codec interface {
	Marshal(issue599Value) ([]byte, error)
	Unmarshal([]byte) (issue599Value, error)
}

type issue599CodecProfile struct {
	name string
	new  func() (issue599Codec, error)
}

type issue599ValueCache interface {
	Set(context.Context, string, issue599Value, time.Duration) error
	Get(context.Context, string) (issue599Value, error)
	Delete(context.Context, string) error
}

var issue599CodecProfiles = []issue599CodecProfile{
	{name: "JSON", new: newIssue599JSONCodec},
	{name: "NativeFast", new: newIssue599NativeFastCodec},
	{name: "NativeCompatible", new: newIssue599NativeCompatibleCodec},
}

func registerIssue599Value(runtime *fory.Fory) error {
	if err := runtime.RegisterStructByName(issue599Event{}, "bluetape.go.issue599.Event"); err != nil {
		return err
	}
	return runtime.RegisterStructByName(issue599Value{}, "bluetape.go.issue599.Value")
}

func issue599Fixtures() []issue599Fixture {
	return []issue599Fixture{
		{name: "Small", value: issue599Value{
			ID: "issue-599-small", Name: "small", Count: 3,
			Tags:    []string{"cache", "fory"},
			Events:  []issue599Event{{Kind: "created", Sequence: 1, Payload: "small-event"}},
			Payload: issue599Payload(128),
		}},
		{name: "Medium", value: issue599Value{
			ID: "issue-599-medium", Name: "medium", Count: 17,
			Tags:    []string{"cache", "fory", "redis", "benchmark", "medium", "go", "schema", "value"},
			Events:  issue599Events(4),
			Payload: issue599Payload(4 << 10),
		}},
		{name: "Repeated", value: issue599Value{
			ID: "issue-599-repeated", Name: "repeated", Count: 64,
			Tags:    issue599RepeatedStrings("tag", 32),
			Events:  issue599Events(16),
			Payload: issue599Payload(4 << 10),
		}},
		{name: "Nil", value: issue599Value{
			ID: "issue-599-nil", Name: "nil", Count: 0,
		}},
		{name: "Empty", value: issue599Value{
			ID: "issue-599-empty", Name: "empty", Count: 0,
			Tags: []string{}, Events: []issue599Event{}, Payload: []byte{},
		}},
	}
}

func issue599Payload(size int) []byte {
	payload := make([]byte, size)
	for index := range payload {
		payload[index] = byte((index * 31) % 251)
	}
	return payload
}

func issue599Events(count int) []issue599Event {
	events := make([]issue599Event, count)
	for index := range events {
		events[index] = issue599Event{
			Kind:     "observed",
			Sequence: index + 1,
			Payload:  fmt.Sprintf("event-%02d", index+1),
		}
	}
	return events
}

func issue599RepeatedStrings(prefix string, count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = prefix
	}
	return values
}

func newIssue599JSONCodec() (issue599Codec, error) {
	return serialization.NewJSONSerializer[issue599Value](), nil
}

func newIssue599NativeFastCodec() (issue599Codec, error) {
	return rediscoordfory.NewNativeFast[issue599Value](rediscoordfory.Options{Register: registerIssue599Value})
}

func newIssue599NativeCompatibleCodec() (issue599Codec, error) {
	return rediscoordfory.NewNativeCompatible[issue599Value](rediscoordfory.Options{Register: registerIssue599Value})
}

func BenchmarkIssue599Codec(b *testing.B) {
	for _, profile := range issue599CodecProfiles {
		profile := profile
		b.Run(profile.name, func(b *testing.B) {
			for _, fixture := range issue599Fixtures() {
				fixture := fixture
				b.Run(fixture.name, func(b *testing.B) {
					codec, err := profile.new()
					if err != nil {
						b.Fatalf("new %s codec: %v", profile.name, err)
					}
					encoded, err := codec.Marshal(fixture.value)
					if err != nil {
						b.Fatalf("seed %s codec: %v", profile.name, err)
					}
					b.ReportAllocs()
					for _, operation := range []string{"Encode", "Decode", "RoundTrip"} {
						operation := operation
						b.Run(operation, func(b *testing.B) {
							b.ReportAllocs()
							b.ResetTimer()
							b.ReportMetric(float64(len(encoded)), "wire-bytes")
							switch operation {
							case "Encode":
								for range b.N {
									encodedValue, err := codec.Marshal(fixture.value)
									if err != nil {
										panic(err)
									}
									if len(encodedValue) == 0 {
										panic("codec returned an empty payload")
									}
								}
							case "Decode":
								for range b.N {
									decoded, err := codec.Unmarshal(encoded)
									if err != nil {
										panic(err)
									}
									if decoded.ID != fixture.value.ID {
										panic("codec returned the wrong value")
									}
								}
							case "RoundTrip":
								for range b.N {
									encodedValue, err := codec.Marshal(fixture.value)
									if err != nil {
										panic(err)
									}
									decoded, err := codec.Unmarshal(encodedValue)
									if err != nil {
										panic(err)
									}
									if decoded.ID != fixture.value.ID {
										panic("codec returned the wrong value")
									}
								}
							}
						})
					}
				})
			}
		})
	}
}

func BenchmarkIssue599Contention(b *testing.B) {
	fixture := issue599Fixtures()[0].value
	for _, tc := range []struct {
		name string
		pool bool
	}{
		{name: "Mutex", pool: false},
		{name: "Pool", pool: true},
	} {
		tc := tc
		b.Run("NativeFast/"+tc.name, func(b *testing.B) {
			shared, err := newIssue599NativeFastCodec()
			if err != nil {
				b.Fatalf("new native-fast codec: %v", err)
			}
			encoded, err := shared.Marshal(fixture)
			if err != nil {
				b.Fatalf("seed native-fast codec: %v", err)
			}
			b.ReportAllocs()

			var codecs sync.Pool
			if tc.pool {
				codecs.New = func() any {
					codec, err := newIssue599NativeFastCodec()
					if err != nil {
						panic(err)
					}
					return codec
				}
				for range runtime.GOMAXPROCS(0) * 4 {
					codecs.Put(codecs.New())
				}
			}
			b.ResetTimer()
			b.ReportMetric(float64(len(encoded)), "wire-bytes")
			b.RunParallel(func(pb *testing.PB) {
				codec := shared
				if tc.pool {
					codec = codecs.Get().(issue599Codec)
					defer codecs.Put(codec)
				}
				for pb.Next() {
					encodedValue, err := codec.Marshal(fixture)
					if err != nil {
						panic(err)
					}
					decoded, err := codec.Unmarshal(encodedValue)
					if err != nil {
						panic(err)
					}
					if decoded.ID != fixture.ID {
						panic("codec returned the wrong value")
					}
				}
			})
		})
	}
}

func BenchmarkIssue599DirectRedis(b *testing.B) {
	client := newIssue599RedisClient(b)
	ctx := context.Background()
	for _, profile := range issue599CodecProfiles {
		profile := profile
		b.Run(profile.name, func(b *testing.B) {
			for _, fixture := range issue599Fixtures()[:2] {
				fixture := fixture
				b.Run(fixture.name, func(b *testing.B) {
					cache, redisKey, cleanup := newIssue599DirectCache(b, client, profile.name, fixture.name)
					defer cleanup()
					key := "value"
					if err := cache.Set(ctx, key, fixture.value, issue599TTL); err != nil {
						b.Fatalf("seed %s cache: %v", profile.name, err)
					}
					wireBytes := issue599RedisWireBytes(b, client, redisKey)
					b.ReportAllocs()
					b.ReportMetric(float64(wireBytes), "wire-bytes")
					for _, operation := range []string{"Set", "Get", "RoundTrip"} {
						operation := operation
						b.Run(operation, func(b *testing.B) {
							b.ReportAllocs()
							b.ResetTimer()
							b.ReportMetric(float64(wireBytes), "wire-bytes")
							for range b.N {
								switch operation {
								case "Set":
									if err := cache.Set(ctx, key, fixture.value, issue599TTL); err != nil {
										panic(err)
									}
								case "Get":
									value, err := cache.Get(ctx, key)
									if err != nil {
										panic(err)
									}
									if value.ID != fixture.value.ID {
										panic("cache returned the wrong value")
									}
								case "RoundTrip":
									if err := cache.Set(ctx, key, fixture.value, issue599TTL); err != nil {
										panic(err)
									}
									value, err := cache.Get(ctx, key)
									if err != nil {
										panic(err)
									}
									if value.ID != fixture.value.ID {
										panic("cache returned the wrong value")
									}
								}
							}
						})
					}
				})
			}
		})
	}
}

func BenchmarkIssue599Coordination(b *testing.B) {
	client := newIssue599RedisClient(b)
	ctx := context.Background()
	fixture := issue599Fixtures()[0].value
	for _, profile := range issue599CodecProfiles {
		profile := profile
		b.Run(profile.name, func(b *testing.B) {
			codec, err := profile.new()
			if err != nil {
				b.Fatalf("new %s coordination codec: %v", profile.name, err)
			}
			for _, scenario := range []string{"Hot", "ColdWinner"} {
				scenario := scenario
				b.Run(scenario, func(b *testing.B) {
					coordination, err := newIssue599CoordinationCache(client, profile.name)
					if err != nil {
						b.Fatalf("new %s coordination cache: %v", profile.name, err)
					}
					encoded, err := codec.Marshal(fixture)
					if err != nil {
						b.Fatalf("measure %s coordination payload: %v", profile.name, err)
					}
					b.ReportAllocs()
					key := "hot"
					if scenario == "Hot" {
						if err := coordination.Set(ctx, key, stringValue(fixture), issue599TTL); err != nil {
							b.Fatalf("seed hot coordination cache: %v", err)
						}
					}
					keys := make([]string, b.N)
					for index := range keys {
						keys[index] = fmt.Sprintf("cold-%d", index)
					}
					b.ResetTimer()
					b.ReportMetric(float64(len(encoded)), "wire-bytes")
					for index := range b.N {
						if scenario == "ColdWinner" {
							key = keys[index]
						}
						value, err := coordination.GetOrLoad(ctx, key, issue599TTL, func(context.Context, string) (stringValue, error) {
							return stringValue(fixture), nil
						})
						if err != nil {
							panic(err)
						}
						if value.ID != fixture.ID {
							panic("coordination cache returned the wrong value")
						}
					}
				})
			}
		})
	}
}

// stringValue is an alias used only to keep the coordination benchmark's
// loader type explicit while preserving the fixture fields in the benchmark.
type stringValue issue599Value

func newIssue599CoordinationCache(client *redis.Client, profile string) (*rediscoord.StampedeCache[stringValue], error) {
	var codec rediscoord.Codec[stringValue]
	var err error
	switch profile {
	case "JSON":
		codec = rediscoord.JSONCodec[stringValue]{}
	case "NativeFast":
		codec, err = rediscoordfory.NewNativeFast[stringValue](rediscoordfory.Options{Register: registerIssue599StringValue})
	case "NativeCompatible":
		codec, err = rediscoordfory.NewNativeCompatible[stringValue](rediscoordfory.Options{Register: registerIssue599StringValue})
	default:
		return nil, fmt.Errorf("unknown coordination profile %q", profile)
	}
	if err != nil {
		return nil, err
	}
	return rediscoord.NewStampedeCache[stringValue](rediscoord.Options[stringValue]{
		Client:    client,
		Cache:     btcache.NewMemory[string, stringValue](),
		Namespace: "issue599-" + strings.ToLower(profile),
		Codec:     codec,
		LockTTL:   5 * time.Second,
		ResultTTL: time.Second,
	})
}

func registerIssue599StringValue(runtime *fory.Fory) error {
	if err := runtime.RegisterStructByName(issue599Event{}, "bluetape.go.issue599.Event"); err != nil {
		return err
	}
	return runtime.RegisterStructByName(stringValue{}, "bluetape.go.issue599.Value")
}

func newIssue599DirectCache(
	b testing.TB,
	client *redis.Client,
	profile string,
	fixture string,
) (issue599ValueCache, string, func()) {
	b.Helper()
	namespace := "issue599-" + strings.ToLower(profile) + "-" + strings.ToLower(fixture)
	var cache issue599ValueCache
	var err error
	switch profile {
	case "JSON":
		config := redisvalue.DefaultConfig().Value
		cache, err = redisvalue.NewValueCache(redisvalue.ValueOptions[issue599Value]{
			Client:     client,
			Namespace:  namespace,
			Serializer: serialization.NewJSONSerializer[issue599Value](),
			Config:     &config,
		})
	case "NativeFast":
		cache, err = redisfory.NewNativeFast[issue599Value](redisfory.Options{
			Client:           client,
			Namespace:        namespace,
			SchemaGeneration: 1,
			Register:         registerIssue599Value,
		})
	case "NativeCompatible":
		cache, err = redisfory.NewNativeCompatible[issue599Value](redisfory.Options{
			Client:           client,
			Namespace:        namespace,
			SchemaGeneration: 1,
			Register:         registerIssue599Value,
		})
	default:
		err = fmt.Errorf("unknown direct Redis profile %q", profile)
	}
	if err != nil {
		b.Fatalf("new %s direct Redis cache: %v", profile, err)
	}
	key := "value"
	if err := cache.Delete(context.Background(), key); err != nil {
		b.Fatalf("clear %s direct Redis cache: %v", profile, err)
	}
	redisKey := issue599DirectRedisKey(profile, namespace, key)
	return cache, redisKey, func() {
		if err := client.Del(context.Background(), redisKey).Err(); err != nil {
			b.Errorf("cleanup %s direct Redis key: %v", profile, err)
		}
	}
}

func issue599DirectRedisKey(profile, namespace, logicalKey string) string {
	prefix := "bluetape:cache:value"
	if profile != "JSON" {
		prefix = "bluetape:cache:fory"
	}
	builder, err := btredis.NewKeyBuilder(prefix)
	if err != nil {
		panic(err)
	}
	builder, err = builder.Structural(namespace)
	if err != nil {
		panic(err)
	}
	if profile != "JSON" {
		builder, err = builder.Structural("g1")
		if err != nil {
			panic(err)
		}
	}
	key, err := builder.LogicalKey(logicalKey)
	if err != nil {
		panic(err)
	}
	return key.Value
}

func issue599RedisWireBytes(b testing.TB, client *redis.Client, key string) int {
	b.Helper()
	wire, err := client.Get(context.Background(), key).Bytes()
	if err != nil && !errors.Is(err, redis.Nil) {
		b.Fatalf("measure Redis wire bytes: %v", err)
	}
	if len(wire) == 0 {
		return 0
	}
	return len(wire)
}

func newIssue599RedisClient(b testing.TB) *redis.Client {
	b.Helper()
	startupContext, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	address := redistestcontainer.Start(startupContext, b)
	client := redis.NewClient(&redis.Options{
		Addr:         address,
		PoolSize:     8,
		MinIdleConns: 1,
	})
	pingContext, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		b.Fatalf("ping Redis benchmark fixture: %v", err)
	}
	b.Cleanup(func() {
		if err := client.Close(); err != nil {
			b.Errorf("close Redis benchmark client: %v", err)
		}
	})
	return client
}

type issue599SchemaV1 struct {
	ID string
}

type issue599SchemaV2 struct {
	ID     string
	Region string
}

func TestIssue599NativeCompatibleSchemaEvolution(t *testing.T) {
	writer, err := rediscoordfory.NewNativeCompatible[issue599SchemaV1](rediscoordfory.Options{
		Register: func(runtime *fory.Fory) error {
			return runtime.RegisterStructByName(issue599SchemaV1{}, "bluetape.go.issue599.SchemaValue")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := rediscoordfory.NewNativeCompatible[issue599SchemaV2](rediscoordfory.Options{
		Register: func(runtime *fory.Fory) error {
			return runtime.RegisterStructByName(issue599SchemaV2{}, "bluetape.go.issue599.SchemaValue")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := writer.Marshal(issue599SchemaV1{ID: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := reader.Unmarshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "legacy" || decoded.Region != "" {
		t.Fatalf("decoded schema value = %#v", decoded)
	}
}

func TestIssue599MalformedPayloadRejected(t *testing.T) {
	value := issue599Fixtures()[0].value
	for _, profile := range issue599CodecProfiles[1:] {
		profile := profile
		t.Run(profile.name, func(t *testing.T) {
			codec, err := profile.new()
			if err != nil {
				t.Fatal(err)
			}
			payload, err := codec.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) < 2 {
				t.Fatalf("payload too short: %d", len(payload))
			}
			if _, err := codec.Unmarshal(payload[:len(payload)-1]); err == nil {
				t.Fatal("truncated payload was accepted")
			}
		})
	}
	jsonCodec := serialization.NewJSONSerializer[issue599Value]()
	if _, err := jsonCodec.Unmarshal([]byte("{")); err == nil {
		t.Fatal("malformed JSON payload was accepted")
	}
}

// Keep the JSON codec implementation in the benchmark file honest: the
// serializer must remain a plain encoding/json profile for the comparison.
var _ issue599Codec = serialization.NewJSONSerializer[issue599Value]()
