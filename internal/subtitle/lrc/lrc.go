// Package lrc implements the LRC (lyrics) subtitle codec.
//
// LRC line form: "[mm:ss.xx]text" with optional milliseconds. End time is not
// stored in the format — it is inferred as the start time of the next cue
// (or +5s for the final cue).
//
// Round-trip preserves ID tags ([ti:], [ar:], [al:], [by:], [length:], [offset:])
// at the top of the output. Multiple timestamps on a single text line are
// expanded into separate lines (no dedup); each translation is rendered for
// every original timestamp it was attached to.
package lrc

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heartleo/subtrans/internal/subtitle"
	"github.com/heartleo/subtrans/internal/translator"
)

// ErrInvalidLRC is returned when content cannot be parsed as LRC.
var ErrInvalidLRC = errors.New("invalid LRC format")

// Codec implements subtitle.Codec for LRC.
type Codec struct{}

func init() { subtitle.Register(subtitle.FormatLRC, Codec{}) }

const tailDuration = 5 * time.Second

// Meta holds file-level LRC state captured during Parse.
type Meta struct {
	// Preamble contains raw ID-tag lines in their original order, e.g.
	// "[ti:Song Title]", "[offset:+200]".
	Preamble []string
	// Offset is the parsed value of [offset:N] in milliseconds, if present.
	Offset time.Duration
}

var (
	stampRE = regexp.MustCompile(`\[(\d+):(\d{1,2})(?:\.(\d{1,3}))?\]`)
	metaRE  = regexp.MustCompile(`^\[([A-Za-z]+):(.*)\]$`)
)

// Parse extracts timed lyric lines from LRC content.
func (Codec) Parse(content string) (*subtitle.Document, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	meta := &Meta{}

	type rawCue struct {
		start time.Duration
		text  string
	}
	var cues []rawCue

	for raw := range strings.SplitSeq(content, "\n") {
		row := strings.TrimSpace(raw)
		if row == "" {
			continue
		}

		// Recognise pure ID-tag lines (no timestamp).
		if m := metaRE.FindStringSubmatch(row); m != nil && !isTimestampKey(m[1]) {
			meta.Preamble = append(meta.Preamble, row)
			if strings.EqualFold(m[1], "offset") {
				if ms, err := strconv.Atoi(strings.TrimSpace(m[2])); err == nil {
					meta.Offset = time.Duration(ms) * time.Millisecond
				}
			}
			continue
		}

		matches := stampRE.FindAllStringSubmatchIndex(row, -1)
		if len(matches) == 0 {
			continue
		}
		text := strings.TrimSpace(row[matches[len(matches)-1][1]:])
		for _, mm := range matches {
			d, err := parseStamp(row[mm[2]:mm[3]], row[mm[4]:mm[5]], subIndex(row, mm, 6, 7))
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidLRC, err)
			}
			cues = append(cues, rawCue{start: d, text: text})
		}
	}

	sort.SliceStable(cues, func(i, j int) bool { return cues[i].start < cues[j].start })

	lines := make([]*translator.Line, 0, len(cues))
	for i, c := range cues {
		end := c.start + tailDuration
		if i+1 < len(cues) {
			end = cues[i+1].start
		}
		lines = append(lines, &translator.Line{
			Number: i + 1,
			Start:  c.start,
			End:    end,
			Text:   c.text,
		})
	}
	return &subtitle.Document{Lines: lines, Meta: meta}, nil
}

// Format renders the document back to LRC. End times are discarded.
func (Codec) Format(doc *subtitle.Document, opts subtitle.FormatOptions) string {
	var b strings.Builder

	if meta, ok := doc.Meta.(*Meta); ok && meta != nil {
		for _, line := range meta.Preamble {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		if len(meta.Preamble) > 0 {
			b.WriteByte('\n')
		}
	}

	for _, line := range doc.Lines {
		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in LRC output", "line", line.Number)
			continue
		}
		translation := line.Translation
		if opts.StripTrailingPunctuation {
			translation = subtitle.StripTrailing(translation)
		}
		if opts.IncludeOriginal && line.Text != "" {
			original := line.Text
			if opts.StripTrailingPunctuation {
				original = subtitle.StripTrailing(original)
			}
			fmt.Fprintf(&b, "[%s]%s\n", formatStamp(line.Start), original)
		}
		fmt.Fprintf(&b, "[%s]%s\n", formatStamp(line.Start), translation)
	}
	return b.String()
}

// isTimestampKey reports whether key is an all-digit string that looks like a
// timestamp ([mm:ss]). Pure ID tags use alphabetic keys like "ti", "ar".
func isTimestampKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseStamp(minS, secS, msS string) (time.Duration, error) {
	m, err := strconv.Atoi(minS)
	if err != nil || m < 0 {
		return 0, fmt.Errorf("minutes %q: %w", minS, err)
	}
	s, err := strconv.Atoi(secS)
	if err != nil || s < 0 || s >= 60 {
		return 0, fmt.Errorf("seconds out of range %q", secS)
	}
	var sub time.Duration
	if msS != "" {
		v, err := strconv.Atoi(msS)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("fraction %q: %w", msS, err)
		}
		switch len(msS) {
		case 1:
			sub = time.Duration(v) * 100 * time.Millisecond
		case 2:
			sub = time.Duration(v) * 10 * time.Millisecond
		case 3:
			sub = time.Duration(v) * time.Millisecond
		}
	}
	return time.Duration(m)*time.Minute + time.Duration(s)*time.Second + sub, nil
}

func subIndex(s string, m []int, a, b int) string {
	if a >= len(m) || m[a] < 0 || m[b] < 0 {
		return ""
	}
	return s[m[a]:m[b]]
}

func formatStamp(d time.Duration) string {
	total := int(d / time.Millisecond)
	m := total / 60000
	s := (total / 1000) % 60
	cs := (total / 10) % 100
	return fmt.Sprintf("%02d:%02d.%02d", m, s, cs)
}
