package rediscoordfory_test

import (
	"log"

	"github.com/apache/fory/go/fory"
	rediscoordfory "github.com/bluetape4k/bluetape-go/cache/rediscoord/fory"
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
