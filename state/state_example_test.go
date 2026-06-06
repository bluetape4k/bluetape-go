package state_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/state"
)

func ExampleMachine_Transition() {
	type orderState string
	type orderEvent string

	const (
		created orderState = "created"
		paid    orderState = "paid"

		pay orderEvent = "pay"
	)

	machine, err := state.NewMachine(
		created,
		[]state.Transition[orderState, orderEvent]{
			{From: created, Event: pay, To: paid},
		},
	)
	if err != nil {
		return
	}

	result, err := machine.Transition(context.Background(), pay)
	if err != nil {
		return
	}
	fmt.Println(result.Previous, result.Event, result.Current)
	fmt.Println(errors.Is(err, state.ErrInvalidTransition))

	// Output:
	// created pay paid
	// false
}
