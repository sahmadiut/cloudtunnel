package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

// mockPinger implements Pinger for testing
type mockPinger struct {
	failCount int
	calls     int
}

func (m *mockPinger) Ping(ctx context.Context) error {
	m.calls++
	if m.calls <= m.failCount {
		return errors.New("ping failed")
	}
	return nil
}

func TestCheckerStatus(t *testing.T) {
	log := logger.NewDefault()
	pinger := &mockPinger{failCount: 0}

	checker := NewChecker(pinger, CheckerConfig{
		Interval: 100 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
	}, log)

	// Initial status should be unknown
	if checker.Status() != StatusUnknown {
		t.Errorf("expected StatusUnknown, got %v", checker.Status())
	}

	// Check once
	ctx := context.Background()
	status, err := checker.CheckOnce(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if status != StatusHealthy {
		t.Errorf("expected StatusHealthy, got %v", status)
	}
}

func TestCheckerUnhealthy(t *testing.T) {
	log := logger.NewDefault()
	pinger := &mockPinger{failCount: 10}

	checker := NewChecker(pinger, CheckerConfig{
		Interval: 100 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
	}, log)

	ctx := context.Background()

	// First 3 failures should make it unhealthy
	for i := 0; i < 3; i++ {
		checker.CheckOnce(ctx)
	}

	if checker.Status() != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy after 3 failures, got %v", checker.Status())
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %s, want %s", tt.status, got, tt.want)
		}
	}
}
