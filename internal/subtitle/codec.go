// Package subtitle dispatches between concrete codecs (SRT, VTT, ASS, LRC,
// SBV). Codec implementations live in subpackages and self-register via init().
// Import the convenience meta-package internal/subtitle/all (blank import) to
// pull in every built-in codec.
//
// Round-trip strategy: each codec captures non-text structural elements
// (file-level headers, NOTE/STYLE blocks, per-line metadata) into Document and
// per-line Metadata, then re-emits them verbatim on Format. Inline formatting
// markers inside subtitle text (<i>, {\b1}, \N …) are passed through to the
// LLM with prompt instructions to preserve them — no strict validation.
package subtitle

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/heartleo/subtrans/internal/translator"
)

// Format identifies a supported subtitle format.
type Format string

// Supported formats.
const (
	FormatSRT Format = "srt"
	FormatVTT Format = "vtt"
	FormatASS Format = "ass"
	FormatLRC Format = "lrc"
	FormatSBV Format = "sbv"
)

// FormatOptions controls rendering across all codecs.
type FormatOptions struct {
	IncludeOriginal          bool
	StripTrailingPunctuation bool
}

// Document is the parsed representation of a subtitle file. Lines hold the
// translatable text plus per-line codec metadata; Meta holds file-level
// codec-specific state (headers, style sections, etc.). Pass the same
// Document instance from Parse through Translate to Format to round-trip
// non-text elements verbatim.
type Document struct {
	Lines []*translator.Line
	Meta  any
}

// Codec parses and renders a single subtitle format.
type Codec interface {
	Parse(content string) (*Document, error)
	Format(doc *Document, opts FormatOptions) string
}

// ErrUnknownFormat is returned when a Format value has no registered codec.
var ErrUnknownFormat = fmt.Errorf("subtitle: unknown format")

var (
	registryMu sync.RWMutex
	registry   = map[Format]Codec{}
)

// Register installs c as the codec for f. Intended to be called from a codec
// package's init() function.
func Register(f Format, c Codec) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[f] = c
}

// For returns the codec implementing f.
func For(f Format) (Codec, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	c, ok := registry[f]
	if !ok {
		return nil, fmt.Errorf("%w: %q (did you blank-import internal/subtitle/all?)", ErrUnknownFormat, f)
	}
	return c, nil
}

// DetectByExt maps a filename's extension to a Format.
// Recognised extensions: .srt .vtt .ass .ssa .lrc .sbv (case-insensitive).
func DetectByExt(path string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".srt":
		return FormatSRT, nil
	case ".vtt":
		return FormatVTT, nil
	case ".ass", ".ssa":
		return FormatASS, nil
	case ".lrc":
		return FormatLRC, nil
	case ".sbv":
		return FormatSBV, nil
	}
	return "", fmt.Errorf("%w: extension %q", ErrUnknownFormat, ext)
}
