// Package srt implements the SRT subtitle codec.
package srt

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

// ErrInvalidSRT is returned when the SRT content cannot be parsed.
var ErrInvalidSRT = errors.New("invalid SRT format")

// Codec implements subtitle.Codec for SRT.
type Codec struct{}

func init() { subtitle.Register(subtitle.FormatSRT, Codec{}) }

// Parse parses an SRT string and returns the subtitle lines.
func (Codec) Parse(content string) (*subtitle.Document, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return &subtitle.Document{}, nil
	}

	blocks := splitBlocks(content)
	lines := make([]*translator.Line, 0, len(blocks))

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		line, err := parseBlock(block)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSRT, err)
		}

		lines = append(lines, line)
	}

	return &subtitle.Document{Lines: lines}, nil
}

// Format renders the document to SRT.
func (Codec) Format(doc *subtitle.Document, opts subtitle.FormatOptions) string {
	var b strings.Builder
	number := 1

	for _, line := range doc.Lines {
		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in SRT output", "line", line.Number)
			continue
		}

		translation := line.Translation
		if opts.StripTrailingPunctuation {
			translation = subtitle.StripTrailing(translation)
		}

		b.WriteString(strconv.Itoa(number))
		b.WriteByte('\n')
		b.WriteString(FormatTimestamp(line.Start))
		b.WriteString(" --> ")
		b.WriteString(FormatTimestamp(line.End))
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
		number++
	}

	return b.String()
}

func splitBlocks(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	var blocks []string
	var current strings.Builder

	for rawLine := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(rawLine) == "" {
			if current.Len() > 0 {
				blocks = append(blocks, current.String())
				current.Reset()
			}

			continue
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}

		current.WriteString(rawLine)
	}

	if current.Len() > 0 {
		blocks = append(blocks, current.String())
	}

	return blocks
}

func parseBlock(block string) (*translator.Line, error) {
	rows := strings.SplitN(block, "\n", 3)
	if len(rows) < 2 {
		return nil, fmt.Errorf("block has fewer than 2 rows: %q", block)
	}

	number, err := strconv.Atoi(strings.TrimSpace(rows[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid subtitle number %q: %w", rows[0], err)
	}

	start, end, err := parseTimestampLine(strings.TrimSpace(rows[1]))
	if err != nil {
		return nil, err
	}

	text := ""
	if len(rows) == 3 {
		text = strings.TrimSpace(rows[2])
	}

	return &translator.Line{
		Number: number,
		Start:  start,
		End:    end,
		Text:   text,
	}, nil
}

func parseTimestampLine(s string) (time.Duration, time.Duration, error) {
	parts := strings.SplitN(s, " --> ", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("timestamp line %q missing ' --> '", s)
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
	s = strings.ReplaceAll(s, ",", ".")
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

// FormatTimestamp formats a time.Duration as an SRT timestamp: HH:MM:SS,mmm.
func FormatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
