package srt

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/heartleo/subtrans/internal/translator"
)

// FormatOptions controls SRT output formatting.
type FormatOptions struct {
	IncludeOriginal          bool
	StripTrailingPunctuation bool
}

// Format renders a slice of Lines to a valid SRT string.
// Lines with empty Translation are skipped. Lines are renumbered from 1.
func Format(lines []*translator.Line, opts FormatOptions) string {
	var b strings.Builder
	number := 1

	for _, line := range lines {
		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in SRT output", "line", line.Number)
			continue
		}

		translation := line.Translation
		if opts.StripTrailingPunctuation {
			translation = stripTrailing(translation)
		}

		_, _ = fmt.Fprintf(&b, "%d\n", number)
		_, _ = fmt.Fprintf(&b, "%s --> %s\n", FormatTimestamp(line.Start), FormatTimestamp(line.End))

		if opts.IncludeOriginal && line.Text != "" {
			_, _ = fmt.Fprintf(&b, "%s\n", line.Text)
		}

		_, _ = fmt.Fprintf(&b, "%s\n\n", translation)
		number++
	}

	return b.String()
}

// stripTrailing removes trailing periods and commas (both ASCII and CJK) from text.
// Preserves ellipsis (..., \u2026) and other meaningful punctuation (! ? etc).
func stripTrailing(s string) string {
	// Handle multiline subtitles: process each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = stripTrailingLine(line)
	}
	return strings.Join(lines, "\n")
}

func stripTrailingLine(s string) string {
	s = strings.TrimRight(s, " ")
	if s == "" {
		return s
	}

	// Don't strip ellipsis
	if strings.HasSuffix(s, "...") || strings.HasSuffix(s, "\u2026") {
		return s
	}

	// Strip trailing period/comma (ASCII and CJK fullwidth)
	trailingChars := ".," + // ASCII
		"\uff0c" + // fullwidth comma ，
		"\u3002" + // fullwidth period 。
		"\uff0e" // fullwidth full stop ．

	return strings.TrimRight(s, trailingChars)
}

// FormatTimestamp formats a time.Duration as an SRT timestamp: HH:MM:SS,mmm.
func FormatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
