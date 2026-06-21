package jwt_test

import (
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/jwt"
)

func ExampleProvider() {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	provider, err := jwt.NewFixedHMACProvider(
		jwt.HS256,
		[]byte("0123456789abcdef0123456789abcdef"),
		jwt.WithClock(func() time.Time { return now }),
		jwt.WithKeyIDGenerator(func() (string, error) { return "kid-1", nil }),
	)
	if err != nil {
		panic(err)
	}

	token, err := provider.Compose(
		jwt.WithSubject("account-42"),
		jwt.WithAudience("api"),
		jwt.WithExpiresAfter(time.Hour),
		jwt.WithClaim("role", "admin"),
	)
	if err != nil {
		panic(err)
	}

	reader, err := provider.Parse(
		token,
		jwt.WithExpectedSubject("account-42"),
		jwt.WithExpectedAudience("api"),
		jwt.WithExpirationRequired(),
		jwt.WithParseClock(func() time.Time { return now.Add(time.Minute) }),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(reader.Subject())
	fmt.Println(reader.ClaimString("role"))
	fmt.Println(reader.Kid())

	// Output:
	// account-42
	// admin true
	// kid-1
}

func ExampleProvider_rotatingPS() {
	var next int
	provider, err := jwt.NewRSAProvider(
		jwt.PS256,
		jwt.WithKeyIDGenerator(func() (string, error) {
			next++
			return fmt.Sprintf("ps-%d", next), nil
		}),
	)
	if err != nil {
		panic(err)
	}
	if _, err := provider.ForcedRotate(); err != nil {
		panic(err)
	}

	token, err := provider.Compose(jwt.WithSubject("account-42"), jwt.WithExpiresAfter(time.Hour))
	if err != nil {
		panic(err)
	}
	reader, err := provider.Parse(token, jwt.WithExpectedSubject("account-42"))
	if err != nil {
		panic(err)
	}

	fmt.Println(reader.Algorithm())
	fmt.Println(reader.Kid())

	// Output:
	// PS256
	// ps-2
}

func ExampleNewCachedProvider() {
	provider, err := jwt.NewFixedHMACProvider(
		jwt.HS256,
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		panic(err)
	}
	readerCache := cache.NewMemory[string, *jwt.Reader]()
	cached, err := jwt.NewCachedProvider(provider, readerCache)
	if err != nil {
		panic(err)
	}

	token, err := cached.Compose(jwt.WithSubject("account-42"), jwt.WithExpiresAfter(time.Hour))
	if err != nil {
		panic(err)
	}
	reader, err := cached.Parse(token, jwt.WithExpectedSubject("account-42"))
	if err != nil {
		panic(err)
	}
	fmt.Println(reader.Subject())

	// Output:
	// account-42
}
