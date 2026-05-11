package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
	"github.com/heartleo/subtrans/internal/subtitle"
	_ "github.com/heartleo/subtrans/internal/subtitle/all" // register codecs
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

	format, err := subtitle.DetectByExt(inputPath)
	if err != nil {
		return fmt.Errorf("detect format: %w", err)
	}
	codec, err := subtitle.For(format)
	if err != nil {
		return fmt.Errorf("codec: %w", err)
	}

	outputPath := translateOutput
	if outputPath == "" {
		outputPath = deriveOutputPath(inputPath, translateLanguage, format)
	}

	fileBytes, err := os.ReadFile(inputPath) // #nosec G304 -- user-supplied path is intentional
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	doc, err := codec.Parse(string(fileBytes))
	if err != nil {
		return fmt.Errorf("parse subtitle: %w", err)
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

	batches := translator.BatchLines(doc.Lines, opts)
	handler := &cliHandler{
		outputPath: outputPath,
		codec:      codec,
		doc:        doc,
		fmtOpts: subtitle.FormatOptions{
			IncludeOriginal:          translateIncludeOriginal,
			StripTrailingPunctuation: translateStripPunctuation,
		},
	}
	if err := translator.Translate(context.Background(), batches, opts, openai.NewClient(cfg), handler); err != nil {
		return fmt.Errorf("translate: %w", err)
	}
	return nil
}

func deriveOutputPath(input, lang string, format subtitle.Format) string {
	ext := filepath.Ext(input)
	stem := strings.TrimSuffix(input, ext)
	langSuffix := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	outExt := ext
	if outExt == "" {
		outExt = "." + string(format)
	}
	return stem + "." + langSuffix + outExt
}

type cliHandler struct {
	translator.BaseHandler
	outputPath string
	codec      subtitle.Codec
	doc        *subtitle.Document
	fmtOpts    subtitle.FormatOptions
}

func (h *cliHandler) OnBatchDone(batch int, lines []*translator.Line) {
	_, _ = fmt.Fprintf(os.Stderr, "Batch %d: %d lines translated\n", batch, len(lines))
}

func (h *cliHandler) OnDone(_ []*translator.Line) {
	output := h.codec.Format(h.doc, h.fmtOpts)
	if err := os.WriteFile(h.outputPath, []byte(output), 0o600); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Saved to %s\n", h.outputPath)
	}
}
