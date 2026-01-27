// Package logger provides structured logging using zerolog
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger
type Logger struct {
	zerolog.Logger
}

// Config holds logger configuration
type Config struct {
	Level  string
	Format string
	File   string
}

// New creates a new logger with the given configuration
func New(cfg Config) (*Logger, error) {
	var writers []io.Writer

	// Determine output format
	if cfg.Format == "text" {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		writers = append(writers, os.Stdout)
	}

	// Add file output if specified
	if cfg.File != "" {
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, file)
	}

	// Create multi-writer
	multi := io.MultiWriter(writers...)

	// Parse log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// Create logger
	log := zerolog.New(multi).
		Level(level).
		With().
		Timestamp().
		Logger()

	return &Logger{Logger: log}, nil
}

// NewDefault creates a default logger with info level and text format
func NewDefault() *Logger {
	log := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Logger()

	return &Logger{Logger: log}
}
