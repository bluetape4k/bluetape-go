package id

import (
	"strconv"
	"sync"
	"time"
)

const (
	snowflakeMachineBits  = 10
	snowflakeSequenceBits = 12
	snowflakeMaxMachineID = int64(1<<snowflakeMachineBits - 1)
	snowflakeMaxSequence  = int64(1<<snowflakeSequenceBits - 1)
	snowflakeMaxTimestamp = int64(1<<(63-snowflakeTimeShift) - 1)
	snowflakeMachineShift = snowflakeSequenceBits
	snowflakeTimeShift    = snowflakeMachineBits + snowflakeSequenceBits
)

var defaultSnowflakeEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// SnowflakeGenerator interface 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type SnowflakeGenerator interface {
	Int64Generator
	StringGenerator
}

// SnowflakeOption func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type SnowflakeOption func(*snowflakeConfig) error

type snowflakeConfig struct {
	epoch time.Time
	now   func() time.Time
}

// WithSnowflakeEpoch WithSnowflakeEpoch 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - epoch: WithSnowflakeEpoch 동작에 필요한 epoch 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithSnowflakeEpoch(epoch time.Time) SnowflakeOption {
	return func(c *snowflakeConfig) error {
		if epoch.IsZero() {
			return OptionError{Option: "epoch", Err: errorsNew("must not be zero")}
		}
		c.epoch = epoch.UTC()
		return nil
	}
}

// WithSnowflakeTime WithSnowflakeTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - now: WithSnowflakeTime 동작에 필요한 now 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithSnowflakeTime(now func() time.Time) SnowflakeOption {
	return func(c *snowflakeConfig) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		c.now = now
		return nil
	}
}

type snowflakeGenerator struct {
	mu        sync.Mutex
	machineID int64
	config    snowflakeConfig
	lastMs    int64
	sequence  int64
}

// SnowflakeParts struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type SnowflakeParts struct {
	Time      time.Time
	MachineID int64
	Sequence  int64
}

// NewSnowflakeGenerator NewSnowflakeGenerator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - machineID: NewSnowflakeGenerator 동작에 필요한 machineID 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: NewSnowflakeGenerator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewSnowflakeGenerator(machineID int64, options ...SnowflakeOption) (SnowflakeGenerator, error) {
	if machineID < 0 || machineID > snowflakeMaxMachineID {
		return nil, OptionError{Option: "machineID", Err: ErrInvalidOptions}
	}
	config, err := newSnowflakeConfig(options...)
	if err != nil {
		return nil, err
	}
	return &snowflakeGenerator{machineID: machineID, config: config, lastMs: -1}, nil
}

func newSnowflakeConfig(options ...SnowflakeOption) (snowflakeConfig, error) {
	config := snowflakeConfig{
		epoch: defaultSnowflakeEpoch,
		now:   time.Now,
	}
	for _, option := range options {
		if option == nil {
			return config, OptionError{Option: "option", Err: errorsNew("must not be nil")}
		}
		if err := option(&config); err != nil {
			return config, err
		}
	}
	return config, nil
}

func (g *snowflakeGenerator) NextInt64() (int64, error) {
	if g == nil {
		return 0, OptionError{Option: "generator", Err: errorsNew("must not be nil")}
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	nowMs := int64(g.config.now().UTC().Sub(g.config.epoch) / time.Millisecond)
	if nowMs < 0 {
		return 0, OptionError{Option: "time", Err: errorsNew("before epoch")}
	}
	if nowMs > snowflakeMaxTimestamp {
		return 0, OptionError{Option: "time", Err: errorsNew("outside 63-bit range")}
	}
	if nowMs < g.lastMs {
		return 0, ClockRollbackError{Last: g.lastMs, Now: nowMs}
	}
	if nowMs == g.lastMs {
		if g.sequence == snowflakeMaxSequence {
			return 0, SequenceExhaustedError{Millis: nowMs}
		}
		g.sequence++
	} else {
		g.sequence = 0
		g.lastMs = nowMs
	}

	id := (nowMs << snowflakeTimeShift) |
		(g.machineID << snowflakeMachineShift) |
		g.sequence
	return id, nil
}

func (g *snowflakeGenerator) NextString() (string, error) {
	value, err := g.NextInt64()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(value, 10), nil
}

// ParseSnowflake ParseSnowflake 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ParseSnowflake가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseSnowflake(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		if err == nil {
			err = errorsNew("must be non-negative")
		}
		return 0, ParseError{Kind: "snowflake", Value: value, Err: err}
	}
	return parsed, nil
}

// DecodeSnowflake DecodeSnowflake 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: DecodeSnowflake 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: DecodeSnowflake 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecodeSnowflake(value int64, options ...SnowflakeOption) (SnowflakeParts, error) {
	if value < 0 {
		return SnowflakeParts{}, ParseError{Kind: "snowflake", Value: strconv.FormatInt(value, 10), Err: errorsNew("must be non-negative")}
	}
	config, err := newSnowflakeConfig(options...)
	if err != nil {
		return SnowflakeParts{}, err
	}
	timestamp := value >> snowflakeTimeShift
	machineID := (value >> snowflakeMachineShift) & snowflakeMaxMachineID
	sequence := value & snowflakeMaxSequence
	return SnowflakeParts{
		Time:      config.epoch.Add(time.Duration(timestamp) * time.Millisecond).UTC(),
		MachineID: machineID,
		Sequence:  sequence,
	}, nil
}
