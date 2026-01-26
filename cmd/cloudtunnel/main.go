// CloudTunnel - Advanced Go-Based TCP-over-HTTP Tunnel
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sahmadiut/cloudtunnel/internal/client"
	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
	"github.com/sahmadiut/cloudtunnel/internal/server"
)

// Version information (set by ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	configFile string
	logLevel   string
	resolveIP  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cloudtunnel",
		Short: "CloudTunnel - TCP-over-HTTP tunnel",
		Long: `CloudTunnel is a production-grade, resilient tunnel written in Go 
that forwards TCP traffic over HTTP/HTTPS. Designed for unstable 
networks with high packet loss and strict limitations.`,
	}

	// Client command
	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Run CloudTunnel in client mode",
		Long:  `Start a CloudTunnel client that connects to a remote server and forwards local ports.`,
		RunE:  runClient,
	}

	clientCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file (required)")
	clientCmd.Flags().StringVar(&logLevel, "log-level", "", "Override log level (debug, info, warn, error)")
	clientCmd.Flags().StringVar(&resolveIP, "resolve-ip", "", "Override resolve_ip setting")
	clientCmd.MarkFlagRequired("config")

	// Server command
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Run CloudTunnel in server mode",
		Long:  `Start a CloudTunnel server that accepts client connections and forwards traffic.`,
		RunE:  runServer,
	}

	serverCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file (required)")
	serverCmd.Flags().StringVar(&logLevel, "log-level", "", "Override log level (debug, info, warn, error)")
	serverCmd.MarkFlagRequired("config")

	// Init command
	initCmd := &cobra.Command{
		Use:   "init [client|server]",
		Short: "Generate example configuration",
		Long:  `Generate an example configuration file for client or server mode.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runInit,
	}

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("CloudTunnel %s\n", version)
			fmt.Printf("  Commit: %s\n", commit)
			fmt.Printf("  Built:  %s\n", date)
		},
	}

	rootCmd.AddCommand(clientCmd, serverCmd, initCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runClient(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override settings from flags
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	if resolveIP != "" {
		cfg.Server.ResolveIP = resolveIP
	}

	// Ensure mode is client
	cfg.Mode = "client"

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create logger
	log, err := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		File:   cfg.Logging.File,
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	log.Info().
		Str("version", version).
		Str("config", configFile).
		Msg("starting CloudTunnel client")

	// Create and start client
	c, err := client.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := c.Start(); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")

	c.Stop()
	return nil
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override settings from flags
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}

	// Ensure mode is server
	cfg.Mode = "server"

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create logger
	log, err := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		File:   cfg.Logging.File,
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	log.Info().
		Str("version", version).
		Str("config", configFile).
		Msg("starting CloudTunnel server")

	// Create server
	s, err := server.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	// Wait for interrupt signal or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		log.Info().Msg("received shutdown signal")
	}

	return s.Stop()
}

func runInit(cmd *cobra.Command, args []string) error {
	mode := args[0]

	var cfg *config.Config
	switch mode {
	case "client":
		cfg = config.DefaultClientConfig()
	case "server":
		cfg = config.DefaultServerConfig()
	default:
		return fmt.Errorf("invalid mode: %s, must be 'client' or 'server'", mode)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Print(string(data))
	return nil
}
