package redisbloom

import (
	"context"
	"errors"
	"reflect"
	"strings"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrCuckooUnsupported 서버가 요청한 CF 명령을 지원하지 않을 때 반환한다.
	// middleware가 감싸거나 결합한 오류는 확정적인 미지원으로 분류하지 않는다.
	ErrCuckooUnsupported = errors.New("redis cuckoo: unsupported command")
	// ErrCuckooReply 서버 응답이 문서화된 형식과 다를 때 반환한다.
	ErrCuckooReply = errors.New("redis cuckoo: invalid reply")
	// ErrCuckooFull 공간 부족 또는 확장 자원 부족으로 삽입이 거부되었음을 나타낸다.
	// 서버 응답만으로 두 원인을 구분할 수 없다.
	ErrCuckooFull = errors.New("redis cuckoo: full")
)

// CuckooClient 호출자가 소유한 동시 호출 가능한 Redis 명령 경계다.
// mutation의 client·redirect·middleware 자동 replay를 금지해야 한다.
// 생성자는 이 설정을 검사하지 않는다. 연결·취소·timeout·Close는 호출자 책임이다.
type CuckooClient interface {
	Do(context.Context, ...any) *redis.Cmd
}

// CuckooOptions 필터의 client와 비민감 논리 namespace를 지정한다.
type CuckooOptions struct {
	Client    CuckooClient
	Namespace string
}

// CuckooReserveOptions 신뢰하는 운영 설정이다. 호출자는 메모리·CPU quota를 별도로 제한한다.
// Capacity는 4..2^30이며 정규화된 BucketSize의 두 배 이상이다.
// BucketSize 0은 2, MaxIterations 0은 20이다. Expansion 0은 확장 금지이며 최대32768이다.
// Capacity와 양수 Expansion은 서버가 2의 거듭제곱으로 반올림하며 조기 포화될 수 있다.
type CuckooReserveOptions struct {
	Capacity      int64
	BucketSize    uint8
	MaxIterations uint16
	Expansion     uint16
}

// Cuckoo 모듈을 지원하는 Redis의 확률적 membership 필터다. exact set이나 인증 수단이 아니다.
// item은 빈 문자열을 포함한 최대64 KiB byte열이며 변환하지 않는다.
// nil context는 ErrInvalidOptions, 전송 전 취소는 전송0회다. 전송된 mutation 실패는
// redis.ErrCommitUnknown일 수 있으므로 성공으로 세거나 자동 재실행해서는 안 된다.
// 반환 오류 문자열은 안전하지만 unwrap한 원인에는 민감정보가 남을 수 있다.
type Cuckoo interface {
	// Reserve 새 필터를 예약한다. 기존 key를 덮어쓰거나 자동 복구하지 않는다.
	Reserve(context.Context, CuckooReserveOptions) error
	// Add 예약된 필터에 한 번 삽입한다. 미예약 key는 자동 생성하지 않는다.
	Add(context.Context, string) error
	// Exists 존재 가능성을 반환한다. false는 부재·missing·wrong-type을 구별하지 않는다.
	Exists(context.Context, string) (bool, error)
	// Count 충돌로 과대 추정할 수 있는 중복 삽입 수를 반환한다.
	Count(context.Context, string) (int64, error)
	// Delete 알려진 성공 삽입을 한 번 제거한다. Exists만 보고 삭제하면 안 된다.
	// never-added 또는 초과 삭제는 다른 표본을 제거해 false negative를 만들 수 있다.
	Delete(context.Context, string) (bool, error)
}

type cuckoo struct {
	client CuckooClient
	key    string
}

// NewCuckoo IO 없이 client와 namespace를 검증한다. nil·typed-nil client를 거부한다.
// Namespace는 1..128 byte ASCII 영숫자와 ._-:만 허용하며 기존 namespace 예약 규칙을 따른다.
// client는 no-retry, deadline/IO timeout과 신뢰된 endpoint를 제공해야 한다.
// raw Do의 RESP 디코딩 메모리 상한은 이 adapter가 보장하지 않는다.
func NewCuckoo(options CuckooOptions) (Cuckoo, error) {
	if options.Client == nil {
		return nil, ErrInvalidOptions
	}
	v := reflect.ValueOf(options.Client)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return nil, ErrInvalidOptions
		}
	}
	builder, err := keyBuilderForNamespace("bluetape:probabilistic:cuckoo:v1", options.Namespace)
	if err != nil {
		return nil, ErrInvalidOptions
	}
	key, err := structuralKeyValue(builder)
	if err != nil {
		return nil, ErrInvalidOptions
	}
	return &cuckoo{client: options.Client, key: key}, nil
}

