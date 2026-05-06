package cli

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/heartleo/subtrans/api"
	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
)

var (
	serveHost string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for subtitle translation via SSE",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServe()
	},
}

func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config (API key required): %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/translate", api.NewHandler(cfg, openai.NewClient(cfg)))

	addr := fmt.Sprintf("%s:%d", serveHost, servePort)
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
}
