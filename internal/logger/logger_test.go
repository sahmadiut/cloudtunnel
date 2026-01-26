package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefault(t *testing.T) {
	log := NewDefault()
	if log == nil {
		t.Fatal("expected non-nil logger")
	}

	// Test that logging doesn't panic
	log.Info().Msg("test message")
	log.Debug().Str("key", "value").Msg("test with fields")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "text format",
			cfg: Config{
				Level:  "info",
				Format: "text",
			},
			wantErr: false,
		},
		{
			name: "json format",
			cfg: Config{
				Level:  "debug",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "invalid level defaults to info",
			cfg: Config{
				Level:  "invalid",
				Format: "text",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if log == nil {
				t.Error("expected non-nil logger")
			}
		})
	}
}

func TestNewWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:  "info",
		Format: "json",
		File:   logFile,
	}

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Write a log message
	log.Info().Msg("test message to file")

	// Verify file exists and has content
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("log file is empty")
	}
}
