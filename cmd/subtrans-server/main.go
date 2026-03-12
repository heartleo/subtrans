// Command subtrans-server serves the translation HTTP API.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/heartleo/subtrans/api"
	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		host    string
		port    int
		verbose bool
	)

	cmd := &cobra.Command{
		Use:   "subtrans-server",
		Short: "HTTP server for subtitle translation via SSE",
		RunE: func(_ *cobra.Command, _ []string) error {
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("config (API key required): %w", err)
			}

			client := openai.NewClient(cfg)
			handler := api.NewHandler(cfg, client)

			mux := http.NewServeMux()
			mux.Handle("/translate", handler)

			addr := fmt.Sprintf("%s:%d", host, port)
			slog.Info("starting server", "addr", addr)

			server := &http.Server{
				Addr:              addr,
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
			}

			if err := server.ListenAndServe(); err != nil {
				return fmt.Errorf("server: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "localhost", "host to listen on")
	cmd.Flags().IntVarP(&port, "port", "p", 8091, "port to listen on")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")

	return cmd
}
