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
	"github.com/heartleo/subtrans/internal/subtitle"
	_ "github.com/heartleo/subtrans/internal/subtitle/all" // register all codecs
	"github.com/heartleo/subtrans/internal/translator"
)

// Exported errors for programmatic checking.
var (
	ErrEmptyContent          = errors.New("subtrans: srt content is empty")
	ErrInvalidConfig         = errors.New("subtrans: invalid config")
	ErrTranslationIncomplete = errors.New("subtrans: translation incomplete")
)

// Default connection-level settings, exposed for callers that wish to inspect
// them. Mirrored by [New] when zero-valued Config fields are passed in.
const (
	DefaultBaseURL    = "https://api.openai.com/v1"
	DefaultModel      = "gpt-4.1"
	DefaultMaxRetries = 3
)

// Config holds the connection-level configuration for the LLM API.
type Config struct {
	APIKey      string
	BaseURL     string  // default: [DefaultBaseURL]
	Model       string  // default: [DefaultModel]
	Temperature float64 // default: 0
	MaxRetries  int     // default: [DefaultMaxRetries]
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
		conf.BaseURL = DefaultBaseURL
	}
	if conf.Model == "" {
		conf.Model = DefaultModel
	}
	if conf.MaxRetries == 0 {
		conf.MaxRetries = DefaultMaxRetries
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

	defaults := translator.DefaultOptions()
	o := options{
		maxBatchSize:          defaults.MaxBatchSize,
		batchSplitPunctuation: defaults.BatchSplitPunctuation,
		stripPunctuation:      defaults.StripTrailingPunctuation,
	}
	for _, opt := range opts {
		opt.apply(&o)
	}

	format := o.format
	if format == "" {
		format = subtitle.FormatSRT
	}
	codec, err := subtitle.For(format)
	if err != nil {
		return nil, fmt.Errorf("subtrans: %w", err)
	}

	doc, err := codec.Parse(srtContent)
	if err != nil {
		return nil, fmt.Errorf("parse subtitle: %w", err)
	}

	ops := translator.Options{
		TargetLanguage:           language,
		Instructions:             o.instructions,
		Prompt:                   o.prompt,
		MaxBatchSize:             o.maxBatchSize,
		BatchSplitPunctuation:    o.batchSplitPunctuation,
		IncludeOriginal:          o.includeOriginal,
		StripTrailingPunctuation: o.stripPunctuation,
	}

	batches := translator.BatchLines(doc.Lines, ops)

	if err := translator.Translate(ctx, batches, ops, t.client, translator.BaseHandler{}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTranslationIncomplete, err)
	}

	output := codec.Format(doc, subtitle.FormatOptions{
		IncludeOriginal:          o.includeOriginal,
		StripTrailingPunctuation: o.stripPunctuation,
	})

	return &Result{
		SRT:        output,
		LineCount:  len(doc.Lines),
		BatchCount: len(batches),
	}, nil
}
