package dynamodbleader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bluetape4k/bluetape-go/leader"
)

// Client Elector가 사용하는 DynamoDB SDK method의 최소 subset이다.
// Client의 생성, credential, retry, timeout, close lifecycle은 caller가 소유한다.
type Client interface {
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

const defaultRetryDelay = 25 * time.Millisecond

const (
	defaultKeyAttribute   = "group"
	defaultOwnerAttribute = "owner_token"
	defaultLeaseAttribute = "lease_until_ms"
	defaultTTLAttribute   = "expires_at"
)

type config struct {
	keyAttribute   string
	ownerAttribute string
	leaseAttribute string
	ttlAttribute   string
	clock          func() time.Time
	retryDelay     time.Duration
	logger         *slog.Logger
}

// Option DynamoDB leader provider의 backend 고유 설정을 변경한다.
type Option func(*config) error

// WithAttributeNames item key, owner, lease, TTL attribute 이름을 설정한다.
func WithAttributeNames(key, owner, lease, ttl string) Option {
	return func(cfg *config) error {
		values := []string{key, owner, lease, ttl}
		for _, value := range values {
			if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
				return invalidOptions("attribute names must be non-blank")
			}
		}
		for i, value := range values {
			for _, other := range values[i+1:] {
				if value == other {
					return invalidOptions("attribute names must be distinct")
				}
			}
		}
		cfg.keyAttribute = key
		cfg.ownerAttribute = owner
		cfg.leaseAttribute = lease
		cfg.ttlAttribute = ttl
		return nil
	}
}

// WithClock lease deadline 계산에 사용할 시계를 주입한다.
func WithClock(clock func() time.Time) Option {
	return func(cfg *config) error {
		if clock == nil {
			return invalidOptions("clock must not be nil")
		}
		cfg.clock = clock
		return nil
	}
}

// WithRetryDelay contention retry 사이의 bounded delay를 설정한다.
func WithRetryDelay(delay time.Duration) Option {
	return func(cfg *config) error {
		if delay <= 0 {
			return invalidOptions("retry delay must be positive")
		}
		cfg.retryDelay = delay
		return nil
	}
}

// WithLogger lifecycle 및 provider failure를 기록할 caller-owned logger를 설정한다.
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) error {
		if logger == nil {
			return invalidOptions("logger must not be nil")
		}
		cfg.logger = logger
		return nil
	}
}

func defaultConfig() config {
	return config{
		keyAttribute:   defaultKeyAttribute,
		ownerAttribute: defaultOwnerAttribute,
		leaseAttribute: defaultLeaseAttribute,
		ttlAttribute:   defaultTTLAttribute,
		clock:          time.Now,
		retryDelay:     defaultRetryDelay,
		logger:         slog.Default(),
	}
}

// New caller-owned DynamoDB client와 table 위에 leader elector를 생성한다.
func New(client Client, tableName string, options leader.Options, optionFns ...Option) (*Elector, error) {
	if isNil(client) {
		return nil, ErrInvalidClient
	}
	if strings.TrimSpace(tableName) == "" {
		return nil, invalidOptions("table name must be non-blank")
	}
	normalized, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.Lease < time.Millisecond || normalized.RenewInterval < time.Millisecond || normalized.RenewInterval >= normalized.Lease {
		return nil, invalidOptions("lease and renew interval must be millisecond values with renew interval below lease")
	}
	cfg := defaultConfig()
	for _, option := range optionFns {
		if option == nil {
			return nil, invalidOptions("option must not be nil")
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	token, err := newOwnerToken(normalized.MemberID)
	if err != nil {
		return nil, err
	}
	return &Elector{
		client:    client,
		tableName: tableName,
		opts:      normalized,
		cfg:       cfg,
		token:     token,
	}, nil
}

func newOwnerToken(memberID string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("dynamodb leader owner token: %w", err)
	}
	return memberID + ":" + hex.EncodeToString(random[:]), nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func invalidOptions(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidOptions, detail)
}
