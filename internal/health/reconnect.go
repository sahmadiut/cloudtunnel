// Package health provides auto-reconnection with exponential backoff
package health

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

// Reconnector manages automatic reconnection with exponential backoff
type Reconnector struct {
	connect      func(context.Context) error
	minDelay     time.Duration
	maxDelay     time.Duration
	maxElapsed   time.Duration
	log          *logger.Logger

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	attempts int
}

// ReconnectorConfig holds configuration for the reconnector
type ReconnectorConfig struct {
	MinDelay   time.Duration
	MaxDelay   time.Duration
	MaxElapsed time.Duration // 0 means no limit
}

// NewReconnector creates a new reconnector
func NewReconnector(
	connect func(context.Context) error,
	cfg ReconnectorConfig,
	log *logger.Logger,
) *Reconnector {
	if cfg.MinDelay <= 0 {
		cfg.MinDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Reconnector{
		connect:    connect,
		minDelay:   cfg.MinDelay,
		maxDelay:   cfg.MaxDelay,
		maxElapsed: cfg.MaxElapsed,
		log:        log,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Reconnect attempts to reconnect with exponential backoff
func (r *Reconnector) Reconnect() error {
	r.mu.Lock()
	r.attempts = 0
	r.mu.Unlock()

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = r.minDelay
	bo.MaxInterval = r.maxDelay
	bo.MaxElapsedTime = r.maxElapsed
	bo.Reset()

	operation := func() error {
		r.mu.Lock()
		r.attempts++
		attempt := r.attempts
		r.mu.Unlock()

		r.log.Info().
			Int("attempt", attempt).
			Msg("attempting to reconnect")

		err := r.connect(r.ctx)
		if err != nil {
			r.log.Warn().
				Err(err).
				Int("attempt", attempt).
				Msg("reconnection failed")
			return err
		}

		r.log.Info().
			Int("attempt", attempt).
			Msg("reconnection successful")
		return nil
	}

	return backoff.Retry(operation, backoff.WithContext(bo, r.ctx))
}

// ReconnectAsync starts reconnection in the background
func (r *Reconnector) ReconnectAsync(callback func(error)) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		err := r.Reconnect()
		if callback != nil {
			callback(err)
		}
	}()
}

// Stop stops any ongoing reconnection attempts
func (r *Reconnector) Stop() {
	r.cancel()
	r.wg.Wait()
}

// Attempts returns the number of reconnection attempts
func (r *Reconnector) Attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

// Reset resets the reconnector state
func (r *Reconnector) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = 0
	r.ctx, r.cancel = context.WithCancel(context.Background())
}
