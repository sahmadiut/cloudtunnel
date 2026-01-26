// Package chunker provides packet size control for tunnel data
package chunker

import (
	"io"
	"sync"
)

// Chunker handles splitting and reassembling data into fixed-size chunks
type Chunker struct {
	size int
	pool *sync.Pool
}

// New creates a new Chunker with the specified chunk size
func New(size int) *Chunker {
	return &Chunker{
		size: size,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// ChunkSize returns the configured chunk size
func (c *Chunker) ChunkSize() int {
	return c.size
}

// GetBuffer gets a buffer from the pool
func (c *Chunker) GetBuffer() []byte {
	return c.pool.Get().([]byte)
}

// PutBuffer returns a buffer to the pool
func (c *Chunker) PutBuffer(buf []byte) {
	// Only put back buffers of the correct size
	if cap(buf) >= c.size {
		c.pool.Put(buf[:c.size])
	}
}

// Split splits data into chunks of the configured size
func (c *Chunker) Split(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	numChunks := (len(data) + c.size - 1) / c.size
	chunks := make([][]byte, numChunks)

	for i := 0; i < numChunks; i++ {
		start := i * c.size
		end := start + c.size
		if end > len(data) {
			end = len(data)
		}
		chunks[i] = data[start:end]
	}

	return chunks
}

// ChunkedReader reads data in chunks
type ChunkedReader struct {
	reader  io.Reader
	chunker *Chunker
	buf     []byte
	pos     int
}

// NewChunkedReader creates a new chunked reader
func NewChunkedReader(r io.Reader, chunkSize int) *ChunkedReader {
	return &ChunkedReader{
		reader:  r,
		chunker: New(chunkSize),
	}
}

// ReadChunk reads the next chunk of data
func (r *ChunkedReader) ReadChunk() ([]byte, error) {
	buf := r.chunker.GetBuffer()

	n, err := io.ReadFull(r.reader, buf)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			// Partial read is okay, return what we got
			return buf[:n], nil
		}
		r.chunker.PutBuffer(buf)
		return nil, err
	}

	return buf[:n], nil
}

// Read implements io.Reader with chunk-aware buffering
func (r *ChunkedReader) Read(p []byte) (int, error) {
	// If we have data in the buffer, return it first
	if r.pos < len(r.buf) {
		n := copy(p, r.buf[r.pos:])
		r.pos += n
		return n, nil
	}

	// Read a new chunk
	chunk, err := r.ReadChunk()
	if err != nil {
		return 0, err
	}

	r.buf = chunk
	r.pos = 0

	n := copy(p, r.buf)
	r.pos = n
	return n, nil
}

// ChunkedWriter writes data in chunks
type ChunkedWriter struct {
	writer  io.Writer
	chunker *Chunker
	buf     []byte
	pos     int
}

// NewChunkedWriter creates a new chunked writer
func NewChunkedWriter(w io.Writer, chunkSize int) *ChunkedWriter {
	return &ChunkedWriter{
		writer:  w,
		chunker: New(chunkSize),
		buf:     make([]byte, chunkSize),
	}
}

// Write implements io.Writer with chunk-aware buffering
func (w *ChunkedWriter) Write(p []byte) (int, error) {
	written := 0

	for len(p) > 0 {
		// Fill buffer
		n := copy(w.buf[w.pos:], p)
		w.pos += n
		p = p[n:]
		written += n

		// Flush if buffer is full
		if w.pos >= w.chunker.size {
			if err := w.Flush(); err != nil {
				return written - n, err
			}
		}
	}

	return written, nil
}

// Flush writes any buffered data
func (w *ChunkedWriter) Flush() error {
	if w.pos == 0 {
		return nil
	}

	_, err := w.writer.Write(w.buf[:w.pos])
	w.pos = 0
	return err
}

// Close flushes and closes the writer if it implements io.Closer
func (w *ChunkedWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}

	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}
