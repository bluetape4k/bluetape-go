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

// SnowflakeGenerator 패키지에서 공개하는 인터페이스다.
type SnowflakeGenerator interface {
	Int64Generator
	StringGenerator
}

// SnowflakeOption func 공개 타입이다.
type SnowflakeOption func(*snowflakeConfig) error

type snowflakeConfig struct {
	epoch time.Time
	now   func() time.Time
}

// WithSnowflakeEpoch SnowflakeEpoch 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - epoch: WithSnowflakeEpoch에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithSnowflakeEpoch(epoch time.Time) SnowflakeOption {
	return func(c *snowflakeConfig) error {
		if epoch.IsZero() {
			return OptionError{Option: "epoch", Err: errorsNew("must not be zero")}
		}
		c.epoch = epoch.UTC()
		return nil
	}
}

// WithSnowflakeTime SnowflakeTime 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - now: WithSnowflakeTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// SnowflakeParts 패키지에서 공개하는 구조체다.
type SnowflakeParts struct {
	Time      time.Time
	MachineID int64
	Sequence  int64
}

// NewSnowflakeGenerator SnowflakeGenerator 인스턴스를 생성한다.
//
// 매개변수:
//   - machineID: NewSnowflakeGenerator에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// ParseSnowflake 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - value: ParseSnowflake가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// DecodeSnowflake Snowflake 형식의 입력을 원래 값으로 디코딩한다.
//
// 매개변수:
//   - value: DecodeSnowflake에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
