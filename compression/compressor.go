package compression

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// ErrDecompressedSizeExceeded 변수 공개 값이다.
// 호출자는 이 식별자를 패키지의 오류, 옵션, 상수, 또는 기본값 계약을 비교할 때 사용한다.
var ErrDecompressedSizeExceeded = errors.New("decompressed size exceeded")

// Compressor interface 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Compressor interface {
	Name() string
	Compress([]byte) ([]byte, error)
	Decompress([]byte) ([]byte, error)
	NewWriter(io.Writer) (io.WriteCloser, error)
	NewReader(io.Reader) (io.ReadCloser, error)
}

type streamCompressor struct {
	name   string
	writer func(io.Writer) (io.WriteCloser, error)
	reader func(io.Reader) (io.ReadCloser, error)
}

func (c streamCompressor) Name() string {
	return c.name
}

func (c streamCompressor) Compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := c.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("%s compress: %w", c.name, err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("%s compress close: %w", c.name, err)
	}
	return buffer.Bytes(), nil
}

func (c streamCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := c.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%s decompress: %w", c.name, err)
	}
	return decompressed, nil
}

// DecompressLimit DecompressLimit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - compressor: DecompressLimit 동작에 필요한 compressor 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - data: DecompressLimit가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - maxBytes: DecompressLimit 동작에 필요한 maxBytes 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func DecompressLimit(compressor Compressor, data []byte, maxBytes int64) ([]byte, error) {
	if compressor == nil {
		return nil, fmt.Errorf("compressor must not be nil")
	}
	if maxBytes < 0 {
		return nil, fmt.Errorf("maxBytes[%d] must be non-negative", maxBytes)
	}
	reader, err := compressor.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	limited := io.LimitReader(reader, maxBytes+1)
	decompressed, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%s decompress: %w", compressor.Name(), err)
	}
	if int64(len(decompressed)) > maxBytes {
		return nil, fmt.Errorf("%w: max %d bytes", ErrDecompressedSizeExceeded, maxBytes)
	}
	return decompressed, nil
}

func (c streamCompressor) NewWriter(writer io.Writer) (io.WriteCloser, error) {
	if writer == nil {
		return nil, fmt.Errorf("%s writer must not be nil", c.name)
	}
	return c.writer(writer)
}

func (c streamCompressor) NewReader(reader io.Reader) (io.ReadCloser, error) {
	if reader == nil {
		return nil, fmt.Errorf("%s reader must not be nil", c.name)
	}
	return c.reader(reader)
}

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error {
	return nil
}
