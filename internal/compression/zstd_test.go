package compression

import (
	"bytes"
	"io"
	"testing"
)

func TestCompressor(t *testing.T) {
	c := New()

	original := []byte("Hello, World! This is a test of the compression system.")

	// Compress
	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	// Decompress
	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}

	// Verify
	if !bytes.Equal(original, decompressed) {
		t.Errorf("decompressed data doesn't match original")
	}
}

func TestCompressorLargeData(t *testing.T) {
	c := New()

	// Create a larger data set with repeating pattern (good for compression)
	original := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 1000)

	// Compress
	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	// Check that compression actually reduced size
	if len(compressed) >= len(original) {
		t.Logf("warning: compressed size (%d) >= original size (%d)", len(compressed), len(original))
	}

	// Decompress
	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}

	// Verify
	if !bytes.Equal(original, decompressed) {
		t.Errorf("decompressed data doesn't match original")
	}
}

func TestCompressedWriter(t *testing.T) {
	var buf bytes.Buffer

	// Create compressed writer
	w, err := NewCompressedWriter(&buf)
	if err != nil {
		t.Fatalf("failed to create compressed writer: %v", err)
	}

	// Write data
	data := []byte("Hello, compressed world!")
	_, err = w.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Close to flush
	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Read back with compressed reader
	r, err := NewCompressedReader(&buf)
	if err != nil {
		t.Fatalf("failed to create compressed reader: %v", err)
	}
	defer r.Close()

	result := make([]byte, len(data)*2) // Use larger buffer
	n, err := r.Read(result)
	if err != nil && err != io.EOF {
		t.Fatalf("read failed: %v", err)
	}

	if !bytes.Equal(data, result[:n]) {
		t.Errorf("read data doesn't match written data: got %q, want %q", result[:n], data)
	}
}

func BenchmarkCompress(b *testing.B) {
	c := New()
	data := bytes.Repeat([]byte("benchmark data for compression testing "), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Compress(data)
	}
}

func BenchmarkDecompress(b *testing.B) {
	c := New()
	data := bytes.Repeat([]byte("benchmark data for compression testing "), 100)
	compressed, _ := c.Compress(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Decompress(compressed)
	}
}
