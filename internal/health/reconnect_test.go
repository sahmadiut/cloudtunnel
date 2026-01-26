package health

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

func TestReconnectorSuccess(t *testing.T) {
	log := logger.NewDefault()
	calls := int32(0)

	connect := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	r := NewReconnector(connect, ReconnectorConfig{
		MinDelay: 10 * time.Millisecond,
		MaxDelay: 100 * time.Millisecond,
	}, log)

	err := r.Reconnect()
	if err != nil {
		t.Errorf("expected successful reconnection, got error: %v", err)
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestReconnectorRetry(t *testing.T) {
	log := logger.NewDefault()
	calls := int32(0)

	connect := func(ctx context.Context) error {
		c := atomic.AddInt32(&calls, 1)
		if c < 3 {
			return errors.New("connection failed")
		}
		return nil
	}

	r := NewReconnector(connect, ReconnectorConfig{
		MinDelay: 10 * time.Millisecond,
		MaxDelay: 50 * time.Millisecond,
	}, log)

	err := r.Reconnect()
	if err != nil {
		t.Errorf("expected successful reconnection after retries, got error: %v", err)
	}

	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}

	if r.Attempts() != 3 {
		t.Errorf("expected 3 attempts, got %d", r.Attempts())
	}
}

func TestReconnectorStop(t *testing.T) {
	log := logger.NewDefault()
	calls := int32(0)

	connect := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}

	r := NewReconnector(connect, ReconnectorConfig{
		MinDelay: 10 * time.Millisecond,
		MaxDelay: 50 * time.Millisecond,
	}, log)

	// Start async reconnection
	done := make(chan struct{})
	go func() {
		r.Reconnect()
		close(done)
	}()

	// Wait a bit then stop
	time.Sleep(50 * time.Millisecond)
	r.Stop()

	// Should complete quickly after stop
	select {
	case <-done:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("reconnector did not stop in time")
	}
}

func TestReconnectorReset(t *testing.T) {
	log := logger.NewDefault()

	connect := func(ctx context.Context) error {
		return nil
	}

	r := NewReconnector(connect, ReconnectorConfig{
		MinDelay: 10 * time.Millisecond,
		MaxDelay: 50 * time.Millisecond,
	}, log)

	r.Reconnect()

	if r.Attempts() == 0 {
		t.Error("expected attempts > 0")
	}

	r.Reset()

	if r.Attempts() != 0 {
		t.Errorf("expected 0 attempts after reset, got %d", r.Attempts())
	}
}
