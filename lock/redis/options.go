package redislock

import (
	"fmt"
	"strings"
	"time"
)

// Options 는 Redis lock 생성 설정이다.
type Options struct {
	// Key는 Redis lock key다. 필수다.
	Key string
	// TTL은 lock이 자동 만료될 시간이다. 양수여야 한다.
	TTL time.Duration
	// Token은 선택적 owner token이다. 비우면 acquire마다 무작위 token을 만든다.
	Token string
}

type options struct {
	key   string
	ttl   time.Duration
	token string
}

func (o Options) normalize() (options, error) {
	if strings.TrimSpace(o.Key) == "" {
		return options{}, fmt.Errorf("redis lock key must not be empty")
	}
	if o.TTL <= 0 {
		return options{}, fmt.Errorf("redis lock ttl must be positive")
	}
	token := strings.TrimSpace(o.Token)
	if o.Token != "" && token == "" {
		return options{}, fmt.Errorf("redis lock token must not be blank")
	}
	return options{key: o.Key, ttl: o.TTL, token: token}, nil
}
