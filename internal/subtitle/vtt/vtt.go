// Package vtt implements the WebVTT subtitle codec.
//
// Round-trip preserves the WEBVTT header line + any descriptor, header
// metadata lines, and NOTE/STYLE/REGION blocks at their original positions
// relative to cues. Cue identifiers and cue settings on each cue line are
// also preserved. Inline tags (<i>, <v>, <c.cls>, <ruby>, …) are left in
// place inside the text and passed through to the LLM with prompt
// instructions to keep them verbatim.
package vtt

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

// ErrInvalidVTT is returned when content does not look like WebVTT.
var ErrInvalidVTT = errors.New("invalid WebVTT format")

// Codec implements subtitle.Codec for WebVTT.
type Codec struct{}

func init() { subtitle.Register(subtitle.FormatVTT, Codec{}) }

// Meta holds file-level VTT state captured during Parse.
type Meta struct {
	// Header is the first line of the file ("WEBVTT" plus optional descriptor).
	Header string
	// HeaderMetadata are the lines immediately after the header up to the
	// first blank line (e.g. "Kind: captions", "Language: en").
	HeaderMetadata []string
	// Blocks are NOTE / STYLE / REGION sections to re-emit, each anchored
	// before the cue with that index (BeforeCue==len(Lines) means trailing).
	Blocks []Block
}

// Block is a non-cue section that must be re-emitted verbatim.
type Block struct {
	BeforeCue int    // 0-based cue index this block precedes; len(Lines) for trailing
	Raw       string // raw block text, no leading/trailing blank line
}

// LineMeta is per-cue carrier captured during Parse.
type LineMeta struct {
	// CueID is the optional identifier line that may precede the timestamp.
	CueID string
	// Settings is the cue settings string after the end timestamp
	// (e.g. "align:start position:50%"). Empty if absent.
	Settings string
}

// Parse parses a WebVTT string into a Document.
func (Codec) Parse(content string) (*subtitle.Document, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return &subtitle.Document{}, nil
	}
	if !strings.HasPrefix(trimmed, "WEBVTT") {
		return nil, fmt.Errorf("%w: missing WEBVTT header", ErrInvalidVTT)
	}

	meta := &Meta{}
	var lines []*translator.Line

	// Split into blocks by blank lines, preserving order.
	blocks := strings.Split(content, "\n\n")
	for i, raw := range blocks {
		raw = strings.TrimRight(raw, "\n")
		raw = strings.TrimLeft(raw, "\n")
		if raw == "" {
			continue
		}

		if i == 0 {
			// First block: WEBVTT line + optional header metadata lines.
			hdr := strings.Split(raw, "\n")
			meta.Header = hdr[0]
			if len(hdr) > 1 {
				meta.HeaderMetadata = hdr[1:]
			}
			continue
		}

		switch {
		case isBlockType(raw, "NOTE"), isBlockType(raw, "STYLE"), isBlockType(raw, "REGION"):
			meta.Blocks = append(meta.Blocks, Block{BeforeCue: len(lines), Raw: raw})
		default:
			line, err := parseCue(raw, len(lines)+1)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidVTT, err)
			}
			if line != nil {
				lines = append(lines, line)
			}
		}
	}

	return &subtitle.Document{Lines: lines, Meta: meta}, nil
}

// Format renders the document back to WebVTT.
func (Codec) Format(doc *subtitle.Document, opts subtitle.FormatOptions) string {
	var b strings.Builder

	meta, _ := doc.Meta.(*Meta)
	header := "WEBVTT"
	var headerMeta []string
	var blocks []Block
	if meta != nil {
		if meta.Header != "" {
			header = meta.Header
		}
		headerMeta = meta.HeaderMetadata
		blocks = meta.Blocks
	}

	b.WriteString(header)
	b.WriteByte('\n')
	for _, line := range headerMeta {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	emitBlocksBefore := func(idx int) {
		for _, blk := range blocks {
			if blk.BeforeCue == idx {
				b.WriteString(blk.Raw)
				b.WriteString("\n\n")
			}
		}
	}

	cueIdx := 0
	for _, line := range doc.Lines {
		emitBlocksBefore(cueIdx)
		cueIdx++

		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in VTT output", "line", line.Number)
			continue
		}

		translation := line.Translation
		if opts.StripTrailingPunctuation {
			translation = subtitle.StripTrailing(translation)
		}

		lm, _ := line.Metadata.(*LineMeta)
		if lm != nil && lm.CueID != "" {
			b.WriteString(lm.CueID)
			b.WriteByte('\n')
		}
		b.WriteString(formatTimestamp(line.Start))
		b.WriteString(" --> ")
		b.WriteString(formatTimestamp(line.End))
		if lm != nil && lm.Settings != "" {
			b.WriteByte(' ')
			b.WriteString(lm.Settings)
		}
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

	// Trailing blocks (anchored past the last cue).
	emitBlocksBefore(cueIdx)

	return b.String()
}

func isBlockType(raw, name string) bool {
	return raw == name || strings.HasPrefix(raw, name+"\n") || strings.HasPrefix(raw, name+" ")
}

func parseCue(block string, number int) (*translator.Line, error) {
	rows := strings.Split(block, "\n")
	tsIdx := -1
	for i, row := range rows {
		if strings.Contains(row, "-->") {
			tsIdx = i
			break
		}
	}
	if tsIdx < 0 {
		return nil, nil
	}

	cueID := ""
	if tsIdx > 0 {
		cueID = strings.TrimSpace(strings.Join(rows[:tsIdx], "\n"))
	}

	start, end, settings, err := parseTimestampLine(rows[tsIdx])
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(strings.Join(rows[tsIdx+1:], "\n"))
	return &translator.Line{
		Number:   number,
		Start:    start,
		End:      end,
		Text:     text,
		Metadata: &LineMeta{CueID: cueID, Settings: settings},
	}, nil
}

func parseTimestampLine(s string) (time.Duration, time.Duration, string, error) {
	parts := strings.SplitN(s, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, "", fmt.Errorf("timestamp line %q missing '-->'", s)
	}
	fields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(fields) == 0 {
		return 0, 0, "", fmt.Errorf("timestamp line %q missing end time", s)
	}

	start, err := parseTimestamp(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid start timestamp: %w", err)
	}
	end, err := parseTimestamp(fields[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid end timestamp: %w", err)
	}
	settings := strings.Join(fields[1:], " ")
	return start, end, settings, nil
}

func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	var hours, minutes int
	var secField string
	switch len(parts) {
	case 2:
		secField = parts[1]
		var err error
		minutes, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("minutes in %q: %w", s, err)
		}
	case 3:
		secField = parts[2]
		var err error
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("hours in %q: %w", s, err)
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("minutes in %q: %w", s, err)
		}
	default:
		return 0, fmt.Errorf("bad timestamp format: %q", s)
	}
	if minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("minutes out of range in %q", s)
	}

	secParts := strings.SplitN(secField, ".", 2)
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
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
