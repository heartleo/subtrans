// Package ass implements the Advanced SubStation Alpha (ASS/SSA) codec.
//
// Round-trip preserves:
//   - All sections before [Events] verbatim (Script Info, V4+ Styles, Fonts,
//     Graphics, Aegisub Project Garbage, …) in original order.
//   - The [Events] Format line.
//   - All non-Dialogue lines inside [Events] (Comment, etc.) anchored to
//     their position relative to Dialogue cues.
//   - Per-Dialogue non-text fields (Layer, Style, Name, MarginL/R/V, Effect)
//     and any trailing fields not named in Format.
//
// Inline override blocks ({\b1}, {\fad(…)}, …) and ASS escapes (\N, \h) are
// left inside the text and forwarded to the LLM with prompt instructions to
// keep them verbatim.
package ass

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

// ErrInvalidASS is returned when content cannot be parsed as ASS/SSA.
var ErrInvalidASS = errors.New("invalid ASS/SSA format")

// Codec implements subtitle.Codec for ASS/SSA.
type Codec struct{}

func init() { subtitle.Register(subtitle.FormatASS, Codec{}) }

// Meta holds file-level ASS state captured during Parse.
type Meta struct {
	// Preamble is the raw text of every line up to (but not including) the
	// "[Events]" header.
	Preamble string
	// EventsHeader is the "[Events]" line as written in the source.
	EventsHeader string
	// EventsFormat is the "Format: …" line inside [Events].
	EventsFormat string
	// EventsFormatFields is the parsed Format field list.
	EventsFormatFields []string
	// EventsExtras are non-Dialogue lines inside [Events], anchored to a
	// Dialogue index (BeforeCue==len(Lines) for trailing).
	EventsExtras []ExtraLine
}

// ExtraLine is a non-Dialogue row inside [Events] to re-emit verbatim.
type ExtraLine struct {
	BeforeCue int
	Raw       string
}

// LineMeta holds the per-Dialogue non-text fields.
type LineMeta struct {
	// Prefix is the literal Dialogue line keyword (typically "Dialogue").
	Prefix string
	// Fields holds all fields named by EventsFormat except Text, indexed by
	// the field name (lower-case). Start and End are stored as raw strings
	// so they round-trip exactly; the translator pipeline uses the parsed
	// Start/End on the Line for batching.
	Fields map[string]string
	// FieldOrder is the lower-cased Format field order, repeated here so
	// Format can rebuild without reading Meta.
	FieldOrder []string
}

// Parse parses an ASS/SSA file into a Document.
func (Codec) Parse(content string) (*subtitle.Document, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if strings.TrimSpace(content) == "" {
		return &subtitle.Document{}, nil
	}

	meta := &Meta{}
	var lines []*translator.Line

	var preamble strings.Builder
	inEvents := false
	beforeEvents := true

	for raw := range strings.SplitSeq(content, "\n") {
		row := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(row)

		switch {
		case strings.EqualFold(trimmed, "[Events]"):
			inEvents = true
			beforeEvents = false
			meta.EventsHeader = row
			continue
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			inEvents = false
		}

		if beforeEvents {
			preamble.WriteString(row)
			preamble.WriteByte('\n')
			continue
		}

		if !inEvents {
			// A new section after [Events] — append back to preamble-style
			// trailing buffer. Treat as a trailing extra anchored at the end.
			meta.EventsExtras = append(meta.EventsExtras, ExtraLine{BeforeCue: -1, Raw: row})
			continue
		}

		// Inside [Events].
		switch {
		case strings.HasPrefix(trimmed, "Format:"):
			meta.EventsFormat = row
			meta.EventsFormatFields = splitCSV(strings.TrimPrefix(trimmed, "Format:"))
		case strings.HasPrefix(trimmed, "Dialogue:"):
			line, err := parseDialogue(trimmed, "Dialogue", meta.EventsFormatFields, len(lines)+1)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidASS, err)
			}
			if line != nil {
				lines = append(lines, line)
			}
		default:
			if trimmed == "" {
				continue
			}
			meta.EventsExtras = append(meta.EventsExtras, ExtraLine{BeforeCue: len(lines), Raw: row})
		}
	}

	meta.Preamble = preamble.String()
	return &subtitle.Document{Lines: lines, Meta: meta}, nil
}

