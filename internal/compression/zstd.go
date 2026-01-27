// Package compression provides zstd compression for tunnel data
package compression

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Pool manages reusable encoder/decoder instances
type Pool struct {
	encoders sync.Pool
	decoders sync.Pool
}

// NewPool creates a new compression pool
func NewPool() *Pool {
	return &Pool{
		encoders: sync.Pool{
			New: func() interface{} {
				enc, _ := zstd.NewWriter(nil,
					zstd.WithEncoderLevel(zstd.SpeedDefault),
				)
				return enc
			},
		},
		decoders: sync.Pool{
			New: func() interface{} {
				dec, _ := zstd.NewReader(nil)
				return dec
			},
		},
	}
}

// GetEncoder gets an encoder from the pool
func (p *Pool) GetEncoder() *zstd.Encoder {
	return p.encoders.Get().(*zstd.Encoder)
}

// PutEncoder returns an encoder to the pool
func (p *Pool) PutEncoder(enc *zstd.Encoder) {
	p.encoders.Put(enc)
}

// GetDecoder gets a decoder from the pool
func (p *Pool) GetDecoder() *zstd.Decoder {
	return p.decoders.Get().(*zstd.Decoder)
}

// PutDecoder returns a decoder to the pool
func (p *Pool) PutDecoder(dec *zstd.Decoder) {
	p.decoders.Put(dec)
}

// Compressor provides compression and decompression operations
type Compressor struct {
	pool *Pool
}

// New creates a new Compressor
func New() *Compressor {
	return &Compressor{
		pool: NewPool(),
	}
}

// Compress compresses the input data using zstd
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	enc := c.pool.GetEncoder()
	defer c.pool.PutEncoder(enc)

	return enc.EncodeAll(data, nil), nil
}

// Decompress decompresses the input data using zstd
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	dec := c.pool.GetDecoder()
	defer c.pool.PutDecoder(dec)

	return dec.DecodeAll(data, nil)
}

// CompressedReader wraps a reader with decompression
type CompressedReader struct {
	reader  io.Reader
	decoder *zstd.Decoder
}

// NewCompressedReader creates a new compressed reader
func NewCompressedReader(r io.Reader) (*CompressedReader, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &CompressedReader{
		reader:  r,
		decoder: dec,
	}, nil
}

// Read reads and decompresses data
func (r *CompressedReader) Read(p []byte) (int, error) {
	return r.decoder.Read(p)
}

// Close closes the decoder
func (r *CompressedReader) Close() error {
	r.decoder.Close()
	return nil
}

// CompressedWriter wraps a writer with compression
type CompressedWriter struct {
	writer  io.Writer
	encoder *zstd.Encoder
}

// NewCompressedWriter creates a new compressed writer
func NewCompressedWriter(w io.Writer) (*CompressedWriter, error) {
	enc, err := zstd.NewWriter(w)
	if err != nil {
		return nil, err
	}

	return &CompressedWriter{
		writer:  w,
		encoder: enc,
	}, nil
}

// Write compresses and writes data
func (w *CompressedWriter) Write(p []byte) (int, error) {
	return w.encoder.Write(p)
}

// Close flushes and closes the encoder
func (w *CompressedWriter) Close() error {
	return w.encoder.Close()
}

// Flush flushes the encoder
func (w *CompressedWriter) Flush() error {
	return w.encoder.Flush()
}
