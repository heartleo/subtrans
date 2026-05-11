// Package translator contains the core subtitle translation types and logic.
package translator

import "time"

// Line is a single subtitle entry.
type Line struct {
	Number      int
	Start       time.Duration
	End         time.Duration
	Text        string
	Translation string // filled in-place by the translator after parsing
	// Metadata is codec-private per-line state captured during Parse and
	// consumed during Format (e.g. ASS Dialogue non-text fields). The
	// translator pipeline does not inspect or mutate this field.
	Metadata any
}

// Batch is a group of lines sent in a single API call.
type Batch struct {
	Number      int
	Lines       []*Line
	RawResponse string // raw AI response text; kept for debugging
	Errors      []error
}

// Options controls translation behaviour.
type Options struct {
	TargetLanguage           string
	Instructions             string // system instructions text (resolved to string before calling translator)
	Prompt                   string // user prompt prefix
	MaxBatchSize             int    // default: 30
	BatchSplitPunctuation    string // punctuation characters used as batch split points; default: "."
	Temperature              float64
	MaxRetries               int  // API-level retries on 429/5xx only; default: 3
	IncludeOriginal          bool // write original text alongside translation in output SRT
	StripTrailingPunctuation bool // strip trailing periods and commas from each subtitle line; default: true
}

// Default translation settings.
const (
	DefaultMaxBatchSize          = 30
	DefaultBatchSplitPunctuation = "."
)

// DefaultOptions returns an Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		TargetLanguage:           "zh",
		MaxBatchSize:             DefaultMaxBatchSize,
		BatchSplitPunctuation:    DefaultBatchSplitPunctuation,
		MaxRetries:               3,
		StripTrailingPunctuation: true,
	}
}

// Message is a single message in a chat conversation.
// Defined in subtrans (consumer side) rather than in the openai package
// to avoid an import cycle.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}
