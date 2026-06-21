package id

import (
	"bufio"
	"crypto/rand"
	"io"
	"sync"
)

const defaultEntropyBufferSize = 4096

var defaultEntropy = &lockedBufferedEntropy{
	reader: newBufferedEntropyReader(rand.Reader, defaultEntropyBufferSize),
}

type lockedBufferedEntropy struct {
	mu     sync.Mutex
	reader *bufio.Reader
}

func defaultEntropyReader() io.Reader {
	return defaultEntropy
}

func newBufferedEntropyReader(reader io.Reader, size int) *bufio.Reader {
	return bufio.NewReaderSize(reader, size)
}

func (r *lockedBufferedEntropy) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return io.ReadFull(r.reader, p)
}
