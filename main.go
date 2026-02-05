package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lifinance/lifi-mcp/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const version = "2.0.0"

func main() {
	var (
		port        = flag.Int("port", 8080, "HTTP server port")
		host        = flag.String("host", "0.0.0.0", "HTTP server host (use 0.0.0.0 for container deployment)")
		showVersion = flag.Bool("version", false, "Show version information")
		logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	// Initialize structured logging
	logger := initLogger(*logLevel)
	slog.SetDefault(logger)

	if *showVersion {
		fmt.Printf("lifi-mcp version %s\n", version)
		return
	}

	// Create the server (no API key - it's per-request now)
	s := server.NewServer(version, logger)

	logger.Info("API keys are now passed per-request via Authorization header (Bearer token) or X-LiFi-Api-Key header")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create the Streamable HTTP server
	httpServer := mcpserver.NewStreamableHTTPServer(
		s.GetMCPServer(),
		mcpserver.WithEndpointPath("/mcp"),
		mcpserver.WithHeartbeatInterval(30*time.Second),
		mcpserver.WithStateLess(true), // Stateless for multi-tenant
		mcpserver.WithHTTPContextFunc(server.ExtractAPIKeyFromRequest),
	)

	// Start server in a goroutine
	addr := fmt.Sprintf("%s:%d", *host, *port)
	go func() {
		logger.Info("Starting LiFi MCP Server",
			"version", version,
			"address", addr,
			"endpoint", "/mcp",
		)
		if err := httpServer.Start(addr); err != nil {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Received shutdown signal, exiting...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Error during shutdown", "error", err)
	}
}

// initLogger creates a structured logger with the specified level
func initLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Use JSON handler for structured logging (easier to parse in production)
	handler := slog.NewJSONHandler(os.Stderr, opts)
	return slog.New(handler)
}