// Format renders the document back to ASS/SSA.
func (Codec) Format(doc *subtitle.Document, opts subtitle.FormatOptions) string {
	meta, _ := doc.Meta.(*Meta)
	var b strings.Builder

	if meta != nil {
		b.WriteString(meta.Preamble)
		if meta.EventsHeader != "" {
			b.WriteString(meta.EventsHeader)
		} else {
			b.WriteString("[Events]")
		}
		b.WriteByte('\n')
		if meta.EventsFormat != "" {
			b.WriteString(meta.EventsFormat)
		} else {
			b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")
		}
		b.WriteByte('\n')
	} else {
		b.WriteString(defaultPreamble)
	}

	emitExtras := func(idx int) {
		if meta == nil {
			return
		}
		for _, ex := range meta.EventsExtras {
			if ex.BeforeCue == idx {
				b.WriteString(ex.Raw)
				b.WriteByte('\n')
			}
		}
	}

	for i, line := range doc.Lines {
		emitExtras(i)

		if line.Translation == "" {
			slog.Warn("skipping line with empty translation in ASS output", "line", line.Number)
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
			writeDialogueLine(&b, line, original)
		}
		writeDialogueLine(&b, line, translation)
	}

	emitExtras(len(doc.Lines))

	// Trailing sections after [Events].
	if meta != nil {
		for _, ex := range meta.EventsExtras {
			if ex.BeforeCue == -1 {
				b.WriteString(ex.Raw)
				b.WriteByte('\n')
			}
		}
	}

	return b.String()
}

func writeDialogueLine(b *strings.Builder, line *translator.Line, text string) {
	encoded := strings.ReplaceAll(text, "\n", `\N`)

	lm, _ := line.Metadata.(*LineMeta)
	if lm == nil || len(lm.FieldOrder) == 0 {
		// Fallback canonical form.
		fmt.Fprintf(b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatTimestamp(line.Start), formatTimestamp(line.End), encoded)
		return
	}

	b.WriteString(lm.Prefix)
	b.WriteString(": ")
	for i, name := range lm.FieldOrder {
		if i > 0 {
			b.WriteByte(',')
		}
		switch name {
		case "text":
			b.WriteString(encoded)
		case "start":
			b.WriteString(formatTimestamp(line.Start))
		case "end":
			b.WriteString(formatTimestamp(line.End))
		default:
			b.WriteString(lm.Fields[name])
		}
	}
	b.WriteByte('\n')
}

const defaultPreamble = `[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,30,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`

func parseDialogue(line, prefix string, format []string, number int) (*translator.Line, error) {
	if len(format) == 0 {
		return nil, errors.New("dialogue before format")
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, prefix+":"))
	fields := splitDialogue(payload, len(format))
	if len(fields) < len(format) {
		return nil, fmt.Errorf("dialogue has %d fields, expected %d", len(fields), len(format))
	}

	order := make([]string, len(format))
	stored := make(map[string]string, len(format))
	for i, name := range format {
		key := strings.ToLower(name)
		order[i] = key
		if key != "text" {
			stored[key] = fields[i]
		}
	}

	startStr := strings.TrimSpace(stored["start"])
	endStr := strings.TrimSpace(stored["end"])
	text := fields[len(format)-1] // text is conventionally last; safe given splitDialogue contract

	start, err := parseTimestamp(startStr)
	if err != nil {
		return nil, fmt.Errorf("start %q: %w", startStr, err)
	}
	end, err := parseTimestamp(endStr)
	if err != nil {
		return nil, fmt.Errorf("end %q: %w", endStr, err)
	}

	// Decode ASS escapes for the LLM-facing text but keep override blocks as-is.
	decoded := strings.ReplaceAll(text, `\N`, "\n")
	decoded = strings.ReplaceAll(decoded, `\h`, " ")

	if strings.TrimSpace(decoded) == "" {
		return nil, nil
	}

	return &translator.Line{
		Number: number,
		Start:  start,
		End:    end,
		Text:   decoded,
		Metadata: &LineMeta{
			Prefix:     prefix,
			Fields:     stored,
			FieldOrder: order,
		},
	}, nil
}

func splitDialogue(s string, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := strings.IndexByte(s, ',')
		if idx < 0 {
			out = append(out, s)
			return out
		}
		out = append(out, s[:idx])
		s = s[idx+1:]
	}
	out = append(out, s)
	return out
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
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

	cs := 0
	if len(secParts) == 2 {
		if len(secParts[1]) != 2 {
			return 0, fmt.Errorf("centiseconds must be 2 digits in %q", s)
		}
		cs, err = strconv.Atoi(secParts[1])
		if err != nil || cs < 0 || cs >= 100 {
			return 0, fmt.Errorf("centiseconds out of range in %q", s)
		}
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(cs)*10*time.Millisecond, nil
}

func formatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	cs := int(d.Milliseconds()/10) % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}
