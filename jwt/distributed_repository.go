package jwt

import (
	"context"
	"reflect"
	"time"
)

// DistributedKeyChainRepository JWT key provider repository에서 동작과 caller-visible 계약을 설명한다.
type DistributedKeyChainRepository interface {
	Current(ctx context.Context, now time.Time) (*KeyChain, error)
	Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error)
	Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
	ForcedRotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
	DeleteAll(ctx context.Context) error
}

func requireContext(ctx context.Context) error {
	if ctx == nil {
		return OptionError{Option: "context", Err: errorsNew("must not be nil")}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func requireDistributedRepository(repo DistributedKeyChainRepository) error {
	if repo == nil {
		return OptionError{Option: "repository", Err: errorsNew("must not be nil")}
	}
	value := reflect.ValueOf(repo)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return OptionError{Option: "repository", Err: errorsNew("must not be nil")}
		}
	}
	return nil
}

func createWithContext(ctx context.Context, create func() (*KeyChain, error)) func() (*KeyChain, error) {
	return func() (*KeyChain, error) {
		if err := requireContext(ctx); err != nil {
			return nil, err
		}
		key, err := create()
		if err != nil {
			return nil, err
		}
		if err := requireContext(ctx); err != nil {
			return nil, err
		}
		return key, nil
	}
}
