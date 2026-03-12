// Command subtrans translates SRT subtitle files using an OpenAI-compatible API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
	"github.com/heartleo/subtrans/internal/srt"
	"github.com/heartleo/subtrans/internal/translator"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// cliHandler implements translator.TranslationHandler for CLI output.
type cliHandler struct {
	outputPath string
	fmtOpts    srt.FormatOptions
}

func (h *cliHandler) OnBatchDone(batch int, lines []*translator.Line) {
	_, _ = fmt.Fprintf(os.Stderr, "Batch %d: %d lines translated\n", batch, len(lines))
}

func (h *cliHandler) OnError(batch int, err error) {
	if batch > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "error (batch %d): %v\n", batch, err)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}

func (h *cliHandler) OnDone(lines []*translator.Line) {
	srtOutput := srt.Format(lines, h.fmtOpts)
	if writeErr := os.WriteFile(h.outputPath, []byte(srtOutput), 0o600); writeErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", writeErr)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Saved to %s\n", h.outputPath)
	}
}

func rootCmd() *cobra.Command {
	var (
		output                string
		language              string
		model                 string
		instructionsFile      string
		prompt                string
		maxBatchSize          int
		batchSplitPunctuation string
		temperature           float64
		maxRetries            int
		includeOriginal       bool
		stripPunctuation      bool
		verbose               bool
	)

	cmd := &cobra.Command{
		Use:   "subtrans [flags] input.srt",
		Short: "Translate SRT subtitle using OpenAI-compatible APIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if model != "" {
				cfg.Model = model
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("config: %w", err)
			}

			inputPath := args[0]
			outputPath := output
			if outputPath == "" {
				outputPath = deriveOutputPath(inputPath, language)
			}

			fileBytes, err := os.ReadFile(inputPath) // #nosec G304 -- user-supplied path is intentional
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}

			lines, err := srt.Parse(string(fileBytes))
			if err != nil {
				return fmt.Errorf("parse SRT: %w", err)
			}

			opts := translator.DefaultOptions()
			opts.TargetLanguage = language
			opts.Prompt = prompt
			opts.MaxBatchSize = maxBatchSize
			opts.BatchSplitPunctuation = batchSplitPunctuation
			opts.Temperature = temperature
			opts.MaxRetries = maxRetries
			opts.IncludeOriginal = includeOriginal

			if instructionsFile != "" {
				b, readErr := os.ReadFile(instructionsFile) // #nosec G304
				if readErr != nil {
					return fmt.Errorf("read instructions: %w", readErr)
				}

				opts.Instructions = string(b)
			}

			batches := translator.BatchLines(lines, opts)
			client := openai.NewClient(cfg)

			handler := &cliHandler{
				outputPath: outputPath,
				fmtOpts: srt.FormatOptions{
					IncludeOriginal:          includeOriginal,
					StripTrailingPunctuation: stripPunctuation,
				},
			}

			translator.Translate(context.Background(), batches, opts, client, handler)

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "output SRT file path")
	cmd.Flags().StringVarP(&language, "language", "l", "zh", "target language ISO code (e.g. zh, en, ja, ko, fr)")
	cmd.Flags().StringVarP(&model, "model", "m", "", "model override")
	cmd.Flags().StringVar(&instructionsFile, "instructions", "", "path to instructions text file")
	cmd.Flags().StringVar(&prompt, "prompt", "", "custom user prompt prefix")
	cmd.Flags().IntVar(&maxBatchSize, "max-batch-size", 30, "lines per batch")
	cmd.Flags().StringVar(&batchSplitPunctuation, "batch-split-punct", ".", "punctuation characters for batch splitting (e.g. \".?!\")")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.0, "LLM temperature")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 3, "API retry count")
	cmd.Flags().BoolVar(&includeOriginal, "include-original", false, "include original in output")
	cmd.Flags().BoolVar(&stripPunctuation, "strip-punctuation", true, "strip trailing periods and commas from subtitles")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")

	return cmd
}

func deriveOutputPath(input, lang string) string {
	if i := strings.LastIndex(input, "."); i >= 0 {
		return input[:i] + "." + strings.ToLower(strings.ReplaceAll(lang, " ", "_")) + ".srt"
	}

	return input + ".translated.srt"
}
