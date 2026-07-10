package rediscoordfory_test

import (
	"log"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/rediscoord"
	rediscoordfory "github.com/bluetape4k/bluetape-go/cache/rediscoord/fory"
	"github.com/redis/go-redis/v9"
)

type exampleValue struct{ Name string }

func registerExampleValue(runtime *fory.Fory) error {
	return runtime.RegisterStructByName(exampleValue{}, "example.Value")
}

func ExampleNewNativeFast() {
	codec, err := rediscoordfory.NewNativeFast[exampleValue](rediscoordfory.Options{
		Register: registerExampleValue,
	})
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := codec.Marshal(exampleValue{Name: "cached"})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := codec.Unmarshal(encoded); err != nil {
		log.Fatal(err)
	}
}

func ExampleNewNativeCompatible() {
	codec, err := rediscoordfory.NewNativeCompatible[exampleValue](rediscoordfory.Options{
		Register: registerExampleValue,
	})
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := codec.Marshal(exampleValue{Name: "cached"})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := codec.Unmarshal(encoded); err != nil {
		log.Fatal(err)
	}
}

func ExampleCodec_stampedeCache() {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()

	codec, err := rediscoordfory.NewNativeFast[exampleValue](rediscoordfory.Options{
		Register: registerExampleValue,
	})
	if err != nil {
		log.Fatal(err)
	}
	coordinated, err := rediscoord.NewStampedeCache[exampleValue](rediscoord.Options[exampleValue]{
		Client:         client,
		Cache:          cache.NewMemory[string, exampleValue](),
		Namespace:      "catalog:fory-native-fast:schema-v1",
		Codec:          codec,
		MaxResultBytes: 2 << 20,
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = coordinated
}
