package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
	"github.com/heartleo/subtrans/internal/srt"
	"github.com/heartleo/subtrans/internal/translator"
)

var (
	translateOutput           string
	translateLanguage         string
	translateModel            string
	translateInstructionsFile string
	translatePrompt           string
	translateMaxBatchSize     int
	translateBatchSplitPunct  string
	translateTemperature      float64
	translateMaxRetries       int
	translateIncludeOriginal  bool
	translateStripPunctuation bool
)

func runTranslate(inputPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if translateModel != "" {
		cfg.Model = translateModel
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	outputPath := translateOutput
	if outputPath == "" {
		outputPath = deriveOutputPath(inputPath, translateLanguage)
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
	opts.TargetLanguage = translateLanguage
	opts.Prompt = translatePrompt
	opts.MaxBatchSize = translateMaxBatchSize
	opts.BatchSplitPunctuation = translateBatchSplitPunct
	opts.Temperature = translateTemperature
	opts.MaxRetries = translateMaxRetries
	opts.IncludeOriginal = translateIncludeOriginal

	if translateInstructionsFile != "" {
		b, err := os.ReadFile(translateInstructionsFile) // #nosec G304
		if err != nil {
			return fmt.Errorf("read instructions: %w", err)
		}
		opts.Instructions = string(b)
	}

	batches := translator.BatchLines(lines, opts)
	handler := &cliHandler{
		outputPath: outputPath,
		fmtOpts: srt.FormatOptions{
			IncludeOriginal:          translateIncludeOriginal,
			StripTrailingPunctuation: translateStripPunctuation,
		},
	}
	if err := translator.Translate(context.Background(), batches, opts, openai.NewClient(cfg), handler); err != nil {
		return fmt.Errorf("translate: %w", err)
	}
	return nil
}

func deriveOutputPath(input, lang string) string {
	if i := strings.LastIndex(input, "."); i >= 0 {
		return input[:i] + "." + strings.ToLower(strings.ReplaceAll(lang, " ", "_")) + ".srt"
	}
	return input + ".translated.srt"
}

type cliHandler struct {
	translator.BaseHandler
	outputPath string
	fmtOpts    srt.FormatOptions
}

func (h *cliHandler) OnBatchDone(batch int, lines []*translator.Line) {
	_, _ = fmt.Fprintf(os.Stderr, "Batch %d: %d lines translated\n", batch, len(lines))
}

func (h *cliHandler) OnDone(lines []*translator.Line) {
	srtOutput := srt.Format(lines, h.fmtOpts)
	if err := os.WriteFile(h.outputPath, []byte(srtOutput), 0o600); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Saved to %s\n", h.outputPath)
	}
}
