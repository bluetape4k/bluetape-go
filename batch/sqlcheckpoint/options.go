package sqlcheckpoint

import "errors"

const (
	// DefaultMaxKeyBytes is the default maximum raw checkpoint-key byte size.
	DefaultMaxKeyBytes = 512
	// MaxKeyBytes is the largest configurable raw checkpoint-key byte size.
	MaxKeyBytes = 1024
	// DefaultMaxPayloadBytes is the default maximum encoded checkpoint payload size.
	DefaultMaxPayloadBytes = 1 << 20
	// MaxPayloadBytes is the largest configurable encoded checkpoint payload size.
	MaxPayloadBytes = 16 << 20
)

const maxNamespaceBytes = 128

var (
	errNamespaceTooLong = errors.New("sqlcheckpoint: namespace exceeds 128 bytes")
	errMaxKeyBytes      = errors.New("sqlcheckpoint: max key bytes must be between 1 and 1024")
	errMaxPayloadBytes  = errors.New("sqlcheckpoint: max payload bytes must be between 1 and 16777216")
)

// Options struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options struct {
	// Namespace identifies one checkpoint domain as raw bytes.
	Namespace string
	// MaxKeyBytes limits the raw checkpoint-key byte size.
	MaxKeyBytes int
	// MaxPayloadBytes limits the encoded checkpoint payload size.
	MaxPayloadBytes int
}

type normalizedOptions struct {
	namespace       []byte
	maxKeyBytes     int
	maxPayloadBytes int
}

func (options Options) normalize() (normalizedOptions, error) {
	namespace := []byte(options.Namespace)
	if len(namespace) == 0 {
		namespace = []byte("default")
	}
	if len(namespace) > maxNamespaceBytes {
		return normalizedOptions{}, errNamespaceTooLong
	}

	maxKeyBytes := options.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = DefaultMaxKeyBytes
	}
	if maxKeyBytes < 1 || maxKeyBytes > MaxKeyBytes {
		return normalizedOptions{}, errMaxKeyBytes
	}

	maxPayloadBytes := options.MaxPayloadBytes
	if maxPayloadBytes == 0 {
		maxPayloadBytes = DefaultMaxPayloadBytes
	}
	if maxPayloadBytes < 1 || maxPayloadBytes > MaxPayloadBytes {
		return normalizedOptions{}, errMaxPayloadBytes
	}

	return normalizedOptions{
		namespace:       namespace,
		maxKeyBytes:     maxKeyBytes,
		maxPayloadBytes: maxPayloadBytes,
	}, nil
}
