package leader

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/bluetape4k/bluetape-go/core"
)

const (
	defaultLease         = 10 * time.Second
	defaultRenewInterval = 3 * time.Second
	defaultKeyPrefix     = "bluetape:leader"
)

// Options leader election 참가자를 설정한다.
type Options struct {
	Group         string
	MemberID      string
	Lease         time.Duration
	RenewInterval time.Duration
	KeyPrefix     string
}

// Normalize 옵션을 검증하고 기본값을 채운다.
func (o Options) Normalize() (Options, error) {
	if err := core.RequireNotBlank("group", o.Group); err != nil {
		return Options{}, err
	}
	if err := core.RequireNotBlank("memberID", o.MemberID); err != nil {
		return Options{}, err
	}
	if err := validateIdentityPart("group", o.Group); err != nil {
		return Options{}, err
	}
	if err := validateIdentityPart("memberID", o.MemberID); err != nil {
		return Options{}, err
	}
	if o.KeyPrefix != "" {
		if err := validateKeyPrefix(o.KeyPrefix); err != nil {
			return Options{}, err
		}
	}

	if o.Lease <= 0 {
		o.Lease = defaultLease
	}
	if o.RenewInterval <= 0 {
		o.RenewInterval = defaultRenewInterval
	}
	if o.KeyPrefix == "" {
		o.KeyPrefix = defaultKeyPrefix
	}
	if len(o.KeyPrefix)+1+len(o.Group) > 512 {
		return Options{}, fmt.Errorf("leader key exceeds 512 bytes")
	}
	return o, nil
}

func validateIdentityPart(name, value string) error {
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("leader %s must not have surrounding whitespace", name)
	}
	if len(value) > 256 {
		return fmt.Errorf("leader %s exceeds 256 bytes", name)
	}
	if strings.ContainsAny(value, ":{}") || containsControl(value) {
		return fmt.Errorf("leader %s has unsafe structure", name)
	}
	return nil
}

func validateKeyPrefix(value string) error {
	if value != strings.TrimSpace(value) || containsControl(value) {
		return fmt.Errorf("leader key prefix has unsafe structure")
	}
	for _, segment := range strings.Split(value, ":") {
		if segment == "" || strings.ContainsAny(segment, "{}") {
			return fmt.Errorf("leader key prefix has unsafe segment")
		}
	}
	return nil
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