func (c *cuckoo) Reserve(ctx context.Context, options CuckooReserveOptions) error {
	if options.BucketSize == 0 {
		options.BucketSize = 2
	}
	if options.MaxIterations == 0 {
		options.MaxIterations = 20
	}
	if options.Capacity < 4 || options.Capacity > 1<<30 || options.Capacity < 2*int64(options.BucketSize) || options.Expansion > 32768 {
		return ErrInvalidOptions
	}
	_, err := c.execute(ctx, "reserve", true, []any{"CF.RESERVE", c.key, options.Capacity, "BUCKETSIZE", options.BucketSize, "MAXITERATIONS", options.MaxIterations, "EXPANSION", options.Expansion}, func(v any) (any, error) {
		if s, ok := v.(string); ok && s == "OK" {
			return nil, nil
		}
		return nil, ErrCuckooReply
	})
	return err
}

func (c *cuckoo) Add(ctx context.Context, item string) error {
	if len(item) > 64<<10 {
		return ErrInvalidOptions
	}
	_, err := c.execute(ctx, "add", true, []any{"CF.INSERT", c.key, "NOCREATE", "ITEMS", item}, func(v any) (any, error) {
		items, ok := v.([]any)
		if !ok || len(items) != 1 {
			return nil, ErrCuckooReply
		}
		switch item := items[0].(type) {
		case int64:
			if item == 1 {
				return nil, nil
			}
			if item == -1 {
				return nil, ErrCuckooFull
			}
		case bool:
			if item {
				return nil, nil
			}
			return nil, ErrCuckooFull
		}
		return nil, ErrCuckooReply
	})
	return err
}

func (c *cuckoo) Exists(ctx context.Context, item string) (bool, error) {
	return c.boolean(ctx, "exists", "CF.EXISTS", false, item)
}

func (c *cuckoo) Delete(ctx context.Context, item string) (bool, error) {
	return c.boolean(ctx, "delete", "CF.DEL", true, item)
}

func (c *cuckoo) boolean(ctx context.Context, operation, command string, mutation bool, item string) (bool, error) {
	if len(item) > 64<<10 {
		return false, ErrInvalidOptions
	}
	value, err := c.execute(ctx, operation, mutation, []any{command, c.key, item}, func(v any) (any, error) {
		switch n := v.(type) {
		case int64:
			if n == 0 || n == 1 {
				return n == 1, nil
			}
		case bool:
			return n, nil
		}
		return nil, ErrCuckooReply
	})
	if err != nil {
		return false, err
	}
	return value.(bool), nil
}

func (c *cuckoo) Count(ctx context.Context, item string) (int64, error) {
	if len(item) > 64<<10 {
		return 0, ErrInvalidOptions
	}
	value, err := c.execute(ctx, "count", false, []any{"CF.COUNT", c.key, item}, func(v any) (any, error) {
		n, ok := v.(int64)
		if !ok || n < 0 {
			return nil, ErrCuckooReply
		}
		return n, nil
	})
	if err != nil {
		return 0, err
	}
	return value.(int64), nil
}

func (c *cuckoo) execute(ctx context.Context, operation string, mutation bool, args []any, decode func(any) (any, error)) (any, error) {
	if ctx == nil {
		return nil, ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return nil, c.operationError(operation, err)
	}
	cmd := c.client.Do(ctx, args...)
	var raw any
	var providerErr error
	if cmd == nil {
		providerErr = ErrCuckooReply
	} else {
		raw, providerErr = cmd.Result()
	}
	late := ctx.Err()
	if providerErr != nil || late != nil {
		cause := errors.Join(providerErr, late)
		unsupported := raw == nil && late == nil && cuckooUnsupported(providerErr, args[0].(string))
		if unsupported {
			cause = errors.Join(cause, ErrCuckooUnsupported)
		} else if mutation {
			cause = errors.Join(cause, btredis.ErrCommitUnknown)
		}
		return nil, c.operationError(operation, cause)
	}
	value, err := decode(raw)
	late = ctx.Err()
	if err != nil || late != nil {
		cause := errors.Join(err, late)
		if mutation && (late != nil || !errors.Is(err, ErrCuckooFull)) {
			cause = errors.Join(cause, btredis.ErrCommitUnknown)
		}
		return nil, c.operationError(operation, cause)
	}
	return value, nil
}

func (c *cuckoo) operationError(operation string, cause error) error {
	return btredis.NewOpError(btredis.OpLabels{Family: "redis cuckoo", Operation: operation}, c.key, cause)
}

func cuckooUnsupported(err error, command string) bool {
	// transport와 server error를 합친 오류는 확정적인 미지원 응답이 아니다.
	server, ok := err.(redis.Error) //nolint:errorlint // 감싼 transport 오류를 확정적인 미지원으로 오인하지 않는다.
	if !ok {
		return false
	}
	message := strings.TrimPrefix(strings.ToLower(server.Error()), "err ")
	return strings.HasPrefix(message, "unknown command '"+strings.ToLower(command)+"'")
}
