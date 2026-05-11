// Package sbv implements the SubViewer / YouTube SBV codec.
package sbv

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/heartleo/subtrans/internal/subtitle"
	"github.com/heartleo/subtrans/internal/translator"
)

// ErrInvalidSBV is returned when content cannot be parsed as SBV.
var ErrInvalidSBV = errors.New("invalid SBV format")

// Codec implements subtitle.Codec for SBV.
type Codec struct{}

func init() { subtitle.Register(subtitle.FormatSBV, Codec{}) }

// Parse parses an SBV string into a Document.
func (Codec) Parse(content string) (*subtitle.Document, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSpace(content)
	if content == "" {
		return &subtitle.Document{}, nil
	}

	var lines []*translator.Line
	number := 1

	for block := range strings.SplitSeq(content, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		rows := strings.SplitN(block, "\n", 2)
		if len(rows) < 1 {
			continue
		}
		start, end, err := parseTimestampLine(rows[0])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSBV, err)
		}
		text := ""
		if len(rows) == 2 {
			text = strings.TrimSpace(rows[1])
		}
		lines = append(lines, &translator.Line{
			Number: number,
			Start:  start,
			End:    end,
			Text:   text,
		})
		number++
	}
	return &subtitle.Document{Lines: lines}, nil
}

// Format renders the document back to SBV.
func (Codec) Format(doc *subtitle.Document, opts subtitle.FormatOptions) string {
	var b strings.Builder
	for _, line := range doc.Lines {
		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in SBV output", "line", line.Number)
			continue
		}
		translation := line.Translation
		if opts.StripTrailingPunctuation {
			translation = subtitle.StripTrailing(translation)
		}

		b.WriteString(formatTimestamp(line.Start))
		b.WriteByte(',')
		b.WriteString(formatTimestamp(line.End))
		b.WriteByte('\n')

		if opts.IncludeOriginal && line.Text != "" {
			original := line.Text
			if opts.StripTrailingPunctuation {
				original = subtitle.StripTrailing(original)
			}
			b.WriteString(original)
			b.WriteByte('\n')
		}

		b.WriteString(translation)
		b.WriteString("\n\n")
	}
	return b.String()
}

func parseTimestampLine(s string) (time.Duration, time.Duration, error) {
	parts := strings.SplitN(strings.TrimSpace(s), ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("timestamp line %q missing comma", s)
	}
	start, err := parseTimestamp(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start timestamp: %w", err)
	}
	end, err := parseTimestamp(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end timestamp: %w", err)
	}
	return start, end, nil
}

func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("bad timestamp format: %q", s)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 {
		return 0, fmt.Errorf("hours in %q: %w", s, err)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("minutes out of range in %q", s)
	}

	secParts := strings.SplitN(parts[2], ".", 2)
	seconds, err := strconv.Atoi(secParts[0])
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("seconds out of range in %q", s)
	}

	ms := 0
	if len(secParts) == 2 {
		if len(secParts[1]) != 3 {
			return 0, fmt.Errorf("milliseconds must be 3 digits in %q", s)
		}
		ms, err = strconv.Atoi(secParts[1])
		if err != nil || ms < 0 || ms >= 1000 {
			return 0, fmt.Errorf("milliseconds out of range in %q", s)
		}
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}

func formatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms)
}
