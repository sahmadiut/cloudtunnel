// Package health provides connection health monitoring
package health

import (
	"context"
	"sync"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

// Status represents the health status of a connection
type Status int

const (
	StatusUnknown Status = iota
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
)

func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Pinger is an interface for checking connection health
type Pinger interface {
	Ping(ctx context.Context) error
}

// Checker monitors connection health
type Checker struct {
	pinger   Pinger
	interval time.Duration
	timeout  time.Duration
	log      *logger.Logger

	mu             sync.RWMutex
	status         Status
	lastCheck      time.Time
	consecutiveFails int
	onUnhealthy    func()
	onRecovered    func()

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// CheckerConfig holds configuration for the health checker
type CheckerConfig struct {
	Interval    time.Duration
	Timeout     time.Duration
	OnUnhealthy func()
	OnRecovered func()
}

// NewChecker creates a new health checker
func NewChecker(pinger Pinger, cfg CheckerConfig, log *logger.Logger) *Checker {
	ctx, cancel := context.WithCancel(context.Background())

	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = cfg.Interval / 2
	}

	return &Checker{
		pinger:      pinger,
		interval:    cfg.Interval,
		timeout:     cfg.Timeout,
		log:         log,
		status:      StatusUnknown,
		onUnhealthy: cfg.OnUnhealthy,
		onRecovered: cfg.OnRecovered,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins health checking
func (c *Checker) Start() {
	c.wg.Add(1)
	go c.run()
}

// Stop stops health checking
func (c *Checker) Stop() {
	c.cancel()
	c.wg.Wait()
}

// Status returns the current health status
func (c *Checker) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// LastCheck returns the time of the last health check
func (c *Checker) LastCheck() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastCheck
}

func (c *Checker) run() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Initial check
	c.check()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.check()
		}
	}
}

func (c *Checker) check() {
	ctx, cancel := context.WithTimeout(c.ctx, c.timeout)
	defer cancel()

	err := c.pinger.Ping(ctx)

	c.mu.Lock()
	c.lastCheck = time.Now()
	prevStatus := c.status

	if err != nil {
		c.consecutiveFails++
		c.log.Debug().
			Err(err).
			Int("consecutive_fails", c.consecutiveFails).
			Msg("health check failed")

		if c.consecutiveFails >= 3 {
			c.status = StatusUnhealthy
		} else {
			c.status = StatusDegraded
		}
	} else {
		wasUnhealthy := c.status == StatusUnhealthy
		c.consecutiveFails = 0
		c.status = StatusHealthy

		if wasUnhealthy && c.onRecovered != nil {
			c.mu.Unlock()
			c.onRecovered()
			c.log.Info().Msg("connection recovered")
			return
		}
	}

	if prevStatus != StatusUnhealthy && c.status == StatusUnhealthy && c.onUnhealthy != nil {
		c.mu.Unlock()
		c.onUnhealthy()
		c.log.Warn().Msg("connection unhealthy")
		return
	}

	c.mu.Unlock()
}

// CheckOnce performs a single health check
func (c *Checker) CheckOnce(ctx context.Context) (Status, error) {
	err := c.pinger.Ping(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastCheck = time.Now()

	if err != nil {
		c.consecutiveFails++
		if c.consecutiveFails >= 3 {
			c.status = StatusUnhealthy
		} else {
			c.status = StatusDegraded
		}
		return c.status, err
	}

	c.consecutiveFails = 0
	c.status = StatusHealthy
	return c.status, nil
}
