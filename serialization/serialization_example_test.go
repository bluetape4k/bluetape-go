package serialization_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/serialization"
)

type exampleAccount struct {
	ID     string `json:"id"`
	Active bool   `json:"active"`
}

func ExampleJSONSerializer() {
	serializer := serialization.NewJSONSerializer[exampleAccount]()

	data, err := serializer.Marshal(exampleAccount{ID: "acct-1", Active: true})
	if err != nil {
		return
	}
	value, err := serializer.Unmarshal(data)
	if err != nil {
		return
	}

	fmt.Println(string(data))
	fmt.Println(value.ID, value.Active)

	// Output:
	// {"id":"acct-1","active":true}
	// acct-1 true
}

func ExampleVersionedSerializer() {
	jsonSerializer := serialization.NewJSONSerializer[exampleAccount]()
	serializer, err := serialization.NewVersionedSerializer[exampleAccount](jsonSerializer, 1)
	if err != nil {
		return
	}

	data, err := serializer.Marshal(exampleAccount{ID: "acct-1", Active: true})
	if err != nil {
		return
	}
	value, err := serializer.Unmarshal(data)
	if err != nil {
		return
	}

	fmt.Println(serializer.Format(), serializer.Version())
	fmt.Println(string(data[:4]))
	fmt.Println(value.ID)

	// Output:
	// json 1
	// BTGS
	// acct-1
}

func ExampleStringSerializer() {
	serializer := serialization.StringSerializer{}

	data, err := serializer.Marshal("안녕, bluetape-go")
	if err != nil {
		return
	}
	value, err := serializer.Unmarshal(data)
	if err != nil {
		return
	}

	fmt.Println(serializer.Format())
	fmt.Println(value)

	// Output:
	// string
	// 안녕, bluetape-go
}
