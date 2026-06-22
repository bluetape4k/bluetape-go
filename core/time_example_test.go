package core_test

import (
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/core"
)

func ExampleParseYearQuarter() {
	quarter, err := core.ParseYearQuarter("2026-Q3")
	if err != nil {
		return
	}
	start, err := quarter.Start(time.UTC)
	if err != nil {
		return
	}
	end, err := quarter.End(time.UTC)
	if err != nil {
		return
	}

	fmt.Println(quarter)
	fmt.Println(start.Format(time.DateOnly))
	fmt.Println(end.Format(time.DateOnly))

	// Output:
	// 2026-Q3
	// 2026-07-01
	// 2026-10-01
}

func ExampleDatesUntil() {
	start := time.Date(2026, time.January, 30, 18, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.February, 2, 9, 0, 0, 0, time.UTC)

	for date := range core.DatesUntil(start, end) {
		fmt.Println(date.Format(time.DateOnly))
	}

	// Output:
	// 2026-01-30
	// 2026-01-31
	// 2026-02-01
}
