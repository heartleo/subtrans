// Package subtrans provides a public API for translating SRT subtitle files
// using an OpenAI-compatible LLM.
//
// Basic usage:
//
//	t, err := subtrans.New(subtrans.Config{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	result, err := t.Translate(ctx, srtContent, "zh")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	os.WriteFile("output.zh.srt", []byte(result.SRT), 0o644)
package subtrans

import (
	"context"
	"errors"
	"fmt"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/openai"
	"github.com/heartleo/subtrans/internal/srt"
	"github.com/heartleo/subtrans/internal/translator"
)

// Exported errors for programmatic checking.
var (
	ErrEmptyContent          = errors.New("subtrans: srt content is empty")
	ErrInvalidConfig         = errors.New("subtrans: invalid config")
	ErrTranslationIncomplete = errors.New("subtrans: translation incomplete")
)

// Config holds the connection-level configuration for the LLM API.
type Config struct {
	APIKey      string
	BaseURL     string // default: "https://api.openai.com/v1"
	Model       string // default: "gpt-4.1"
	Temperature float64
	MaxRetries  int // default: 3
}

// Result holds the output of a translation.
type Result struct {
	SRT        string // translated SRT content
	LineCount  int    // number of translated lines
	BatchCount int    // number of batches used
}

// Translator translates SRT subtitle content using an LLM.
// Create one with [New] and reuse it across multiple [Translator.Translate] calls.
type Translator struct {
	client translator.Completer
}

// New creates a new [Translator] with the given connection config.
// Returns [ErrInvalidConfig] if APIKey is empty.
func New(cfg Config) (*Translator, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: api_key is required", ErrInvalidConfig)
	}

	conf := config.Config{
		APIKey:      cfg.APIKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxRetries:  cfg.MaxRetries,
	}
	if conf.BaseURL == "" {
		conf.BaseURL = "https://api.openai.com/v1"
	}
	if conf.Model == "" {
		conf.Model = "gpt-4.1"
	}
	if conf.MaxRetries == 0 {
		conf.MaxRetries = 3
	}

	return &Translator{
		client: openai.NewClient(conf),
	}, nil
}

// Translate translates SRT subtitle content into the given language.
// Language is a required parameter (ISO code, e.g. "zh", "ja", "ko").
// Use [Option] values to customize translation behavior.
func (t *Translator) Translate(ctx context.Context, srtContent string, language string, opts ...Option) (*Result, error) {
	if srtContent == "" {
		return nil, ErrEmptyContent
	}
	if language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidConfig)
	}

	// Set defaults, then apply options.
	o := options{
		maxBatchSize:          30,
		batchSplitPunctuation: ".",
		stripPunctuation:      true,
	}
	for _, opt := range opts {
		opt.apply(&o)
	}

	lines, err := srt.Parse(srtContent)
	if err != nil {
		return nil, fmt.Errorf("parse SRT: %w", err)
	}

	ops := translator.Options{
		TargetLanguage:           language,
		Instructions:             o.instructions,
		Prompt:                   o.prompt,
		MaxBatchSize:             o.maxBatchSize,
		BatchSplitPunctuation:    o.batchSplitPunctuation,
		Temperature:              0,
		IncludeOriginal:          o.includeOriginal,
		StripTrailingPunctuation: o.stripPunctuation,
	}

	batches := translator.BatchLines(lines, ops)

	h := &collectHandler{}
	translator.Translate(ctx, batches, ops, t.client, h)

	if h.err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTranslationIncomplete, h.err)
	}

	var allLines []*translator.Line
	for _, batch := range batches {
		allLines = append(allLines, batch.Lines...)
	}

	srtOutput := srt.Format(allLines, srt.FormatOptions{
		IncludeOriginal:          o.includeOriginal,
		StripTrailingPunctuation: o.stripPunctuation,
	})

	return &Result{
		SRT:        srtOutput,
		LineCount:  len(allLines),
		BatchCount: len(batches),
	}, nil
}

// collectHandler implements translator.TranslationHandler to collect errors.
type collectHandler struct {
	translator.BaseHandler
	err error
}

func (h *collectHandler) OnError(_ int, err error) {
	h.err = err
}
