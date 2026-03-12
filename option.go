package subtrans

// Option configures translation behavior.
// Use the With* functions to create options.
type Option interface {
	apply(*options)
}

// options holds all translation-level configuration with defaults.
type options struct {
	instructions          string
	prompt                string
	maxBatchSize          int
	batchSplitPunctuation string
	includeOriginal       bool
	stripPunctuation      bool
}

// instructionsOption sets custom system instructions.
type instructionsOption string

func (o instructionsOption) apply(opts *options) { opts.instructions = string(o) }

// WithInstructions sets custom system instructions appended to the default prompt.
func WithInstructions(s string) Option { return instructionsOption(s) }

// promptOption sets a custom user prompt prefix.
type promptOption string

func (o promptOption) apply(opts *options) { opts.prompt = string(o) }

// WithPrompt sets a custom user prompt prefix.
func WithPrompt(s string) Option { return promptOption(s) }

// maxBatchSizeOption sets the number of subtitle lines per API call.
type maxBatchSizeOption int

func (o maxBatchSizeOption) apply(opts *options) { opts.maxBatchSize = int(o) }

// WithMaxBatchSize sets the number of subtitle lines per API call (default: 30).
func WithMaxBatchSize(n int) Option { return maxBatchSizeOption(n) }

// batchSplitPunctuationOption sets the punctuation characters used as batch split points.
type batchSplitPunctuationOption string

func (o batchSplitPunctuationOption) apply(opts *options) { opts.batchSplitPunctuation = string(o) }

// WithBatchSplitPunctuation sets punctuation characters for batch splitting (default: ".").
func WithBatchSplitPunctuation(s string) Option { return batchSplitPunctuationOption(s) }

// includeOriginalOption includes original text alongside translation in output.
type includeOriginalOption bool

func (o includeOriginalOption) apply(opts *options) { opts.includeOriginal = bool(o) }

// WithIncludeOriginal includes original text alongside translation in the output SRT.
func WithIncludeOriginal(b bool) Option { return includeOriginalOption(b) }

// stripPunctuationOption controls stripping of trailing punctuation.
type stripPunctuationOption bool

func (o stripPunctuationOption) apply(opts *options) { opts.stripPunctuation = bool(o) }

// WithStripPunctuation controls stripping trailing periods and commas from
// translated text (default: true).
func WithStripPunctuation(b bool) Option { return stripPunctuationOption(b) }
