package chunker

import (
	"bytes"
	"testing"
)

func TestNew(t *testing.T) {
	c := New(4096)
	if c.ChunkSize() != 4096 {
		t.Errorf("expected chunk size 4096, got %d", c.ChunkSize())
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		chunkSize int
		data      []byte
		wantCount int
	}{
		{
			name:      "empty data",
			chunkSize: 4096,
			data:      []byte{},
			wantCount: 0,
		},
		{
			name:      "single chunk",
			chunkSize: 10,
			data:      []byte("hello"),
			wantCount: 1,
		},
		{
			name:      "exact multiple",
			chunkSize: 5,
			data:      []byte("helloworld"),
			wantCount: 2,
		},
		{
			name:      "with remainder",
			chunkSize: 4,
			data:      []byte("helloworld"),
			wantCount: 3, // 4 + 4 + 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.chunkSize)
			chunks := c.Split(tt.data)

			if len(chunks) != tt.wantCount {
				t.Errorf("expected %d chunks, got %d", tt.wantCount, len(chunks))
			}

			// Verify all chunks together equal original data
			if tt.wantCount > 0 {
				var combined []byte
				for _, chunk := range chunks {
					combined = append(combined, chunk...)
				}
				if !bytes.Equal(combined, tt.data) {
					t.Errorf("combined chunks don't match original data")
				}
			}
		})
	}
}

func TestChunkedWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewChunkedWriter(&buf, 4)

	data := []byte("hello world")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", buf.String())
	}
}

func TestBufferPool(t *testing.T) {
	c := New(1024)

	buf := c.GetBuffer()
	if len(buf) != 1024 {
		t.Errorf("expected buffer size 1024, got %d", len(buf))
	}

	// Return to pool
	c.PutBuffer(buf)

	// Get again
	buf2 := c.GetBuffer()
	if len(buf2) != 1024 {
		t.Errorf("expected buffer size 1024, got %d", len(buf2))
	}
}
