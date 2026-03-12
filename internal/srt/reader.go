// Package srt handles reading and writing SRT subtitle files.
package srt

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/heartleo/subtrans/internal/translator"
)

// ErrInvalidSRT is returned when the SRT content cannot be parsed.
var ErrInvalidSRT = errors.New("invalid SRT format")

// Parse parses an SRT string and returns the subtitle lines.
// Input must be UTF-8 encoded. Returns an empty slice for empty input.
func Parse(content string) ([]*translator.Line, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return make([]*translator.Line, 0), nil
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
			return nil, fmt.Errorf("%w: %v", ErrInvalidSRT, err)
		}

		lines = append(lines, line)
	}

	return lines, nil
}

func splitBlocks(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	var blocks []string
	var current strings.Builder

	for _, rawLine := range strings.Split(content, "\n") {
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
		return nil, fmt.Errorf("invalid subtitle number %q: %v", rows[0], err)
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
		return 0, 0, fmt.Errorf("invalid start timestamp: %v", err)
	}

	end, err := parseTimestamp(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end timestamp: %v", err)
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
	if err != nil {
		return 0, fmt.Errorf("hours in %q: %v", s, err)
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("minutes in %q: %v", s, err)
	}

	secParts := strings.SplitN(parts[2], ".", 2)

	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, fmt.Errorf("seconds in %q: %v", s, err)
	}

	ms := 0
	if len(secParts) == 2 {
		ms, err = strconv.Atoi(secParts[1])
		if err != nil {
			return 0, fmt.Errorf("milliseconds in %q: %v", s, err)
		}
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}
